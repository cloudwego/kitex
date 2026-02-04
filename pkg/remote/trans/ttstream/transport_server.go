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
	"errors"
	"io"
	"net"
	"sync"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream/container"
)

// transport is used to read/write frames and disturbed frames to different streams
type serverTransport struct {
	conn netpoll.Connection
	// transport should operate directly on stream
	streams sync.Map                       // key=streamID val=stream
	scache  []*serverStream                // size is streamCacheSize
	spipe   *container.Pipe[*serverStream] // in-coming stream pipe

	mu        sync.Mutex // protect writer
	state     int32
	closedErr error
	writer    *writerBuffer

	closedTrigger chan struct{}
}

func newServerTransport(conn netpoll.Connection) *serverTransport {
	// TODO: let it configurable
	_ = conn.SetReadTimeout(0)
	t := &serverTransport{
		conn:          conn,
		streams:       sync.Map{},
		spipe:         container.NewPipe[*serverStream](),
		scache:        make([]*serverStream, 0, streamCacheSize),
		writer:        newWriterBuffer(conn.Writer()),
		closedTrigger: make(chan struct{}, 1),
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
					klog.Warnf("transport[%s] loop read err: %v", t.Addr(), err)
				}
				// if connection is closed by peer, loop read should return ErrConnClosed error,
				// so we should close transport here
				_ = t.Close(err)
			}
			t.closedTrigger <- struct{}{}
		}()
		err = t.loopRead()
	}, gofunc.NewBasicInfo("", addr))
	return t
}

func (t *serverTransport) Addr() net.Addr {
	return t.conn.RemoteAddr()
}

// Close will close transport and destroy all resource and goroutines when connection is disconnected
// when an exception is encountered and the transport needs to be closed,
// the exception is not nil and the currently surviving streams are aware of this exception.
func (t *serverTransport) Close(exception error) error {
	t.mu.Lock()
	if t.state == connStateClosed {
		closedErr := t.closedErr
		t.mu.Unlock()
		return closedErr
	}
	t.setClosedStateLocked(exception)
	t.mu.Unlock()

	t.releaseResources(exception)

	return exception
}

// setClosedStateLocked sets the closed state and closed reason.
// Must be called with t.mu held.
func (t *serverTransport) setClosedStateLocked(err error) {
	t.state = connStateClosed
	t.closedErr = err
}

func (t *serverTransport) releaseResources(err error) {
	klog.Debugf("server transport[%s] is closing", t.Addr())
	// close streams first
	ex := errConnectionClosedCancel.newBuilder().withCause(err)
	t.streams.Range(func(key, value any) bool {
		s := value.(*serverStream)
		_ = s.close(ex)
		return true
	})

	if cErr := t.conn.Close(); cErr != nil {
		klog.Infof("KITEX: ttstream serverTransport Close Connection failed, err: %v", cErr)
	}

	// then close stream pipes
	t.spipe.Close()
}

// WaitClosed waits for send loop and recv loop closed
func (t *serverTransport) WaitClosed() {
	<-t.closedTrigger
}

func (t *serverTransport) IsActive() bool {
	t.mu.Lock()
	isClosed := t.state == connStateClosed
	t.mu.Unlock()
	return !isClosed && t.conn.IsActive()
}

func (t *serverTransport) storeStream(s *serverStream) {
	t.streams.Store(s.sid, s)
}

func (t *serverTransport) loadStream(sid int32) (s *serverStream, ok bool) {
	val, ok := t.streams.Load(sid)
	if !ok {
		return s, false
	}
	s = val.(*serverStream)
	return s, true
}

func (t *serverTransport) deleteStream(sid int32) {
	// remove stream from transport
	t.streams.Delete(sid)
}

func (t *serverTransport) readFrame(reader bufiox.Reader) error {
	fr, err := DecodeFrame(context.Background(), reader)
	if err != nil {
		return err
	}
	defer recycleFrame(fr)

	var s *serverStream
	if fr.typ == headerFrameType {
		// server recv a header frame, we should create a new stream
		ctx, cancel := context.WithCancel(context.Background())
		ctx, cFunc := newContextWithCancelReason(ctx, cancel)
		s = newServerStream(ctx, t, fr.streamFrame)
		s.cancelFunc = cFunc
		t.storeStream(s)
		err = t.spipe.Write(context.Background(), s)
	} else {
		// load exist stream
		var ok bool
		s, ok = t.loadStream(fr.sid)
		if !ok {
			// there is a race condition that server handler returns and client sends rst frame concurrently.
			// then serverTransport would not find the target stream when receiving the rst frame.
			klog.Debugf("transport[%s] read a unknown stream: frame[%s]", t.Addr(), fr.String())
			// ignore unknown stream error
			err = nil
		} else {
			// process different frames
			switch fr.typ {
			case dataFrameType:
				// process data frame: decode and distribute data
				err = s.onReadDataFrame(fr)
			case trailerFrameType:
				// process trailer frame: close the stream read direction
				err = s.onReadTrailerFrame(fr)
			case rstFrameType:
				// process rst frame: finish the stream lifecycle
				err = s.onReadRstFrame(fr)
			}
		}
	}
	return err
}

func (t *serverTransport) loopRead() error {
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

// WriteFrame is concurrent safe
func (t *serverTransport) WriteFrame(fr *Frame) (err error) {
	var needRelease bool
	t.mu.Lock()
	defer func() {
		t.mu.Unlock()
		if needRelease {
			t.releaseResources(err)
		}
	}()
	if t.state == connStateClosed {
		err = t.closedErr
		return err
	}

	if err = encodeFrameAndFlush(context.Background(), t.writer, fr); err != nil {
		t.setClosedStateLocked(err)
		needRelease = true
		return err
	}
	recycleFrame(fr)

	return nil
}

func (t *serverTransport) CloseStream(sid int32) (err error) {
	t.deleteStream(sid)
	return nil
}

// ReadStream wait for a new incoming stream on current connection
// it's typically used by server side
func (t *serverTransport) ReadStream(ctx context.Context) (*serverStream, error) {
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
