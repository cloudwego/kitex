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
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/gopkg/bufiox"
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
	frameCacheSize  = 32
)

var _ Object = (*transport)(nil)

type transport struct {
	kind     int32
	sinfo    *serviceinfo.ServiceInfo
	conn     netpoll.Connection
	reader   bufiox.Reader
	writer   bufiox.Writer
	streams  sync.Map                 // key=streamID val=streamIO
	scache   []*stream                // size is streamCacheSize
	spipe    *container.Pipe[*stream] // in-coming stream channel
	fpipe    *container.Pipe[*Frame]  // out-coming frame channel
	fcache   []*Frame
	ftrigger int32

	// for scavenger check
	lastActive atomic.Value // time.Time
}

func newTransport(kind int32, sinfo *serviceinfo.ServiceInfo, conn netpoll.Connection) *transport {
	_ = conn.SetDeadline(time.Now().Add(time.Hour))
	cbuffer := newConnBuffer(conn)
	t := &transport{
		kind:    kind,
		sinfo:   sinfo,
		conn:    conn,
		reader:  cbuffer,
		writer:  cbuffer,
		streams: sync.Map{},
		spipe:   container.NewPipe[*stream](),
		scache:  make([]*stream, 0, streamCacheSize),
		fpipe:   container.NewPipe[*Frame](),
		fcache:  make([]*Frame, 0, frameCacheSize),
	}
	go func() {
		err := t.loopRead()
		if err != nil && errors.Is(err, io.EOF) {
			klog.Warnf("trans loop read err: %v", err)
		}
	}()
	go func() {
		err := t.loopWrite()
		if err != nil && errors.Is(err, io.EOF) {
			klog.Warnf("trans loop write err: %v", err)
			return
		}
	}()
	return t
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
		now := time.Now()
		t.lastActive.Store(now)

		// decode frame
		fr, err := DecodeFrame(context.Background(), t.reader)
		if err != nil {
			return err
		}

		switch fr.typ {
		case metaFrameType:
			sio, ok := t.loadStreamIO(fr.sid)
			if !ok {
				return fmt.Errorf("transport[%d] read a unknown stream meta: sid=%d", t.kind, fr.sid)
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
				t.spipe.Write(s)
			case clientTransport:
				// Header Frame: client recv header
				sio, ok := t.loadStreamIO(fr.sid)
				if !ok {
					return fmt.Errorf("transport[%d] read a unknown stream header: sid=%d", t.kind, fr.sid)
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
				return fmt.Errorf("transport[%d] read a unknown stream data: sid=%d", t.kind, fr.sid)
			}
			sio.input(fr)
		case trailerFrameType:
			// Trailer Frame: recv trailer, Close read direction
			sio, ok := t.loadStreamIO(fr.sid)
			if !ok {
				return fmt.Errorf("transport[%d] read a unknown stream trailer: sid=%d", t.kind, fr.sid)
			}
			if err = sio.stream.readTrailer(fr.trailer); err != nil {
				return err
			}
		}
	}
}

func (t *transport) writeFrame(frame *Frame) error {
	err := EncodeFrame(context.Background(), t.writer, frame)
	if err != nil {
		return err
	}
	return t.writer.Flush()
}

func (t *transport) writeTrigger() {
	if !atomic.CompareAndSwapInt32(&t.ftrigger, 0, 1) {
		return
	}
	go t.loopWrite()
}

func (t *transport) loopWrite() error {
	frames := make([]*Frame, frameCacheSize)
	for {
		now := time.Now()
		t.lastActive.Store(now)

		n, err := t.fpipe.Read(frames)
		if err != nil {
			if errors.Is(err, container.ErrPipeEOF) {
				return nil
			}
			return err
		}
		for i := 0; i < n; i++ {
			if err := t.writeFrame(frames[i]); err != nil {
				return err
			}
		}
	}
}

func (t *transport) Available() bool {
	v := t.lastActive.Load()
	if v == nil {
		return true
	}
	lastActive := v.(time.Time)
	// let unavailable time configurable
	return time.Now().Sub(lastActive) < time.Minute*10
}

func (t *transport) Close() (err error) {
	klog.Warnf("transport[%s] is closing", t.conn.LocalAddr())
	t.spipe.Close()
	t.fpipe.Close()
	err = t.conn.Close()
	return err
}

func (t *transport) streamSend(sid int32, method string, wheader streamx.Header, payload []byte) (err error) {
	if len(wheader) > 0 {
		err = t.streamSendHeader(sid, method, wheader)
		if err != nil {
			return err
		}
	}
	f := newFrame(streamFrame{sid: sid, method: method}, dataFrameType, payload)
	err = t.fpipe.Write(f)
	return err
}

func (t *transport) streamSendHeader(sid int32, method string, header streamx.Header) (err error) {
	f := newFrame(streamFrame{sid: sid, method: method, header: header}, headerFrameType, []byte{})
	err = t.fpipe.Write(f)
	return err
}

func (t *transport) streamSendTrailer(sid int32, method string, trailer streamx.Trailer) (err error) {
	f := newFrame(streamFrame{sid: sid, method: method, trailer: trailer}, trailerFrameType, []byte{})
	err = t.fpipe.Write(f)
	return err
}

func (t *transport) streamRecv(sid int32) (payload []byte, err error) {
	sio, ok := t.loadStreamIO(sid)
	if !ok {
		return nil, io.EOF
	}
	f, err := sio.output()
	if err != nil {
		return nil, err
	}
	payload = f.payload
	recycleFrame(f)
	return payload, nil
}

func (t *transport) streamCloseRecv(s *stream) (err error) {
	sio, ok := t.loadStreamIO(s.sid)
	if !ok {
		return fmt.Errorf("stream not found in stream map: sid=%d", s.sid)
	}
	sio.eof()
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

func (t *transport) streamClose(s *stream) (err error) {
	// remove stream from transport
	t.streams.Delete(s.sid)
	return nil
}

var clientStreamID int32

// newStream create new stream on current connection
// it's typically used by client side
// newStream is concurrency safe
func (t *transport) newStream(
	ctx context.Context, method string, header streamx.Header) (*stream, error) {
	if t.kind != clientTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}
	sid := atomic.AddInt32(&clientStreamID, 1)
	smode := t.sinfo.MethodInfo(method).StreamingMode()
	smeta := streamFrame{
		sid:    sid,
		method: method,
		header: header,
	}
	f := newFrame(smeta, headerFrameType, []byte{})
	s := newStream(ctx, t, smode, smeta)
	err := t.fpipe.Write(f) // create stream
	if err != nil {
		return nil, err
	}
	return s, nil
}

// readStream wait for a new incoming stream on current connection
// it's typically used by server side
func (t *transport) readStream() (*stream, error) {
	if t.kind != serverTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}
READ:
	if len(t.scache) > 0 {
		s := t.scache[len(t.scache)-1]
		t.scache = t.scache[:len(t.scache)-1]
		return s, nil
	}
	n, err := t.spipe.Read(t.scache[0:streamCacheSize])
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
