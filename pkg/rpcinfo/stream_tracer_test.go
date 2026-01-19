/*
 * Copyright 2026 CloudWeGo Authors
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

package rpcinfo

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestAppendStreamEventHandler(t *testing.T) {
	t.Run("client-side", func(t *testing.T) {
		tc := &TraceController{}
		var startCalled, recvCalled, sendCalled, recvHeaderCalled, finishCalled bool
		handler := ClientStreamEventHandler{
			HandleStreamStartEvent: func(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {
				startCalled = true
			},
			HandleStreamRecvEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
				recvCalled = true
			},
			HandleStreamSendEvent: func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
				sendCalled = true
			},
			HandleStreamRecvHeaderEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvHeaderEvent) {
				recvHeaderCalled = true
			},
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				finishCalled = true
			},
		}
		tc.AppendClientStreamEventHandler(handler)
		test.Assert(t, len(tc.streamStartEventHandlers) == 1)
		test.Assert(t, len(tc.streamRecvEventHandlers) == 1)
		test.Assert(t, len(tc.streamSendEventHandlers) == 1)
		test.Assert(t, len(tc.streamRecvHeaderEventHandlers) == 1)
		test.Assert(t, len(tc.streamFinishEventHandlers) == 1)

		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		tc.HandleStreamStartEvent(ctx, ri, StreamStartEvent{})
		test.Assert(t, startCalled)
		tc.HandleStreamRecvEvent(ctx, ri, StreamRecvEvent{})
		test.Assert(t, recvCalled)
		tc.HandleStreamSendEvent(ctx, ri, StreamSendEvent{})
		test.Assert(t, sendCalled)
		tc.HandleStreamRecvHeaderEvent(ctx, ri, StreamRecvHeaderEvent{})
		test.Assert(t, recvHeaderCalled)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{})
		test.Assert(t, finishCalled)
	})
	t.Run("server-side", func(t *testing.T) {
		tc := &TraceController{}
		var startCalled, recvCalled, sendCalled, finishCalled bool
		handler := ServerStreamEventHandler{
			HandleStreamStartEvent: func(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {
				startCalled = true
			},
			HandleStreamRecvEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
				recvCalled = true
			},
			HandleStreamSendEvent: func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
				sendCalled = true
			},
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				finishCalled = true
			},
		}
		tc.AppendServerStreamEventHandler(handler)
		test.Assert(t, len(tc.streamStartEventHandlers) == 1)
		test.Assert(t, len(tc.streamRecvEventHandlers) == 1)
		test.Assert(t, len(tc.streamSendEventHandlers) == 1)
		test.Assert(t, len(tc.streamRecvHeaderEventHandlers) == 0)
		test.Assert(t, len(tc.streamFinishEventHandlers) == 1)

		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		tc.HandleStreamStartEvent(ctx, ri, StreamStartEvent{})
		test.Assert(t, startCalled)
		tc.HandleStreamRecvEvent(ctx, ri, StreamRecvEvent{})
		test.Assert(t, recvCalled)
		tc.HandleStreamSendEvent(ctx, ri, StreamSendEvent{})
		test.Assert(t, sendCalled)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{})
		test.Assert(t, finishCalled)
	})
}

func TestTraceController_HandleStreamStartEvent(t *testing.T) {
	t.Run("no-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		test.Assert(t, len(tc.streamStartEventHandlers) == 0)
		tc.HandleStreamStartEvent(ctx, ri, StreamStartEvent{})
	})
	t.Run("client-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		called := false
		handler := ClientStreamEventHandler{
			HandleStreamStartEvent: func(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {
				called = true
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamStartEvent(ctx, ri, StreamStartEvent{})
		test.Assert(t, called)
	})
	t.Run("server-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		called := false
		handler := ServerStreamEventHandler{
			HandleStreamStartEvent: func(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {
				called = true
			},
		}

		tc.AppendServerStreamEventHandler(handler)
		tc.HandleStreamStartEvent(ctx, ri, StreamStartEvent{})
		test.Assert(t, called)
	})
	t.Run("client-handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		called := false
		handler := ClientStreamEventHandler{
			HandleStreamStartEvent: func(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {
				called = true
				panic("start event panic")
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamStartEvent(ctx, ri, StreamStartEvent{})
		test.Assert(t, called)
	})
	t.Run("server-handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		called := false
		handler := ServerStreamEventHandler{
			HandleStreamStartEvent: func(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {
				called = true
				panic("start event panic")
			},
		}

		tc.AppendServerStreamEventHandler(handler)
		tc.HandleStreamStartEvent(ctx, ri, StreamStartEvent{})
		test.Assert(t, called)
	})
}

func TestTraceController_HandleStreamRecvEvent(t *testing.T) {
	t.Run("no-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		test.Assert(t, len(tc.streamRecvEventHandlers) == 0)
		tc.HandleStreamRecvEvent(ctx, ri, StreamRecvEvent{Err: errors.New("recv error")})
	})
	t.Run("client-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		recvErr := errors.New("XXX")
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamRecvEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
				called = true
				test.Assert(t, evt.Err == recvErr)
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamRecvEvent(ctx, ri, StreamRecvEvent{Err: recvErr})
		test.Assert(t, called)
	})
	t.Run("server-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		recvErr := errors.New("XXX")
		called := false
		handler := ServerStreamEventHandler{
			HandleStreamRecvEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
				called = true
				test.Assert(t, evt.Err == recvErr)
			},
		}

		tc.AppendServerStreamEventHandler(handler)
		tc.HandleStreamRecvEvent(ctx, ri, StreamRecvEvent{Err: recvErr})
		test.Assert(t, called)
	})
	t.Run("client-handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		recvErr := errors.New("XXX")
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamRecvEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
				called = true
				panic(recvErr)
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamRecvEvent(ctx, ri, StreamRecvEvent{Err: recvErr})
		test.Assert(t, called)
	})
	t.Run("server-handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		recvErr := errors.New("XXX")
		called := false
		handler := ServerStreamEventHandler{
			HandleStreamRecvEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
				called = true
				panic(recvErr)
			},
		}

		tc.AppendServerStreamEventHandler(handler)
		tc.HandleStreamRecvEvent(ctx, ri, StreamRecvEvent{Err: recvErr})
		test.Assert(t, called)
	})
}

func TestTraceController_HandleStreamSendEvent(t *testing.T) {
	t.Run("no-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		test.Assert(t, len(tc.streamSendEventHandlers) == 0)
		tc.HandleStreamSendEvent(ctx, ri, StreamSendEvent{Err: errors.New("send error")})
	})
	t.Run("client-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		sendErr := errors.New("XXX")
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamSendEvent: func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
				called = true
				test.Assert(t, evt.Err == sendErr)
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamSendEvent(ctx, ri, StreamSendEvent{Err: sendErr})
		test.Assert(t, called)
	})
	t.Run("server-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		sendErr := errors.New("XXX")
		called := false
		handler := ServerStreamEventHandler{
			HandleStreamSendEvent: func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
				called = true
				test.Assert(t, evt.Err == sendErr)
			},
		}

		tc.AppendServerStreamEventHandler(handler)
		tc.HandleStreamSendEvent(ctx, ri, StreamSendEvent{Err: sendErr})
		test.Assert(t, called)
	})
	t.Run("client-handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		sendErr := errors.New("XXX")
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamSendEvent: func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
				called = true
				panic(sendErr)
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamSendEvent(ctx, ri, StreamSendEvent{Err: sendErr})
		test.Assert(t, called)
	})
	t.Run("server-handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		sendErr := errors.New("XXX")
		called := false
		handler := ServerStreamEventHandler{
			HandleStreamSendEvent: func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
				called = true
				panic(sendErr)
			},
		}

		tc.AppendServerStreamEventHandler(handler)
		tc.HandleStreamSendEvent(ctx, ri, StreamSendEvent{Err: sendErr})
		test.Assert(t, called)
	})
}

func TestTraceController_HandleStreamRecvHeaderEvent(t *testing.T) {
	t.Run("no-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		test.Assert(t, len(tc.streamRecvHeaderEventHandlers) == 0)
		header := map[string][]string{"key": {"value"}}
		tc.HandleStreamRecvHeaderEvent(ctx, ri, StreamRecvHeaderEvent{GRPCHeader: header})
	})
	t.Run("client-handler-grpc", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		grpcHeader := map[string][]string{"key": {"value"}}
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamRecvHeaderEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvHeaderEvent) {
				called = true
				test.Assert(t, len(evt.GRPCHeader) == 1)
				test.Assert(t, evt.GRPCHeader["key"][0] == "value")
				test.Assert(t, len(evt.TTStreamHeader) == 0)
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamRecvHeaderEvent(ctx, ri, StreamRecvHeaderEvent{GRPCHeader: grpcHeader})
		test.Assert(t, called)
	})
	t.Run("client-handler-ttstream", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		ttStreamHeader := map[string]string{"key": "value"}
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamRecvHeaderEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvHeaderEvent) {
				called = true
				test.Assert(t, len(evt.TTStreamHeader) == 1)
				test.Assert(t, evt.TTStreamHeader["key"] == "value")
				test.Assert(t, len(evt.GRPCHeader) == 0)
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamRecvHeaderEvent(ctx, ri, StreamRecvHeaderEvent{TTStreamHeader: ttStreamHeader})
		test.Assert(t, called)
	})
	t.Run("client-handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		header := map[string][]string{"key": {"value"}}
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamRecvHeaderEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvHeaderEvent) {
				called = true
				panic("recv header panic")
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamRecvHeaderEvent(ctx, ri, StreamRecvHeaderEvent{GRPCHeader: header})
		test.Assert(t, called)
	})
}

func TestTraceController_HandleStreamFinishEvent(t *testing.T) {
	t.Run("no-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		test.Assert(t, len(tc.streamFinishEventHandlers) == 0)
		trailer := map[string][]string{"trailer-key": {"trailer-value"}}
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{GRPCTrailer: trailer})
	})
	t.Run("client-handler-grpc", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		grpcTrailer := map[string][]string{"trailer-key": {"trailer-value"}}
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				called = true
				test.Assert(t, len(evt.GRPCTrailer) == 1)
				test.Assert(t, evt.GRPCTrailer["trailer-key"][0] == "trailer-value")
				test.Assert(t, len(evt.TTStreamTrailer) == 0)
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{GRPCTrailer: grpcTrailer})
		test.Assert(t, called)
	})
	t.Run("server-handler-grpc", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		grpcTrailer := map[string][]string{"trailer-key": {"trailer-value"}}
		called := false
		handler := ServerStreamEventHandler{
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				called = true
				test.Assert(t, len(evt.GRPCTrailer) == 1)
				test.Assert(t, evt.GRPCTrailer["trailer-key"][0] == "trailer-value")
				test.Assert(t, len(evt.TTStreamTrailer) == 0)
			},
		}

		tc.AppendServerStreamEventHandler(handler)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{GRPCTrailer: grpcTrailer})
		test.Assert(t, called)
	})
	t.Run("client-handler-ttstream", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		ttStreamTrailer := map[string]string{"trailer-key": "trailer-value"}
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				called = true
				test.Assert(t, len(evt.TTStreamTrailer) == 1)
				test.Assert(t, evt.TTStreamTrailer["trailer-key"] == "trailer-value")
				test.Assert(t, len(evt.GRPCTrailer) == 0)
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{TTStreamTrailer: ttStreamTrailer})
		test.Assert(t, called)
	})
	t.Run("server-handler-ttstream", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		ttStreamTrailer := map[string]string{"trailer-key": "trailer-value"}
		called := false
		handler := ServerStreamEventHandler{
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				called = true
				test.Assert(t, len(evt.TTStreamTrailer) == 1)
				test.Assert(t, evt.TTStreamTrailer["trailer-key"] == "trailer-value")
				test.Assert(t, len(evt.GRPCTrailer) == 0)
			},
		}

		tc.AppendServerStreamEventHandler(handler)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{TTStreamTrailer: ttStreamTrailer})
		test.Assert(t, called)
	})
	t.Run("client-handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		trailer := map[string][]string{"trailer-key": {"trailer-value"}}
		called := false
		handler := ClientStreamEventHandler{
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				called = true
				panic("finish event panic")
			},
		}

		tc.AppendClientStreamEventHandler(handler)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{GRPCTrailer: trailer})
		test.Assert(t, called)
	})
	t.Run("server-handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		trailer := map[string][]string{"trailer-key": {"trailer-value"}}
		called := false
		handler := ServerStreamEventHandler{
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				called = true
				panic("finish event panic")
			},
		}

		tc.AppendServerStreamEventHandler(handler)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{GRPCTrailer: trailer})
		test.Assert(t, called)
	})
}
