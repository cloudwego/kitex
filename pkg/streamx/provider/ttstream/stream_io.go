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

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
)

// streamIO is an abstraction layer for stream level IO operations
type streamIO struct {
	pipe      *container.Pipe[streamIOMsg]
	cache     [1]streamIOMsg
	exception error // once has exception, the stream should not work normally again
}

type streamIOMsg struct {
	payload   []byte
	exception error
}

func newStreamIO() *streamIO {
	sio := new(streamIO)
	sio.pipe = container.NewPipe[streamIOMsg]()
	return sio
}

func (s *streamIO) input(ctx context.Context, payload []byte) {
	err := s.pipe.Write(ctx, streamIOMsg{payload: payload})
	if err != nil {
		klog.Errorf("pipe write failed: %v", err)
	}
}

func (s *streamIO) output(ctx context.Context) (payload []byte, err error) {
	if s.exception != nil {
		return nil, s.exception
	}

	n, err := s.pipe.Read(ctx, s.cache[:])
	if err != nil {
		if errors.Is(err, container.ErrPipeEOF) {
			err = io.EOF
		}
		s.exception = err
		return nil, s.exception
	}
	if n == 0 {
		s.exception = io.EOF
		return nil, s.exception
	}
	msg := s.cache[0]
	if msg.exception != nil {
		s.exception = msg.exception
		return nil, s.exception
	}
	return msg.payload, nil
}

func (s *streamIO) cancel() {
	s.pipe.Cancel()
}

func (s *streamIO) close(exception error) {
	if exception != nil {
		_ = s.pipe.Write(context.Background(), streamIOMsg{exception: exception})
	}
	s.pipe.Close()
}
