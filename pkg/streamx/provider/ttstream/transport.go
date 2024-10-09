/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ttstream

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
	"github.com/cloudwego/netpoll"
)

const (
	clientTransport int32 = 1
	serverTransport int32 = 2

	streamCacheSize = 32
	frameChanSize   = 32
)

var transIgnoreError = errors.Join(netpoll.ErrEOF, io.EOF, netpoll.ErrConnClosed)

type transport struct {
	kind          int32
	sinfo         *serviceinfo.ServiceInfo
	conn          netpoll.Connection
	streams       sync.Map                 // key=streamID val=streamIO
	scache        []*stream                // size is streamCacheSize
	spipe         *container.Pipe[*stream] // in-coming stream channel
	wchannel      chan *Frame
	closed        chan struct{}
	closedFlag    int32
	streamingFlag int32 // flag == 0 means there is no active stream on transport
}

func newTransport(kind int32, sinfo *serviceinfo.ServiceInfo, conn netpoll.Connection) *transport {
	_ = conn.SetDeadline(time.Now().Add(time.Hour))
	t := &transport{
		kind:     kind,
		sinfo:    sinfo,
		conn:     conn,
		streams:  sync.Map{},
		spipe:    container.NewPipe[*stream](),
		scache:   make([]*stream, 0, streamCacheSize),
		wchannel: make(chan *Frame, frameChanSize),
		closed:   make(chan struct{}),
	}
	go func() {
		err := t.loopRead()
		if err != nil {
			if !errors.Is(err, transIgnoreError) {
				klog.Warnf("transport[%d] loop read err: %v", t.kind, err)
			}
			// if connection is closed by peer, loop read should return ErrConnClosed error, so we should close transport here
			_ = t.Close()
		}
	}()
	go func() {
		err := t.loopWrite()
		if err != nil {
			if !errors.Is(err, transIgnoreError) {
				klog.Warnf("transport[%d] loop write err: %v", t.kind, err)
			}
			_ = t.Close()
			// because loopWrite function return, we should close conn actively
			_ = t.conn.Close()
		}
	}()
	return t
}

// Close will close transport and destroy all resource and goroutines
// server close transport when connection is disconnected
// client close transport when transPool discard the transport
func (t *transport) Close() (err error) {
	if !atomic.CompareAndSwapInt32(&t.closedFlag, 0, 1) {
		return nil
	}
	close(t.closed)
	klog.Debugf("transport[%s] is closing", t.conn.LocalAddr())
	t.spipe.Close()
	t.streams.Range(func(key, value any) bool {
		sio := value.(*streamIO)
		sio.stream.close()
		_ = t.streamClose(sio.stream.sid)
		return true
	})
	return err
}

func (t *transport) IsActive() bool {
	return atomic.LoadInt32(&t.closedFlag) == 0 && t.conn.IsActive()
}

func (t *transport) storeStreamIO(ctx context.Context, s *stream) {
	t.streams.Store(s.sid, newStreamIO(ctx, s))
}

func (t *transport) loadStreamIO(sid int32) (sio *streamIO, ok bool) {
	val, ok := t.streams.Load(sid)
	if !ok {
		return sio, false
	}
	sio = val.(*streamIO)
	return sio, true
}

func (t *transport) loopRead() error {
	for {
		// decode frame
		sizeBuf, err := t.conn.Reader().Peek(4)
		if err != nil {
			return err
		}
		size := binary.BigEndian.Uint32(sizeBuf)
		slice, err := t.conn.Reader().Slice(int(size + 4))
		if err != nil {
			return err
		}
		reader := newReaderBuffer(slice)
		fr, err := DecodeFrame(context.Background(), reader)
		if err != nil {
			return err
		}
		klog.Debugf("transport[%d] DecodeFrame: fr=%v", t.kind, fr)

		switch fr.typ {
		case metaFrameType:
			sio, ok := t.loadStreamIO(fr.sid)
			if !ok {
				klog.Errorf("transport[%d] read a unknown stream meta: sid=%d", t.kind, fr.sid)
				continue
			}
			err = sio.stream.readMetaFrame(fr.meta, fr.header, fr.payload)
			if err != nil {
				return err
			}
		case headerFrameType:
			switch t.kind {
			case serverTransport:
				// Header Frame: server recv a new stream
				smode := t.sinfo.MethodInfo(fr.method).StreamingMode()
				s := newStream(context.Background(), t, smode, fr.streamFrame)
				klog.Debugf("transport[%d] read a new stream: sid=%d", t.kind, s.sid)
				t.spipe.Write(context.Background(), s)
			case clientTransport:
				// Header Frame: client recv header
				sio, ok := t.loadStreamIO(fr.sid)
				if !ok {
					klog.Errorf("transport[%d] read a unknown stream header: sid=%d", t.kind, fr.sid)
					continue
				}
				err = sio.stream.readHeader(fr.header)
				if err != nil {
					return err
				}
			}
		case dataFrameType:
			// Data Frame: decode and distribute data
			sio, ok := t.loadStreamIO(fr.sid)
			if !ok {
				klog.Errorf("transport[%d] read a unknown stream data: sid=%d", t.kind, fr.sid)
				continue
			}
			sio.input(context.Background(), fr)
		case trailerFrameType:
			// Trailer Frame: recv trailer, Close read direction
			sio, ok := t.loadStreamIO(fr.sid)
			if !ok {
				// client recv an unknown trailer is in exception,
				// because the client stream may already be GCed,
				// but the connection is still active so peer server can send a trailer
				klog.Debugf("transport[%d] read a unknown stream trailer: sid=%d", t.kind, fr.sid)
				continue
			}
			if err = sio.stream.readTrailer(fr.trailer); err != nil {
				return err
			}
		}
	}
}

func (t *transport) loopWrite() (err error) {
	defer func() {
		// loop write should help to close connection
		_ = t.conn.Close()
	}()
	writer := newWriterBuffer(t.conn.Writer())
	delay := 0
	// Important note:
	// loopWrite may cannot find stream by sid since it may send trailer and delete sid from streams
	for {
		select {
		case <-t.closed:
			return nil
		case fr, ok := <-t.wchannel:
			if !ok {
				// closed
				return nil
			}
			select {
			case <-t.closed:
				// double check closed
				return nil
			default:
			}

			if err = EncodeFrame(context.Background(), writer, fr); err != nil {
				return err
			}
			if delay >= 8 || len(t.wchannel) == 0 {
				delay = 0
				if err = t.conn.Writer().Flush(); err != nil {
					return err
				}
			} else {
				delay++
			}
		}
	}
}

// writeFrame is concurrent safe
func (t *transport) writeFrame(sframe streamFrame, meta IntHeader, ftype int32, data any) (err error) {
	var payload []byte
	if data != nil {
		// payload should be written nocopy
		payload, err = EncodePayload(context.Background(), data)
		if err != nil {
			return err
		}
	}
	frame := newFrame(sframe, meta, ftype, payload)
	t.wchannel <- frame
	return nil
}

func (t *transport) streamSend(ctx context.Context, sid int32, method string, wheader streamx.Header, res any) (err error) {
	if len(wheader) > 0 {
		err = t.streamSendHeader(sid, method, wheader)
		if err != nil {
			return err
		}
	}
	return t.writeFrame(
		streamFrame{sid: sid, method: method},
		nil, dataFrameType, res,
	)
}

func (t *transport) streamSendHeader(sid int32, method string, header streamx.Header) (err error) {
	return t.writeFrame(
		streamFrame{sid: sid, method: method, header: header},
		nil, headerFrameType, nil)
}

func (t *transport) streamCloseSend(sid int32, method string, trailer streamx.Trailer) (err error) {
	err = t.writeFrame(
		streamFrame{sid: sid, method: method, trailer: trailer},
		nil, trailerFrameType, nil,
	)
	if err != nil {
		return err
	}
	sio, ok := t.loadStreamIO(sid)
	if !ok {
		return nil
	}
	sio.closeSend()
	return nil
}

func (t *transport) streamRecv(ctx context.Context, sid int32, data any) (err error) {
	sio, ok := t.loadStreamIO(sid)
	if !ok {
		return io.EOF
	}
	f, err := sio.output(ctx)
	if err != nil {
		return err
	}
	err = DecodePayload(context.Background(), f.payload, data.(thrift.FastCodec))
	// payload will not be access after decode
	mcache.Free(f.payload)
	recycleFrame(f)
	return nil
}

func (t *transport) streamCloseRecv(s *stream) (err error) {
	sio, ok := t.loadStreamIO(s.sid)
	if !ok {
		return fmt.Errorf("stream not found in stream map: sid=%d", s.sid)
	}
	sio.closeRecv()
	return nil
}

func (t *transport) streamCancel(s *stream) (err error) {
	sio, ok := t.loadStreamIO(s.sid)
	if !ok {
		return fmt.Errorf("stream not found in stream map: sid=%d", s.sid)
	}
	sio.cancel()
	return nil
}

func (t *transport) streamClose(sid int32) (err error) {
	// remove stream from transport
	_, ok := t.streams.LoadAndDelete(sid)
	if !ok {
		return nil
	}
	atomic.AddInt32(&t.streamingFlag, -1)
	return nil
}

func (t *transport) IsStreaming() bool {
	return atomic.LoadInt32(&t.streamingFlag) > 0
}

var clientStreamID int32

// newStream create new stream on current connection
// it's typically used by client side
// newStream is concurrency safe
func (t *transport) newStream(
	ctx context.Context, method string, intHeader IntHeader, strHeader streamx.Header) (*stream, error) {
	if t.kind != clientTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}

	sid := atomic.AddInt32(&clientStreamID, 1)
	smode := t.sinfo.MethodInfo(method).StreamingMode()
	// create stream
	err := t.writeFrame(
		streamFrame{sid: sid, method: method, header: strHeader},
		intHeader, headerFrameType, nil,
	)
	if err != nil {
		return nil, err
	}
	s := newStream(ctx, t, smode, streamFrame{sid: sid, method: method})
	atomic.AddInt32(&t.streamingFlag, 1)
	return s, nil
}

// readStream wait for a new incoming stream on current connection
// it's typically used by server side
func (t *transport) readStream(ctx context.Context) (*stream, error) {
	if t.kind != serverTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}
READ:
	if len(t.scache) > 0 {
		s := t.scache[len(t.scache)-1]
		t.scache = t.scache[:len(t.scache)-1]
		atomic.AddInt32(&t.streamingFlag, 1)
		return s, nil
	}
	n, err := t.spipe.Read(ctx, t.scache[0:streamCacheSize])
	if err != nil {
		if errors.Is(err, container.ErrPipeEOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	if n == 0 {
		panic("Assert: N == 0 !")
	}
	t.scache = t.scache[:n]
	goto READ
}
