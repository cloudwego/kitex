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
	"io"
	"sync/atomic"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
)

type streamIOMsg struct {
	reader    bufiox.Reader
	payload   []byte
	exception error
}

type streamIO struct {
	ctx       context.Context
	trigger   chan struct{}
	stream    *stream
	pipe      *container.Pipe[streamIOMsg]
	cache     [1]streamIOMsg
	exception error // once has exception, the stream should not work normally again
	// eofFlag == 2 when both parties send trailers
	eofFlag int32
	// eofCallback will be called when eofFlag == 2
	// eofCallback will not be called if stream is not be ended in a normal way
	eofCallback func()
}

func newStreamIO(ctx context.Context, s *stream) *streamIO {
	sio := new(streamIO)
	sio.ctx = ctx
	sio.trigger = make(chan struct{})
	sio.stream = s
	sio.pipe = container.NewPipe[streamIOMsg]()
	return sio
}

func (s *streamIO) setEOFCallback(f func()) {
	s.eofCallback = f
}

func (s *streamIO) input(ctx context.Context, msg streamIOMsg) {
	err := s.pipe.Write(ctx, msg)
	if err != nil {
		klog.Errorf("pipe write failed: %v", err)
	}
}

func (s *streamIO) output(ctx context.Context) (msg streamIOMsg, err error) {
	if s.exception != nil {
		return msg, s.exception
	}

	n, err := s.pipe.Read(ctx, s.cache[:])
	if err != nil {
		if errors.Is(err, container.ErrPipeEOF) {
			err = io.EOF
		}
		s.exception = err
		return msg, s.exception
	}
	if n == 0 {
		s.exception = io.EOF
		return msg, s.exception
	}
	msg = s.cache[0]
	if msg.exception != nil {
		s.exception = msg.exception
		return msg, s.exception
	}
	return msg, nil
}

func (s *streamIO) closeRecv() {
	s.pipe.Close()
	if atomic.AddInt32(&s.eofFlag, 1) == 2 && s.eofCallback != nil {
		s.eofCallback()
	}
}

func (s *streamIO) closeSend() {
	if atomic.AddInt32(&s.eofFlag, 1) == 2 && s.eofCallback != nil {
		s.eofCallback()
	}
}

func (s *streamIO) cancel() {
	s.pipe.Cancel()
}
