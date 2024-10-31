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
	"github.com/cloudwego/netpoll"

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

// transport is used to read/write frames and disturbed frames to different streams
type transport struct {
	kind  int32
	sinfo *serviceinfo.ServiceInfo
	conn  netpoll.Connection
	pool  transPool
	// transport should operate directly on stream
	streams       sync.Map                 // key=streamID val=stream
	scache        []*stream                // size is streamCacheSize
	spipe         *container.Pipe[*stream] // in-coming stream pipe
	fpipe         *container.Pipe[*Frame]  // out-coming frame pipe
	closedFlag    int32
	closedTrigger chan struct{}

	metaHandler MetaFrameHandler
}

func newTransport(kind int32, sinfo *serviceinfo.ServiceInfo, conn netpoll.Connection, pool transPool) *transport {
	// stream max idle session is 10 minutes.
	// TODO: let it configurable
	_ = conn.SetReadTimeout(time.Minute * 10)
	t := &transport{
		kind:          kind,
		sinfo:         sinfo,
		conn:          conn,
		pool:          pool,
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

func (t *transport) setMetaFrameHandler(h MetaFrameHandler) {
	t.metaHandler = h
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
		_ = s.closeSend(exception)
		_ = s.closeRecv(exception)
		t.streams.Delete(key)
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

func (t *transport) storeStream(s *stream) {
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

func (t *transport) deleteStream(sid int32) {
	// remove stream from transport
	t.streams.Delete(sid)
}

func (t *transport) recycle() {
	if t.pool != nil {
		t.pool.Put(t)
	}
}

func (t *transport) readFrame(reader bufiox.Reader) error {
	fr, err := DecodeFrame(context.Background(), reader)
	if err != nil {
		return err
	}
	defer recycleFrame(fr)
	klog.Debugf("transport[%d] DecodeFrame: fr=%v", t.kind, fr)

	var s *stream
	if fr.typ == headerFrameType && t.kind == serverTransport {
		// server recv a header frame, we should create a new stream
		smode := t.sinfo.MethodInfo(fr.method).StreamingMode()
		s = newStream(context.Background(), t, smode, fr.streamFrame)
		t.storeStream(s)
		err = t.spipe.Write(context.Background(), s)
	} else {
		// load exist stream
		var ok bool
		s, ok = t.loadStream(fr.sid)
		if !ok {
			klog.Errorf(
				"transport[%d] read a unknown stream: ftype=%d sid=%d fmethod=%s ",
				t.kind, fr.typ, fr.sid, fr.method,
			)
			// ignore unknown stream error
			err = nil
		} else {
			// process different frames
			switch fr.typ {
			case metaFrameType:
				// process meta frame if metaHandler registered
				if t.metaHandler != nil {
					err = t.metaHandler.OnMetaFrame(s, fr.meta, fr.header, fr.payload)
				}
			case headerFrameType:
				// process header frame for client transport
				err = s.onReadHeaderFrame(fr)
			case dataFrameType:
				// process data frame: decode and distribute data
				err = s.onReadDataFrame(fr)
			case trailerFrameType:
				// process trailer frame: close the stream read direction
				err = s.onReadTrailerFrame(fr)
			}
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
func (t *transport) writeFrame(sframe streamFrame, ftype int32, payload []byte) (err error) {
	frame := newFrame(sframe, ftype, payload)
	return t.fpipe.Write(context.Background(), frame)
}

var clientStreamID int32

// WriteStream create new stream on current connection
// it's typically used by client side
// newStream is concurrency safe
func (t *transport) WriteStream(
	ctx context.Context, method string, intHeader IntHeader, strHeader streamx.Header,
) (*stream, error) {
	if t.kind != clientTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}

	sid := atomic.AddInt32(&clientStreamID, 1)
	smode := t.sinfo.MethodInfo(method).StreamingMode()
	// create stream
	err := t.writeFrame(
		streamFrame{sid: sid, method: method, header: strHeader, meta: intHeader}, headerFrameType, nil,
	)
	if err != nil {
		return nil, err
	}
	s := newStream(ctx, t, smode, streamFrame{sid: sid, method: method})
	t.storeStream(s)
	return s, nil
}

// ReadStream wait for a new incoming stream on current connection
// it's typically used by server side
func (t *transport) ReadStream(ctx context.Context) (*stream, error) {
	if t.kind != serverTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}
READ:
	if len(t.scache) > 0 {
		s := t.scache[len(t.scache)-1]
		t.scache = t.scache[:len(t.scache)-1]
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
