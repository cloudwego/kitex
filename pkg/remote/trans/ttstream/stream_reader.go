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
	"io"

	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream/internal/container"
)

// streamReader is an abstraction layer for stream level IO operations
type streamReader struct {
	pipe      *container.Pipe[streamMsg]
	cache     [1]streamMsg
	exception error // once has exception, the stream should not work normally again
}

type streamMsg struct {
	payload   []byte
	exception error
}

func newStreamReader(ctx context.Context, callback container.CtxDoneCallback) *streamReader {
	sio := new(streamReader)
	sio.pipe = container.NewPipe[streamMsg](ctx, callback)
	return sio
}

func (s *streamReader) input(payload []byte) {
	s.pipe.Write(streamMsg{payload: payload})
}

// output would return err in the following scenarios:
// - pipe finished: io.EOF
// - ctx Done() triggered: callback error or ctx.Err()
// - trailer frame contains err: streamMsg.exception
func (s *streamReader) output() (payload []byte, err error) {
	if s.exception != nil {
		return nil, s.exception
	}

	n, err := s.pipe.Read(s.cache[:])
	return s.handleReadResult(n, err)
}

// outputWithCtx additionally checks a per-read ctx alongside the pipe's own ctx.
func (s *streamReader) outputWithCtx(ctx context.Context, perReadCallback container.CtxDoneCallback) (payload []byte, err error) {
	if s.exception != nil {
		return nil, s.exception
	}

	n, err := s.pipe.ReadCtx(ctx, perReadCallback, s.cache[:])
	return s.handleReadResult(n, err)
}

func (s *streamReader) handleReadResult(n int, err error) ([]byte, error) {
	if err != nil {
		if checkCanRetry(err) {
			// do not set s.exception so that users can continue recv
			return nil, err
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

func (s *streamReader) close(exception error) {
	if exception != nil {
		s.pipe.Write(streamMsg{exception: exception})
	}
	s.pipe.Close()
}
