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
	tc := &TraceController{}

	var startCalled, recvCalled, sendCalled, recvHeaderCalled, finishCalled bool
	handler := StreamEventHandler{
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
	tc.AppendStreamEventHandler(handler)
	test.Assert(t, len(tc.streamStartEventHandlers) == 1, "should have 1 start handler")
	test.Assert(t, len(tc.streamRecvEventHandlers) == 1, "should have 1 recv handler")
	test.Assert(t, len(tc.streamSendEventHandlers) == 1, "should have 1 send handler")
	test.Assert(t, len(tc.streamRecvHeaderEventHandlers) == 1, "should have 1 recv header handler")
	test.Assert(t, len(tc.streamFinishEventHandlers) == 1, "should have 1 finish handler")

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
}

func TestAppendStreamEventHandlerPartial(t *testing.T) {
	tc := &TraceController{}

	handler := StreamEventHandler{
		HandleStreamStartEvent: func(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {},
	}

	tc.AppendStreamEventHandler(handler)

	test.Assert(t, len(tc.streamStartEventHandlers) == 1, "should have 1 start handler")
	test.Assert(t, len(tc.streamRecvEventHandlers) == 0, "should have 0 recv handlers")
	test.Assert(t, len(tc.streamSendEventHandlers) == 0, "should have 0 send handlers")
	test.Assert(t, len(tc.streamRecvHeaderEventHandlers) == 0, "should have 0 recv header handlers")
	test.Assert(t, len(tc.streamFinishEventHandlers) == 0, "should have 0 finish handlers")
}

func TestTraceController_HandleStreamStartEvent(t *testing.T) {
	t.Run("no-handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		test.Assert(t, len(tc.streamStartEventHandlers) == 0)
		tc.HandleStreamStartEvent(ctx, ri, StreamStartEvent{})
	})
	t.Run("handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		called := false
		handler := StreamEventHandler{
			HandleStreamStartEvent: func(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {
				called = true
			},
		}

		tc.AppendStreamEventHandler(handler)
		tc.HandleStreamStartEvent(ctx, ri, StreamStartEvent{})
		test.Assert(t, called)
	})
	t.Run("handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		called := false
		handler := StreamEventHandler{
			HandleStreamStartEvent: func(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {
				called = true
				panic("start event panic")
			},
		}

		tc.AppendStreamEventHandler(handler)
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
	t.Run("handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		recvErr := errors.New("XXX")
		called := false
		handler := StreamEventHandler{
			HandleStreamRecvEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
				called = true
				test.Assert(t, evt.Err == recvErr)
			},
		}

		tc.AppendStreamEventHandler(handler)
		tc.HandleStreamRecvEvent(ctx, ri, StreamRecvEvent{Err: recvErr})
		test.Assert(t, called)
	})
	t.Run("handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		recvErr := errors.New("XXX")
		called := false
		handler := StreamEventHandler{
			HandleStreamRecvEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
				called = true
				panic(recvErr)
			},
		}

		tc.AppendStreamEventHandler(handler)
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
	t.Run("handler", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		sendErr := errors.New("XXX")
		called := false
		handler := StreamEventHandler{
			HandleStreamSendEvent: func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
				called = true
				test.Assert(t, evt.Err == sendErr)
			},
		}

		tc.AppendStreamEventHandler(handler)
		tc.HandleStreamSendEvent(ctx, ri, StreamSendEvent{Err: sendErr})
		test.Assert(t, called)
	})
	t.Run("handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		sendErr := errors.New("XXX")
		called := false
		handler := StreamEventHandler{
			HandleStreamSendEvent: func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
				called = true
				panic(sendErr)
			},
		}

		tc.AppendStreamEventHandler(handler)
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
	t.Run("handler-grpc", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		grpcHeader := map[string][]string{"key": {"value"}}
		called := false
		handler := StreamEventHandler{
			HandleStreamRecvHeaderEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvHeaderEvent) {
				called = true
				test.Assert(t, len(evt.GRPCHeader) == 1)
				test.Assert(t, evt.GRPCHeader["key"][0] == "value")
				test.Assert(t, len(evt.TTStreamHeader) == 0)
			},
		}

		tc.AppendStreamEventHandler(handler)
		tc.HandleStreamRecvHeaderEvent(ctx, ri, StreamRecvHeaderEvent{GRPCHeader: grpcHeader})
		test.Assert(t, called)
	})
	t.Run("handler-ttstream", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		ttStreamHeader := map[string]string{"key": "value"}
		called := false
		handler := StreamEventHandler{
			HandleStreamRecvHeaderEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvHeaderEvent) {
				called = true
				test.Assert(t, len(evt.TTStreamHeader) == 1)
				test.Assert(t, evt.TTStreamHeader["key"] == "value")
				test.Assert(t, len(evt.GRPCHeader) == 0)
			},
		}

		tc.AppendStreamEventHandler(handler)
		tc.HandleStreamRecvHeaderEvent(ctx, ri, StreamRecvHeaderEvent{TTStreamHeader: ttStreamHeader})
		test.Assert(t, called)
	})
	t.Run("handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		header := map[string][]string{"key": {"value"}}
		called := false
		handler := StreamEventHandler{
			HandleStreamRecvHeaderEvent: func(ctx context.Context, ri RPCInfo, evt StreamRecvHeaderEvent) {
				called = true
				panic("recv header panic")
			},
		}

		tc.AppendStreamEventHandler(handler)
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
	t.Run("handler-grpc", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		grpcTrailer := map[string][]string{"trailer-key": {"trailer-value"}}
		called := false
		handler := StreamEventHandler{
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				called = true
				test.Assert(t, len(evt.GRPCTrailer) == 1)
				test.Assert(t, evt.GRPCTrailer["trailer-key"][0] == "trailer-value")
				test.Assert(t, len(evt.TTStreamTrailer) == 0)
			},
		}

		tc.AppendStreamEventHandler(handler)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{GRPCTrailer: grpcTrailer})
		test.Assert(t, called)
	})
	t.Run("handler-ttstream", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		ttStreamTrailer := map[string]string{"trailer-key": "trailer-value"}
		called := false
		handler := StreamEventHandler{
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				called = true
				test.Assert(t, len(evt.TTStreamTrailer) == 1)
				test.Assert(t, evt.TTStreamTrailer["trailer-key"] == "trailer-value")
				test.Assert(t, len(evt.GRPCTrailer) == 0)
			},
		}

		tc.AppendStreamEventHandler(handler)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{TTStreamTrailer: ttStreamTrailer})
		test.Assert(t, called)
	})
	t.Run("handler panic recovered", func(t *testing.T) {
		tc := &TraceController{}
		ctx := context.Background()
		ri := NewRPCInfo(nil, nil, nil, nil, nil)

		trailer := map[string][]string{"trailer-key": {"trailer-value"}}
		called := false
		handler := StreamEventHandler{
			HandleStreamFinishEvent: func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
				called = true
				panic("finish event panic")
			},
		}

		tc.AppendStreamEventHandler(handler)
		tc.HandleStreamFinishEvent(ctx, ri, StreamFinishEvent{GRPCTrailer: trailer})
		test.Assert(t, called)
	})
}
