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

package client

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	streaming_types "github.com/cloudwego/kitex/pkg/streaming/types"
	"github.com/cloudwego/kitex/transport"
)

// Mock implementations for testing
type mockStreamRPCConfig struct {
	streamTimeout        time.Duration
	streamRecvTimeout    time.Duration
	streamSendTimeout    time.Duration
	callbackConfig       *streaming.FinishCallback
	independentLifecycle bool
}

func (m *mockStreamRPCConfig) RPCTimeout() time.Duration                { return 0 }
func (m *mockStreamRPCConfig) ConnectTimeout() time.Duration            { return 0 }
func (m *mockStreamRPCConfig) ReadWriteTimeout() time.Duration          { return 0 }
func (m *mockStreamRPCConfig) IOBufferSize() int                        { return 0 }
func (m *mockStreamRPCConfig) TransportProtocol() transport.Protocol    { return transport.TTHeader }
func (m *mockStreamRPCConfig) InteractionMode() rpcinfo.InteractionMode { return 0 }
func (m *mockStreamRPCConfig) PayloadCodec() serviceinfo.PayloadCodec {
	return serviceinfo.Thrift
}
func (m *mockStreamRPCConfig) StreamTimeout() time.Duration     { return m.streamTimeout }
func (m *mockStreamRPCConfig) StreamRecvTimeout() time.Duration { return m.streamRecvTimeout }
func (m *mockStreamRPCConfig) StreamSendTimeout() time.Duration { return m.streamSendTimeout }
func (m *mockStreamRPCConfig) StreamCallbackConfig() *streaming.FinishCallback {
	return m.callbackConfig
}

func (m *mockStreamRPCConfig) StreamIndependentLifecycle() bool {
	return m.independentLifecycle
}

type mockStreamEndpointInfo struct {
	serviceName string
}

func (m *mockStreamEndpointInfo) ServiceName() string                  { return m.serviceName }
func (m *mockStreamEndpointInfo) Method() string                       { return "" }
func (m *mockStreamEndpointInfo) Address() net.Addr                    { return nil }
func (m *mockStreamEndpointInfo) DefaultTag(key, def string) string    { return "" }
func (m *mockStreamEndpointInfo) DefaultServiceName(def string) string { return m.serviceName }
func (m *mockStreamEndpointInfo) Tag(key string) (string, bool)        { return "", false }

type mockStreamInvocation struct {
	rpcinfo.Invocation
	bizStatusErr kerrors.BizStatusErrorIface
}

// Implement InvocationSetter interface
func (m *mockStreamInvocation) SetPackageName(name string)                      {}
func (m *mockStreamInvocation) SetServiceName(name string)                      {}
func (m *mockStreamInvocation) SetMethodName(name string)                       {}
func (m *mockStreamInvocation) SetMethodInfo(methodInfo serviceinfo.MethodInfo) {}
func (m *mockStreamInvocation) SetStreamingMode(mode serviceinfo.StreamingMode) {}
func (m *mockStreamInvocation) SetSeqID(seqID int32)                            {}
func (m *mockStreamInvocation) SetExtra(key string, value interface{})          {}
func (m *mockStreamInvocation) Reset()                                          {}

func (m *mockStreamInvocation) SetBizStatusErr(err kerrors.BizStatusErrorIface) {
	m.bizStatusErr = err
}

func (m *mockStreamInvocation) BizStatusErr() kerrors.BizStatusErrorIface {
	return m.bizStatusErr
}

type mockStreamRPCInfo struct {
	config     rpcinfo.RPCConfig
	invocation rpcinfo.Invocation
	from       rpcinfo.EndpointInfo
}

func (m *mockStreamRPCInfo) From() rpcinfo.EndpointInfo     { return m.from }
func (m *mockStreamRPCInfo) To() rpcinfo.EndpointInfo       { return nil }
func (m *mockStreamRPCInfo) Invocation() rpcinfo.Invocation { return m.invocation }
func (m *mockStreamRPCInfo) Config() rpcinfo.RPCConfig      { return m.config }
func (m *mockStreamRPCInfo) Stats() rpcinfo.RPCStats        { return nil }
func (m *mockStreamRPCInfo) GetTagValue(key string) (string, bool) {
	return "", false
}

// Test handleGRPC
func Test_handleGRPC(t *testing.T) {
	ctx := context.Background()

	t.Run("nil error should return early", func(t *testing.T) {
		ri := &mockStreamRPCInfo{}
		callback := &streaming.FinishCallback{}
		handleGRPC(ctx, ri, nil, callback)
		// Should not panic and return early
	})

	t.Run("non-status error - BizStatusError", func(t *testing.T) {
		bizErr := kerrors.NewBizStatusError(100, "business error")
		ri := &mockStreamRPCInfo{}

		called := false
		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				called = true
				test.Assert(t, err.BizStatusCode() == 100)
				test.Assert(t, err.BizMessage() == "business error")
			},
		}

		handleGRPC(ctx, ri, bizErr, callback)
		test.Assert(t, called, "OnBusinessError should be called")
	})

	t.Run("non-status error - regular error", func(t *testing.T) {
		regularErr := errors.New("some error")
		invocation := &mockStreamInvocation{}
		ri := &mockStreamRPCInfo{invocation: invocation}

		called := false
		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				called = true
				test.Assert(t, err.BizStatusCode() == int32(codes.Internal))
			},
		}

		handleGRPC(ctx, ri, regularErr, callback)
		test.Assert(t, called, "OnBusinessError should be called")
	})

	t.Run("status error - from business", func(t *testing.T) {
		st := status.New(codes.InvalidArgument, "invalid argument [biz error]")
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		called := false
		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				called = true
				test.Assert(t, err.BizStatusCode() == int32(codes.InvalidArgument))
			},
		}

		handleGRPC(ctx, ri, st.Err(), callback)
		test.Assert(t, called, "OnBusinessError should be called")
	})

	t.Run("status error - local cascaded cancel", func(t *testing.T) {
		st := status.NewCancelStatus(codes.Canceled, "canceled by handler", streaming_types.LocalCascadedCancel)
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		called := false
		callback := &streaming.FinishCallback{
			OnCancelError: func(ctx context.Context, err streaming_types.CancelError) {
				called = true
				test.Assert(t, err.Type() == streaming_types.LocalCascadedCancel)
				test.Assert(t, err.StatusCode() == int32(codes.Canceled))

				// Check if it's cascaded cancel error
				_, ok := err.(localCascadedCancelError)
				test.Assert(t, ok, err)
			},
		}

		handleGRPC(ctx, ri, st.Err(), callback)
		test.Assert(t, called, "OnCancelError should be called")
	})

	t.Run("status error - remote cascaded cancel", func(t *testing.T) {
		st := status.NewCancelStatus(codes.Canceled, "canceled by remote", streaming_types.RemoteCascadedCancel)
		ri := &mockStreamRPCInfo{
			invocation: &mockStreamInvocation{},
			from:       &mockStreamEndpointInfo{serviceName: "remote-service"},
		}

		called := false
		callback := &streaming.FinishCallback{
			OnCancelError: func(ctx context.Context, err streaming_types.CancelError) {
				called = true
				test.Assert(t, err.Type() == streaming_types.RemoteCascadedCancel)

				if cErr, ok := err.(remoteCascadedCancelError); ok {
					cancelPath := cErr.CancelPath()
					test.Assert(t, len(cancelPath) == 1)
					test.Assert(t, cancelPath[0].Name == "remote-service")
				}
			},
		}

		handleGRPC(ctx, ri, st.Err(), callback)
		test.Assert(t, called, "OnCancelError should be called")
	})

	t.Run("status error - active cancel", func(t *testing.T) {
		st := status.NewCancelStatus(codes.Canceled, "actively canceled", streaming_types.ActiveCancel)
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		called := false
		callback := &streaming.FinishCallback{
			OnCancelError: func(ctx context.Context, err streaming_types.CancelError) {
				called = true
				test.Assert(t, err.Type() == streaming_types.ActiveCancel)

				_, ok := err.(activeCancelError)
				test.Assert(t, ok, "should be activeCancelError")
			},
		}

		handleGRPC(ctx, ri, st.Err(), callback)
		test.Assert(t, called, "OnCancelError should be called")
	})

	t.Run("status error - stream timeout", func(t *testing.T) {
		st := status.NewTimeoutStatus(codes.DeadlineExceeded, "stream timeout", streaming_types.StreamTimeout)
		cfg := &mockStreamRPCConfig{streamTimeout: 5 * time.Second}
		ri := &mockStreamRPCInfo{
			invocation: &mockStreamInvocation{},
			config:     cfg,
		}

		called := false
		callback := &streaming.FinishCallback{
			OnTimeoutError: func(ctx context.Context, err streaming_types.TimeoutError) {
				called = true
				test.Assert(t, err.StatusCode() == int32(codes.DeadlineExceeded))
				test.Assert(t, err.TimeoutType() == streaming_types.StreamTimeout)
				test.Assert(t, err.Timeout() == 5*time.Second)
			},
		}

		handleGRPC(ctx, ri, st.Err(), callback)
		test.Assert(t, called, "OnTimeoutError should be called")
	})

	t.Run("status error - recv timeout", func(t *testing.T) {
		st := status.NewTimeoutStatus(codes.DeadlineExceeded, "recv timeout", streaming_types.StreamRecvTimeout)
		cfg := &mockStreamRPCConfig{streamRecvTimeout: 3 * time.Second}
		ri := &mockStreamRPCInfo{
			invocation: &mockStreamInvocation{},
			config:     cfg,
		}

		called := false
		callback := &streaming.FinishCallback{
			OnTimeoutError: func(ctx context.Context, err streaming_types.TimeoutError) {
				called = true
				test.Assert(t, err.TimeoutType() == streaming_types.StreamRecvTimeout)
				test.Assert(t, err.Timeout() == 3*time.Second)
			},
		}

		handleGRPC(ctx, ri, st.Err(), callback)
		test.Assert(t, called, "OnTimeoutError should be called")
	})

	t.Run("status error - send timeout", func(t *testing.T) {
		st := status.NewTimeoutStatus(codes.DeadlineExceeded, "send timeout", streaming_types.StreamSendTimeout)
		cfg := &mockStreamRPCConfig{streamSendTimeout: 2 * time.Second}
		ri := &mockStreamRPCInfo{
			invocation: &mockStreamInvocation{},
			config:     cfg,
		}

		called := false
		callback := &streaming.FinishCallback{
			OnTimeoutError: func(ctx context.Context, err streaming_types.TimeoutError) {
				called = true
				test.Assert(t, err.TimeoutType() == streaming_types.StreamSendTimeout)
				test.Assert(t, err.Timeout() == 2*time.Second)
			},
		}

		handleGRPC(ctx, ri, st.Err(), callback)
		test.Assert(t, called, "OnTimeoutError should be called")
	})

	t.Run("status error - wire error", func(t *testing.T) {
		st := status.New(codes.Unavailable, "service unavailable")
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		called := false
		callback := &streaming.FinishCallback{
			OnWireError: func(ctx context.Context, err streaming_types.WireError) {
				called = true
				test.Assert(t, err.StatusCode() == int32(codes.Unavailable))
				test.Assert(t, err.Error() != "")
			},
		}

		handleGRPC(ctx, ri, st.Err(), callback)
		test.Assert(t, called, "OnWireError should be called")
	})

	t.Run("callback is nil should not panic", func(t *testing.T) {
		st := status.NewCancelStatus(codes.Canceled, "canceled", streaming_types.ActiveCancel)
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		callback := &streaming.FinishCallback{
			// OnCancelError is nil
		}

		// Should not panic
		handleGRPC(ctx, ri, st.Err(), callback)
	})
}

func Test_handleTTStream(t *testing.T) {
	ctx := context.Background()

	t.Run("nil error should return early", func(t *testing.T) {
		ri := &mockStreamRPCInfo{}
		callback := &streaming.FinishCallback{}
		handleTTStream(ctx, ri, nil, callback)
	})

	t.Run("BizStatusError from remote", func(t *testing.T) {
		bizErr := kerrors.NewBizStatusError(200, "ttstream biz error")
		ri := &mockStreamRPCInfo{}

		called := false
		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				called = true
				test.Assert(t, err.BizStatusCode() == 200)
				test.Assert(t, err.BizMessage() == "ttstream biz error")
			},
		}

		handleTTStream(ctx, ri, bizErr, callback)
		test.Assert(t, called, "OnBusinessError should be called")
	})

	t.Run("regular error not BizStatusError", func(t *testing.T) {
		regularErr := errors.New("regular error")
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		called := false
		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				called = true
				// TTStream uses error code 12001 for non-BizStatusError errors
				test.Assert(t, err.BizStatusCode() == 12001)
				test.Assert(t, err.BizMessage() == "regular error")
			},
		}

		handleTTStream(ctx, ri, regularErr, callback)
		test.Assert(t, called, "OnBusinessError should be called")
	})

	t.Run("ttstream.Exception - ApplicationException", func(t *testing.T) {
		// ApplicationException should be handled as business error
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		called := false
		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				called = true
			},
		}

		handleTTStream(ctx, ri, ttstream.ErrApplicationException, callback)
		test.Assert(t, called, "OnBusinessError should be called for ApplicationException")
	})

	t.Run("ttstream.Exception - Protocol error", func(t *testing.T) {
		// Create a protocol error by using one of the exported exceptions
		// Since we can't directly create exceptions, we'll need to test with actual ttstream exceptions
		// For now, test the general wire error path
		ex := &ttstream.Exception{}
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		called := false
		callback := &streaming.FinishCallback{
			OnWireError: func(ctx context.Context, err streaming_types.WireError) {
				called = true
			},
		}

		handleTTStream(ctx, ri, ex, callback)
		test.Assert(t, called, "OnWireError should be called")
	})

	t.Run("callback is nil should not panic", func(t *testing.T) {
		ex := &ttstream.Exception{}
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		callback := &streaming.FinishCallback{
			// OnWireError is nil
		}

		// Should not panic
		handleTTStream(ctx, ri, ex, callback)
	})
}

// Test error type implementations
func Test_wireError(t *testing.T) {
	err := wireError{
		err:  errors.New("wire error"),
		code: int32(codes.Unavailable),
	}

	test.Assert(t, err.Error() == "wire error")
	test.Assert(t, err.StatusCode() == int32(codes.Unavailable))

	// Verify it implements the interface
	var _ streaming_types.WireError = err
}

func Test_cancelError(t *testing.T) {
	err := cancelError{
		err:  errors.New("cancel error"),
		code: int32(codes.Canceled),
		typ:  streaming_types.ActiveCancel,
	}

	test.Assert(t, err.Error() == "cancel error")
	test.Assert(t, err.StatusCode() == int32(codes.Canceled))
	test.Assert(t, err.Type() == streaming_types.ActiveCancel)

	// Verify it implements the interface
	var _ streaming_types.CancelError = err
}

func Test_cascadedCancelError(t *testing.T) {
	t.Run("canceled by handler return", func(t *testing.T) {
		err := localCascadedCancelError{
			cancelError: cancelError{
				err:  errors.New("cascaded cancel"),
				code: int32(codes.Canceled),
				typ:  streaming_types.LocalCascadedCancel,
			},
		}

		test.Assert(t, err.Error() == "cascaded cancel")
		test.Assert(t, err.Type() == streaming_types.LocalCascadedCancel)
	})

	t.Run("canceled by remote with path", func(t *testing.T) {
		err := remoteCascadedCancelError{
			cancelError: cancelError{
				err:  errors.New("cascaded cancel"),
				code: int32(codes.Canceled),
				typ:  streaming_types.RemoteCascadedCancel,
			},
			cancelPath: []string{"service1", "service2"},
		}
		path := err.CancelPath()
		test.Assert(t, len(path) == 2)
		test.Assert(t, path[0].Name == "service1")
		test.Assert(t, path[1].Name == "service2")
	})

	t.Run("empty cancel path", func(t *testing.T) {
		err := remoteCascadedCancelError{
			cancelError: cancelError{
				err:  errors.New("cascaded cancel"),
				code: int32(codes.Canceled),
				typ:  streaming_types.RemoteCascadedCancel,
			},
			cancelPath: []string{},
		}

		// Empty slice should still work
		path := err.CancelPath()
		test.Assert(t, len(path) == 0)
	})
}

func Test_activeCancelError(t *testing.T) {
	err := activeCancelError{
		cancelError: cancelError{
			err:  errors.New("active cancel"),
			code: int32(codes.Canceled),
			typ:  streaming_types.ActiveCancel,
		},
	}

	test.Assert(t, err.Error() == "active cancel")
	test.Assert(t, err.Type() == streaming_types.ActiveCancel)
	test.Assert(t, err.StatusCode() == int32(codes.Canceled))
}

func Test_timeoutError(t *testing.T) {
	err := timeoutError{
		err:         errors.New("timeout"),
		code:        int32(codes.DeadlineExceeded),
		timeoutType: streaming_types.StreamTimeout,
		timeout:     5 * time.Second,
	}

	test.Assert(t, err.Error() == "timeout")
	test.Assert(t, err.StatusCode() == int32(codes.DeadlineExceeded))
	test.Assert(t, err.TimeoutType() == streaming_types.StreamTimeout)
	test.Assert(t, err.Timeout() == 5*time.Second)

	// Verify it implements the interface
	var _ streaming_types.TimeoutError = err
}

func Test_handleGRPCCancelStatus_nilCallback(t *testing.T) {
	ctx := context.Background()
	st := status.NewCancelStatus(codes.Canceled, "test", streaming_types.ActiveCancel)
	ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

	callback := &streaming.FinishCallback{
		OnCancelError: nil,
	}

	// Should return early and not panic
	handleGRPCCancelStatus(ctx, ri, st.Err(), st, callback)
}

func Test_handleGRPCTimeoutStatus_nilCallback(t *testing.T) {
	ctx := context.Background()
	st := status.NewTimeoutStatus(codes.DeadlineExceeded, "test", streaming_types.StreamTimeout)
	ri := &mockStreamRPCInfo{
		invocation: &mockStreamInvocation{},
		config:     &mockStreamRPCConfig{streamTimeout: time.Second},
	}

	callback := &streaming.FinishCallback{
		OnTimeoutError: nil,
	}

	// Should return early and not panic
	handleGRPCTimeoutStatus(ctx, ri, st.Err(), st, callback)
}

func Test_handleBusinessError(t *testing.T) {
	ctx := context.Background()

	t.Run("BizStatusError", func(t *testing.T) {
		bizErr := kerrors.NewBizStatusError(100, "biz error")

		called := false
		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				called = true
				test.Assert(t, err.BizStatusCode() == 100)
			},
		}

		handleBusinessError(ctx, bizErr, int32(codes.Internal), callback)
		test.Assert(t, called)
	})

	t.Run("regular error converted to BizStatusError", func(t *testing.T) {
		regularErr := errors.New("test error")

		called := false
		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				called = true
				test.Assert(t, err.BizStatusCode() == int32(codes.Internal))
				test.Assert(t, err.BizMessage() == "test error")
			},
		}

		handleBusinessError(ctx, regularErr, int32(codes.Internal), callback)
		test.Assert(t, called)
	})

	t.Run("nil callback should not panic", func(t *testing.T) {
		regularErr := errors.New("test error")
		callback := &streaming.FinishCallback{
			OnBusinessError: nil,
		}

		// Should not panic
		handleBusinessError(ctx, regularErr, int32(codes.Internal), callback)
	})
}

func Test_handleGRPCStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("business error path", func(t *testing.T) {
		st := status.New(codes.FailedPrecondition, "business logic failed [biz error]")
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		called := false
		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				called = true
			},
		}

		handleGRPCStatus(ctx, ri, st.Err(), st, callback)
		test.Assert(t, called)
	})

	t.Run("default wire error path", func(t *testing.T) {
		st := status.New(codes.Internal, "internal error")
		ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

		called := false
		callback := &streaming.FinishCallback{
			OnWireError: func(ctx context.Context, err streaming_types.WireError) {
				called = true
				test.Assert(t, err.StatusCode() == int32(codes.Internal))
			},
		}

		handleGRPCStatus(ctx, ri, st.Err(), st, callback)
		test.Assert(t, called)
	})
}

// Test buildTimeoutError
func Test_buildTimeoutError(t *testing.T) {
	testErr := errors.New("timeout occurred")
	cfg := &mockStreamRPCConfig{
		streamTimeout:     5 * time.Second,
		streamRecvTimeout: 3 * time.Second,
		streamSendTimeout: 2 * time.Second,
	}
	ri := &mockStreamRPCInfo{config: cfg}

	t.Run("StreamTimeout", func(t *testing.T) {
		te := buildTimeoutError(testErr, int32(codes.DeadlineExceeded), ri, streaming_types.StreamTimeout)
		test.Assert(t, te.Error() == "timeout occurred")
		test.Assert(t, te.StatusCode() == int32(codes.DeadlineExceeded))
		test.Assert(t, te.TimeoutType() == streaming_types.StreamTimeout)
		test.Assert(t, te.Timeout() == 5*time.Second)
	})

	t.Run("StreamRecvTimeout", func(t *testing.T) {
		te := buildTimeoutError(testErr, int32(codes.DeadlineExceeded), ri, streaming_types.StreamRecvTimeout)
		test.Assert(t, te.TimeoutType() == streaming_types.StreamRecvTimeout)
		test.Assert(t, te.Timeout() == 3*time.Second)
	})

	t.Run("StreamSendTimeout", func(t *testing.T) {
		te := buildTimeoutError(testErr, int32(codes.DeadlineExceeded), ri, streaming_types.StreamSendTimeout)
		test.Assert(t, te.TimeoutType() == streaming_types.StreamSendTimeout)
		test.Assert(t, te.Timeout() == 2*time.Second)
	})

	t.Run("Unknown timeout type", func(t *testing.T) {
		// Unknown timeout type should still return valid error with zero timeout
		te := buildTimeoutError(testErr, int32(codes.DeadlineExceeded), ri, streaming_types.TimeoutType(99))
		test.Assert(t, te.Error() == "timeout occurred")
		test.Assert(t, te.Timeout() == 0)
	})
}

// Test buildCancelError
func Test_buildCancelError(t *testing.T) {
	testErr := errors.New("canceled")
	testCode := int32(codes.Canceled)

	t.Run("LocalCascadedCancel", func(t *testing.T) {
		callCount := 0
		getPath := func() []string {
			callCount++
			return []string{"should-not-be-called"}
		}

		ce := buildCancelError(testErr, testCode, streaming_types.LocalCascadedCancel, getPath)
		test.Assert(t, ce.Error() == "canceled")
		test.Assert(t, ce.StatusCode() == testCode)
		test.Assert(t, ce.Type() == streaming_types.LocalCascadedCancel)

		// Verify lazy evaluation - function should not be called for LocalCascadedCancel
		test.Assert(t, callCount == 0, "getPath should not be called for LocalCascadedCancel")

		// Verify type
		_, ok := ce.(localCascadedCancelError)
		test.Assert(t, ok, "should be localCascadedCancelError")
	})

	t.Run("RemoteCascadedCancel", func(t *testing.T) {
		callCount := 0
		expectedPath := []string{"service1", "service2"}
		getPath := func() []string {
			callCount++
			return expectedPath
		}

		ce := buildCancelError(testErr, testCode, streaming_types.RemoteCascadedCancel, getPath)
		test.Assert(t, ce.Type() == streaming_types.RemoteCascadedCancel)

		// Verify lazy evaluation - function SHOULD be called for RemoteCascadedCancel
		test.Assert(t, callCount == 1, "getPath should be called once for RemoteCascadedCancel")

		// Verify path is set correctly
		if rce, ok := ce.(remoteCascadedCancelError); ok {
			cancelPath := rce.CancelPath()
			test.Assert(t, len(cancelPath) == 2)
			test.Assert(t, cancelPath[0].Name == "service1")
			test.Assert(t, cancelPath[1].Name == "service2")
		} else {
			t.Fatal("should be remoteCascadedCancelError")
		}
	})

	t.Run("ActiveCancel", func(t *testing.T) {
		callCount := 0
		getPath := func() []string {
			callCount++
			return []string{"should-not-be-called"}
		}

		ce := buildCancelError(testErr, testCode, streaming_types.ActiveCancel, getPath)
		test.Assert(t, ce.Type() == streaming_types.ActiveCancel)

		// Verify lazy evaluation - function should not be called for ActiveCancel
		test.Assert(t, callCount == 0, "getPath should not be called for ActiveCancel")

		// Verify type
		_, ok := ce.(activeCancelError)
		test.Assert(t, ok, "should be activeCancelError")
	})

	t.Run("Unknown cancel type - fallback to base cancelError", func(t *testing.T) {
		callCount := 0
		getPath := func() []string {
			callCount++
			return []string{"should-not-be-called"}
		}

		// Use an undefined cancel type
		unknownType := streaming_types.CancelType(99)
		ce := buildCancelError(testErr, testCode, unknownType, getPath)

		// Should return base cancelError
		test.Assert(t, ce.Error() == "canceled")
		test.Assert(t, ce.StatusCode() == testCode)
		test.Assert(t, ce.Type() == unknownType)

		// Function should not be called for unknown type
		test.Assert(t, callCount == 0, "getPath should not be called for unknown cancel type")

		// Should be base cancelError type
		_, ok := ce.(cancelError)
		test.Assert(t, ok, "should fallback to base cancelError for unknown type")
	})
}

// Test GRPCBizStatusError priority
func Test_handleGRPC_GRPCBizStatusErrorPriority(t *testing.T) {
	ctx := context.Background()
	ri := &mockStreamRPCInfo{invocation: &mockStreamInvocation{}}

	t.Run("GRPCBizStatusError should be handled before status conversion", func(t *testing.T) {
		// Create a GRPCBizStatusError with a code that could be misinterpreted as cancel/timeout
		bizErr := kerrors.NewGRPCBizStatusError(int32(codes.Canceled), "business decided to cancel")

		businessErrorCalled := false
		cancelErrorCalled := false

		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				businessErrorCalled = true
				test.Assert(t, err.BizStatusCode() == int32(codes.Canceled))
				test.Assert(t, err.BizMessage() == "business decided to cancel")
			},
			OnCancelError: func(ctx context.Context, err streaming_types.CancelError) {
				cancelErrorCalled = true
			},
		}

		handleGRPC(ctx, ri, bizErr, callback)

		// Should call OnBusinessError, NOT OnCancelError
		test.Assert(t, businessErrorCalled, "OnBusinessError should be called")
		test.Assert(t, !cancelErrorCalled, "OnCancelError should NOT be called for GRPCBizStatusError")
	})

	t.Run("GRPCBizStatusError with DeadlineExceeded code", func(t *testing.T) {
		bizErr := kerrors.NewGRPCBizStatusError(int32(codes.DeadlineExceeded), "business timeout")

		businessErrorCalled := false
		timeoutErrorCalled := false

		callback := &streaming.FinishCallback{
			OnBusinessError: func(ctx context.Context, err kerrors.BizStatusErrorIface) {
				businessErrorCalled = true
			},
			OnTimeoutError: func(ctx context.Context, err streaming_types.TimeoutError) {
				timeoutErrorCalled = true
			},
		}

		handleGRPC(ctx, ri, bizErr, callback)

		// Should call OnBusinessError, NOT OnTimeoutError
		test.Assert(t, businessErrorCalled, "OnBusinessError should be called")
		test.Assert(t, !timeoutErrorCalled, "OnTimeoutError should NOT be called for GRPCBizStatusError")
	})
}

// Test remote service name fallback
func Test_handleGRPCCancelStatus_RemoteServiceFallback(t *testing.T) {
	ctx := context.Background()
	st := status.NewCancelStatus(codes.Canceled, "test", streaming_types.RemoteCascadedCancel)

	t.Run("empty service name should use default", func(t *testing.T) {
		ri := &mockStreamRPCInfo{
			invocation: &mockStreamInvocation{},
			from:       &mockStreamEndpointInfo{serviceName: ""}, // Empty service name
		}

		called := false
		callback := &streaming.FinishCallback{
			OnCancelError: func(ctx context.Context, err streaming_types.CancelError) {
				called = true
				if rce, ok := err.(remoteCascadedCancelError); ok {
					cancelPath := rce.CancelPath()
					test.Assert(t, len(cancelPath) == 1)
					test.Assert(t, cancelPath[0].Name == defaultGRPCRemoteService)
				} else {
					t.Fatal("should be remoteCascadedCancelError")
				}
			},
		}

		handleGRPCCancelStatus(ctx, ri, st.Err(), st, callback)
		test.Assert(t, called)
	})

	t.Run("valid service name should be used", func(t *testing.T) {
		ri := &mockStreamRPCInfo{
			invocation: &mockStreamInvocation{},
			from:       &mockStreamEndpointInfo{serviceName: "my-service"},
		}

		called := false
		callback := &streaming.FinishCallback{
			OnCancelError: func(ctx context.Context, err streaming_types.CancelError) {
				called = true
				if rce, ok := err.(remoteCascadedCancelError); ok {
					cancelPath := rce.CancelPath()
					test.Assert(t, len(cancelPath) == 1)
					test.Assert(t, cancelPath[0].Name == "my-service")
				}
			},
		}

		handleGRPCCancelStatus(ctx, ri, st.Err(), st, callback)
		test.Assert(t, called)
	})
}
