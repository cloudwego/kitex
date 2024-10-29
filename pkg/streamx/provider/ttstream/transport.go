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

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/rpcinfo"

	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
)

const (
	clientTransport int32 = 1
	serverTransport int32 = 2

	streamCacheSize = 32
	frameCacheSize  = 256
)

func isIgnoreError(err error) bool {
	return errors.Is(err, netpoll.ErrEOF) || errors.Is(err, io.EOF) || errors.Is(err, netpoll.ErrConnClosed)
}

type transport struct {
	kind  int32
	sinfo *serviceinfo.ServiceInfo
	conn  netpoll.Connection
	// transport should operate directly on stream
	streams       sync.Map                 // key=streamID val=stream
	scache        []*stream                // size is streamCacheSize
	spipe         *container.Pipe[*stream] // in-coming stream pipe
	fpipe         *container.Pipe[*Frame]  // out-coming frame pipe
	closedFlag    int32
	streamingFlag int32 // flag == 0 means there is no active stream on transport
	closedTrigger chan struct{}
}

func newTransport(kind int32, sinfo *serviceinfo.ServiceInfo, conn netpoll.Connection) *transport {
	// stream max idle session is 10 minutes.
	// TODO: let it configurable
	_ = conn.SetReadTimeout(time.Minute * 10)
	t := &transport{
		kind:          kind,
		sinfo:         sinfo,
		conn:          conn,
		streams:       sync.Map{},
		spipe:         container.NewPipe[*stream](),
		scache:        make([]*stream, 0, streamCacheSize),
		fpipe:         container.NewPipe[*Frame](),
		closedTrigger: make(chan struct{}, 2),
	}
	go func() {
		defer func() {
			t.closedTrigger <- struct{}{}
		}()
		err := t.loopRead()
		if err != nil {
			if !isIgnoreError(err) {
				klog.Warnf("transport[%d] loop read err: %v", t.kind, err)
			}
			// if connection is closed by peer, loop read should return ErrConnClosed error,
			// so we should close transport here
			_ = t.Close(err)
		}
	}()
	go func() {
		defer func() {
			t.closedTrigger <- struct{}{}
		}()
		err := t.loopWrite()
		if err != nil {
			if !isIgnoreError(err) {
				klog.Warnf("transport[%d] loop write err: %v", t.kind, err)
			}
			_ = t.Close(err)
		}
	}()
	return t
}

// Close will close transport and destroy all resource and goroutines
// server close transport when connection is disconnected
// client close transport when transPool discard the transport
// when an exception is encountered and the transport needs to be closed,
// the exception is not nil and the currently surviving streams are aware of this exception.
func (t *transport) Close(exception error) (err error) {
	if !atomic.CompareAndSwapInt32(&t.closedFlag, 0, 1) {
		return nil
	}
	switch t.kind {
	case clientTransport:
		klog.Debugf("transport[%d-%s] is closing", t.kind, t.conn.LocalAddr())
	case serverTransport:
		klog.Debugf("transport[%d-%s] is closing", t.kind, t.conn.RemoteAddr())
	}
	t.spipe.Close()
	t.fpipe.Close()
	t.streams.Range(func(key, value any) bool {
		s := value.(*stream)
		s.close(exception)
		return true
	})
	return err
}

func (t *transport) WaitClosed() {
	<-t.closedTrigger
	<-t.closedTrigger
}

func (t *transport) IsActive() bool {
	return atomic.LoadInt32(&t.closedFlag) == 0 && t.conn.IsActive()
}

func (t *transport) storeStream(ctx context.Context, s *stream) {
	s.sio = newStreamIO(ctx)
	copts := streamxcallopt.GetCallOptionsFromCtx(ctx)
	if copts != nil && copts.StreamCloseCallback != nil {
		s.sio.closeCallback = copts.StreamCloseCallback
	}
	t.streams.Store(s.sid, s)
}

func (t *transport) loadStream(sid int32) (s *stream, ok bool) {
	val, ok := t.streams.Load(sid)
	if !ok {
		return s, false
	}
	s = val.(*stream)
	return s, true
}

func (t *transport) readFrame(reader bufiox.Reader) error {
	fr, err := DecodeFrame(context.Background(), reader)
	if err != nil {
		return err
	}
	defer recycleFrame(fr)
	klog.Debugf("transport[%d] DecodeFrame: fr=%v", t.kind, fr)

	switch fr.typ {
	case metaFrameType:
		s, ok := t.loadStream(fr.sid)
		if ok {
			err = s.readMetaFrame(fr.meta, fr.header, fr.payload)
		} else {
			klog.Errorf("transport[%d] read a unknown stream meta: sid=%d", t.kind, fr.sid)
		}
	case headerFrameType:
		switch t.kind {
		case serverTransport:
			// Header Frame: server recv a new stream
			smode := t.sinfo.MethodInfo(fr.method).StreamingMode()
			s := newStream(t, smode, fr.streamFrame)
			t.storeStream(context.Background(), s)
			err = t.spipe.Write(context.Background(), s)
		case clientTransport:
			// Header Frame: client recv header
			s, ok := t.loadStream(fr.sid)
			if ok {
				if sErr := s.readHeader(fr.header); sErr != nil {
					s.close(sErr)
				}
			} else {
				klog.Errorf("transport[%d] read a unknown stream header: sid=%d header=%v",
					t.kind, fr.sid, fr.header)
			}
		}
	case dataFrameType:
		// Data Frame: decode and distribute data
		s, ok := t.loadStream(fr.sid)
		if ok {
			s.sio.input(context.Background(), streamIOMsg{payload: fr.payload})
		} else {
			klog.Errorf("transport[%d] read a unknown stream data: sid=%d", t.kind, fr.sid)
		}
	case trailerFrameType:
		// Trailer Frame: recv trailer, Close read direction
		s, ok := t.loadStream(fr.sid)
		if ok {
			if sErr := s.readTrailerFrame(fr); sErr != nil {
				s.close(sErr)
			}
		} else {
			// client recv an unknown trailer is in exception,
			// because the client stream may already be GCed,
			// but the connection is still active so peer server can send a trailer
			klog.Errorf("transport[%d] read a unknown stream trailer: sid=%d trailer=%v",
				t.kind, fr.sid, fr.trailer)
		}
	}
	return err
}

func (t *transport) loopRead() error {
	reader := newReaderBuffer(t.conn.Reader())
	for {
		err := t.readFrame(reader)
		// judge whether it is connection-level error
		// read frame return an un-recovered error, so we should close the transport
		if err != nil {
			return err
		}
	}
}

func (t *transport) loopWrite() error {
	defer func() {
		// loop write should help to close connection
		_ = t.conn.Close()
	}()
	writer := newWriterBuffer(t.conn.Writer())
	fcache := make([]*Frame, frameCacheSize)
	// Important note:
	// loopWrite may cannot find stream by sid since it may send trailer and delete sid from streams
	for {
		n, err := t.fpipe.Read(context.Background(), fcache)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.EOF
		}
		for i := 0; i < n; i++ {
			fr := fcache[i]
			klog.Debugf("transport[%d] EncodeFrame: fr=%v IsActive=%v", t.kind, fr, t.conn.IsActive())
			if err = EncodeFrame(context.Background(), writer, fr); err != nil {
				return err
			}
			recycleFrame(fr)
		}
		if err = t.conn.Writer().Flush(); err != nil {
			return err
		}
	}
}

// writeFrame is concurrent safe
func (t *transport) writeFrame(sframe streamFrame, meta IntHeader, ftype int32, payload []byte) (err error) {
	frame := newFrame(sframe, meta, ftype, payload)
	return t.fpipe.Write(context.Background(), frame)
}

func (t *transport) streamSend(ctx context.Context, s *stream, res any) (err error) {
	if s.isClosed() {
		return s.sio.exception
	}
	if s.isSendFinished() {
		return io.EOF
	}
	if len(s.wheader) > 0 {
		err = t.streamSendHeader(s, s.wheader)
		if err != nil {
			return err
		}
	}
	payload, err := EncodePayload(ctx, res)
	if err != nil {
		return err
	}
	// tracing
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri != nil && ri.Stats() != nil {
		if rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats()); rpcStats != nil {
			rpcStats.IncrSendSize(uint64(len(payload)))
		}
	}
	return t.writeFrame(
		streamFrame{sid: s.sid, method: s.method},
		nil, dataFrameType, payload,
	)
}

func (t *transport) streamSendHeader(s *stream, header streamx.Header) (err error) {
	return t.writeFrame(
		streamFrame{sid: s.sid, method: s.method, header: header},
		nil, headerFrameType, nil)
}

func (t *transport) streamCloseSend(s *stream, trailer streamx.Trailer, exception tException) (err error) {
	var payload []byte
	if exception != nil {
		payload, err = EncodeException(context.Background(), s.method, s.sid, exception)
		if err != nil {
			return err
		}
	}
	err = t.writeFrame(
		streamFrame{sid: s.sid, method: s.method, trailer: trailer},
		nil, trailerFrameType, payload,
	)
	if err != nil {
		return err
	}
	s.sio.closeSend()
	return nil
}

func (t *transport) streamRecv(ctx context.Context, s *stream, data any) (err error) {
	msg, err := s.sio.output(ctx)
	if err != nil {
		return err
	}
	err = DecodePayload(context.Background(), msg.payload, data.(thrift.FastCodec))
	// payload will not be access after decode
	mcache.Free(msg.payload)

	// tracing
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri != nil && ri.Stats() != nil {
		if rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats()); rpcStats != nil {
			rpcStats.IncrRecvSize(uint64(len(msg.payload)))
		}
	}
	return err
}

func (t *transport) streamCloseRecv(s *stream, exception error) error {
	if exception != nil {
		s.close(exception)
	} else {
		s.sio.closeRecv()
	}
	return nil
}

func (t *transport) streamDelete(sid int32) {
	// remove stream from transport
	_, ok := t.streams.LoadAndDelete(sid)
	if !ok {
		return
	}
	atomic.AddInt32(&t.streamingFlag, -1)
}

func (t *transport) IsStreaming() bool {
	return atomic.LoadInt32(&t.streamingFlag) > 0
}

var clientStreamID int32

// newStream create new stream on current connection
// it's typically used by client side
// newStream is concurrency safe
func (t *transport) newStream(
	ctx context.Context, method string, intHeader IntHeader, strHeader streamx.Header,
) (*stream, error) {
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
	s := newStream(t, smode, streamFrame{sid: sid, method: method})
	t.storeStream(ctx, s)
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
