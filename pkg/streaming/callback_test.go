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

package streaming

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

type mockReq struct{}

type mockRes struct{}

type mockClientStream struct {
	ClientStream
	doFinish func(error)
	cancel   func(error)
}

func (m *mockClientStream) DoFinish(err error) {
	m.doFinish(err)
}

func (m *mockClientStream) Cancel(err error) {
	m.cancel(err)
}

type mockServerStreamingClient[Res interface{}] struct {
	*mockClientStream
}

func (cli *mockServerStreamingClient[Res]) Recv(ctx context.Context) (*Res, error) {
	return nil, nil
}

type mockClientStreamingClient[Req, Res interface{}] struct {
	*mockClientStream
}

func (cli *mockClientStreamingClient[Req, Res]) Send(ctx context.Context, req *Req) error {
	return nil
}

func (cli *mockClientStreamingClient[Req, Res]) CloseAndRecv(ctx context.Context) (*Res, error) {
	return nil, nil
}

type mockBidiStreamingClient[Req, Res interface{}] struct {
	*mockClientStream
}

func (cli *mockBidiStreamingClient[Req, Res]) Recv(ctx context.Context) (*Res, error) {
	return nil, nil
}

func (cli *mockBidiStreamingClient[Req, Res]) Send(ctx context.Context, req *Req) error {
	return nil
}

func TestHandleStreaming(t *testing.T) {
	// server-streaming
	t.Run("server-streaming", func(t *testing.T) {
		var testErr error
		st := &mockServerStreamingClient[mockRes]{
			mockClientStream: &mockClientStream{
				doFinish: func(err error) {
					test.Assert(t, err == testErr, err)
				},
				cancel: func(err error) {
					test.Assert(t, err == testErr, err)
				},
			},
		}
		// nil err
		testErr = nil
		err := HandleServerStreaming[mockRes, ServerStreamingClient[mockRes]](context.Background(), st, func(ctx context.Context, stream ServerStreamingClient[mockRes]) error {
			return testErr
		})
		test.Assert(t, err == testErr, err)
		// non-nil err
		testErr = errors.New("test")
		err = HandleServerStreaming[mockRes, ServerStreamingClient[mockRes]](context.Background(), st, func(ctx context.Context, stream ServerStreamingClient[mockRes]) error {
			return testErr
		})
		test.Assert(t, err == testErr, err)
		// panic
		testErr = nil
		err = HandleServerStreaming[mockRes, ServerStreamingClient[mockRes]](context.Background(), st, func(ctx context.Context, stream ServerStreamingClient[mockRes]) error {
			panic("panic during callback")
		})
		test.Assert(t, err == testErr, err)
	})
	// client-streaming
	t.Run("client-streaming", func(t *testing.T) {
		var testErr error
		st := &mockClientStreamingClient[mockReq, mockRes]{
			mockClientStream: &mockClientStream{
				doFinish: func(err error) {
					test.Assert(t, err == testErr, err)
				},
				cancel: func(err error) {
					test.Assert(t, err == testErr, err)
				},
			},
		}
		// nil err
		testErr = nil
		err := HandleClientStreaming[mockReq, mockRes, ClientStreamingClient[mockReq, mockRes]](context.Background(), st, func(ctx context.Context, stream ClientStreamingClient[mockReq, mockRes]) error {
			return testErr
		})
		test.Assert(t, err == testErr, err)
		// non-nil err
		testErr = errors.New("test")
		err = HandleClientStreaming[mockReq, mockRes, ClientStreamingClient[mockReq, mockRes]](context.Background(), st, func(ctx context.Context, stream ClientStreamingClient[mockReq, mockRes]) error {
			return testErr
		})
		test.Assert(t, err == testErr, err)
		// panic
		testErr = nil
		err = HandleClientStreaming[mockReq, mockRes, ClientStreamingClient[mockReq, mockRes]](context.Background(), st, func(ctx context.Context, stream ClientStreamingClient[mockReq, mockRes]) error {
			panic("panic during callback")
		})
		test.Assert(t, err == testErr, err)
	})
	// bidi-streaming
	t.Run("bidi-streaming", func(t *testing.T) {
		var testErr error
		st := &mockBidiStreamingClient[mockReq, mockRes]{
			mockClientStream: &mockClientStream{
				doFinish: func(err error) {
					test.Assert(t, err == testErr, err)
				},
				cancel: func(err error) {
					test.Assert(t, err == testErr, err)
				},
			},
		}
		// nil err
		testErr = nil
		err := HandleBidiStreaming[mockReq, mockRes, BidiStreamingClient[mockReq, mockRes]](context.Background(), st, func(ctx context.Context, stream BidiStreamingClient[mockReq, mockRes]) error {
			return testErr
		})
		test.Assert(t, err == testErr, err)
		// non-nil err
		testErr = errors.New("test")
		err = HandleBidiStreaming[mockReq, mockRes, BidiStreamingClient[mockReq, mockRes]](context.Background(), st, func(ctx context.Context, stream BidiStreamingClient[mockReq, mockRes]) error {
			return testErr
		})
		test.Assert(t, err == testErr, err)
		// panic
		testErr = nil
		err = HandleBidiStreaming[mockReq, mockRes, BidiStreamingClient[mockReq, mockRes]](context.Background(), st, func(ctx context.Context, stream BidiStreamingClient[mockReq, mockRes]) error {
			panic("panic during callback")
		})
		test.Assert(t, err == testErr, err)
	})
}
