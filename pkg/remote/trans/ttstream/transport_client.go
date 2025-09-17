/*
 * Copyright 2025 CloudWeGo Authors
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
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream/container"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/utils"
)

// ticker is used to manage cleaning canceled stream task.
// it triggers and cleans up actively cancelled streams every 5s.
// Streaming QPS is generally not too high so that using the Sync SharedTicker to reduce
// the overhead of goroutines in a multi-connection scenario.
//
// This is a workaround: when the minimum Go version supports 1.21, use `context.AfterFunc` instead.
var ticker = utils.NewSyncSharedTicker(5 * time.Second)

type clientTransport struct {
	conn          netpoll.Connection
	pool          transPool
	streams       sync.Map                       // key=streamID val=clientStream
	scache        []*clientStream                // size is streamCacheSize
	spipe         *container.Pipe[*clientStream] // in-coming clientStream pipe
	fpipe         *container.Pipe[*Frame]        // out-coming frame pipe
	closedFlag    int32
	closedTrigger chan struct{}
}

func newClientTransport(conn netpoll.Connection, pool transPool) *clientTransport {
	// TODO: let it configurable
	_ = conn.SetReadTimeout(0)
	t := &clientTransport{
		conn:          conn,
		pool:          pool,
		streams:       sync.Map{},
		spipe:         container.NewPipe[*clientStream](),
		scache:        make([]*clientStream, 0, streamCacheSize),
		fpipe:         container.NewPipe[*Frame](),
		closedTrigger: make(chan struct{}, 2),
	}
	addr := ""
	if t.Addr() != nil {
		addr = t.Addr().String()
	}
	gofunc.RecoverGoFuncWithInfo(context.Background(), func() {
		var err error
		defer func() {
			if err != nil {
				if !isIgnoreError(err) {
					klog.Warnf("clientTransport[%s] loop read err: %v", t.Addr(), err)
				}
				// if connection is closed by peer, loop read should return ErrConnClosed error,
				// so we should close transport here
				_ = t.Close(err)
			}
			t.closedTrigger <- struct{}{}
		}()
		err = t.loopRead()
	}, gofunc.NewBasicInfo("", addr))
	gofunc.RecoverGoFuncWithInfo(context.Background(), func() {
		var err error
		defer func() {
			if err != nil {
				if !isIgnoreError(err) {
					klog.Warnf("clientTransport[%s] loop write err: %v", t.Addr(), err)
				}
				_ = t.Close(err)
			}
			t.closedTrigger <- struct{}{}
		}()
		err = t.loopWrite()
	}, gofunc.NewBasicInfo("", addr))

	// add to stream cleanup ticker
	ticker.Add(t)

	return t
}

func (t *clientTransport) Addr() net.Addr {
	return t.conn.LocalAddr()
}

// Close will close transport and destroy all resource and goroutines when transPool discard the transport
// when an exception is encountered and the transport needs to be closed,
// the exception is not nil and the currently surviving streams are aware of this exception.
func (t *clientTransport) Close(exception error) (err error) {
	if !atomic.CompareAndSwapInt32(&t.closedFlag, 0, 1) {
		return nil
	}
	klog.Debugf("client transport[%s] is closing", t.Addr())
	// close streams first
	t.streams.Range(func(key, value any) bool {
		s := value.(*clientStream)
		s.close(exception, false, "")
		return true
	})
	// then close stream and frame pipes
	t.spipe.Close()
	t.fpipe.Close()

	// remove cleanup stream task from ticker to avoid goroutine leak
	ticker.Delete(t)

	return err
}

// WaitClosed waits for send loop and recv loop closed
func (t *clientTransport) WaitClosed() {
	<-t.closedTrigger
	<-t.closedTrigger
}

func (t *clientTransport) IsActive() bool {
	return atomic.LoadInt32(&t.closedFlag) == 0 && t.conn.IsActive()
}

func (t *clientTransport) storeStream(s *clientStream) {
	t.streams.Store(s.sid, s)
}

func (t *clientTransport) loadStream(sid int32) (s *clientStream, ok bool) {
	val, ok := t.streams.Load(sid)
	if !ok {
		return s, false
	}
	s = val.(*clientStream)
	return s, true
}

func (t *clientTransport) deleteStream(sid int32) {
	// remove stream from transport
	t.streams.Delete(sid)
}

func (t *clientTransport) readFrame(reader bufiox.Reader) error {
	fr, err := DecodeFrame(context.Background(), reader)
	if err != nil {
		return err
	}
	defer recycleFrame(fr)

	var s *clientStream
	// load exist stream
	var ok bool
	s, ok = t.loadStream(fr.sid)
	if !ok {
		klog.Errorf("transport[%s] read a unknown stream: frame[%s]", t.Addr(), fr)
		// ignore unknown stream error
		err = nil
	} else {
		// process different frames
		switch fr.typ {
		case metaFrameType:
			err = s.onReadMetaFrame(fr)
		case headerFrameType:
			// process header frame for client transport
			err = s.onReadHeaderFrame(fr)
		case dataFrameType:
			// process data frame: decode and distribute data
			err = s.onReadDataFrame(fr)
		case trailerFrameType:
			// process trailer frame: finish the stream lifecycle
			err = s.onReadTrailerFrame(fr)
		case rstFrameType:
			// process rst frame: finish the stream lifecycle
			err = s.onReadRstFrame(fr)
		}
	}
	return err
}

func (t *clientTransport) loopRead() error {
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

func (t *clientTransport) loopWrite() error {
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
			if err = EncodeFrame(context.Background(), writer, fr); err != nil {
				return err
			}
			recycleFrame(fr)
		}
		if err = writer.Flush(); err != nil {
			return err
		}
	}
}

// WriteFrame is concurrent safe
func (t *clientTransport) WriteFrame(fr *Frame) (err error) {
	return t.fpipe.Write(context.Background(), fr)
}

func (t *clientTransport) CloseStream(sid int32) (err error) {
	t.deleteStream(sid)
	// clientSide may require to return the transport to transPool
	if t.pool != nil {
		t.pool.Put(t)
	}
	return nil
}

var clientStreamID int32

// stream id can be negative
func genStreamID() int32 {
	// here have a really rare case that one connection get two same stream id when exist (2*max_int32) streams,
	// but it just happens in theory because in real world, no service can process soo many streams in the same time.
	sid := atomic.AddInt32(&clientStreamID, 1)
	// we preserve streamId=0 for connection level control frame in the future.
	if sid == 0 {
		sid = atomic.AddInt32(&clientStreamID, 1)
	}
	return sid
}

// WriteStream create new stream on current connection
// it's typically used by client side
// newStream is concurrency safe
func (t *clientTransport) WriteStream(
	ctx context.Context, s *clientStream, intHeader IntHeader, strHeader streaming.Header,
) error {
	t.storeStream(s)
	// send create stream request for server
	fr := newFrame(streamFrame{sid: s.sid, method: s.method, header: strHeader, meta: intHeader}, headerFrameType, nil)
	if err := t.WriteFrame(fr); err != nil {
		return err
	}
	return nil
}
