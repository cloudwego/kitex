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

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
)

type streamIO struct {
	ctx     context.Context
	trigger chan struct{}
	stream  *stream
	// eofFlag == 2 when both parties send trailers
	eofFlag int32
	// eofCallback will be called when eofFlag == 2
	// eofCallback will not be called if stream is not be ended in a normal way
	eofCallback func()
	fpipe       *container.Pipe[*Frame]
	fcache      [1]*Frame
}

func newStreamIO(ctx context.Context, s *stream) *streamIO {
	sio := new(streamIO)
	sio.ctx = ctx
	sio.trigger = make(chan struct{})
	sio.stream = s
	sio.fpipe = container.NewPipe[*Frame]()
	return sio
}

func (s *streamIO) setEOFCallback(f func()) {
	s.eofCallback = f
}

func (s *streamIO) input(ctx context.Context, f *Frame) {
	err := s.fpipe.Write(ctx, f)
	if err != nil {
		klog.Errorf("fpipe write failed: %v", err)
	}
}

func (s *streamIO) output(ctx context.Context) (f *Frame, err error) {
	n, err := s.fpipe.Read(ctx, s.fcache[:])
	if err != nil {
		if errors.Is(err, container.ErrPipeEOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	if n == 0 {
		return nil, io.EOF
	}
	return s.fcache[0], nil
}

func (s *streamIO) closeRecv() {
	s.fpipe.Close()
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
	s.fpipe.Cancel()
}
