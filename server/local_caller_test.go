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

// Package server_test uses external test package to avoid import cycle:
// testservice/server.go imports package server, so importing testservice
// from server package would create a cycle. External test package breaks this.
package server_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	thrift "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/mocks/thrift/testservice"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/acl"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/genericserver"
)

// --- base handler that satisfies thrift.TestService ---

type baseHandler struct{}

func (h *baseHandler) Echo(stream thrift.TestService_EchoServer) error {
	return nil
}

func (h *baseHandler) EchoClient(stream thrift.TestService_EchoClientServer) error {
	return nil
}

func (h *baseHandler) EchoServer(req *thrift.Request, stream thrift.TestService_EchoServerServer) error {
	return nil
}

func (h *baseHandler) EchoUnary(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	return &thrift.Response{}, nil
}

func (h *baseHandler) EchoBizException(stream thrift.TestService_EchoBizExceptionServer) error {
	return nil
}

func (h *baseHandler) EchoPingPong(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	return &thrift.Response{Message: "echo:" + req.Message}, nil
}

type streamingHandler struct{ baseHandler }

func (h *streamingHandler) Echo(stream thrift.TestService_EchoServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.Send(&thrift.Response{Message: "bidi:" + req.Message}); err != nil {
			return err
		}
	}
}

func (h *streamingHandler) EchoClient(stream thrift.TestService_EchoClientServer) error {
	var parts []string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&thrift.Response{Message: "client:" + strings.Join(parts, ",")})
		}
		if err != nil {
			return err
		}
		parts = append(parts, req.Message)
	}
}

func (h *streamingHandler) EchoServer(req *thrift.Request, stream thrift.TestService_EchoServerServer) error {
	if err := stream.Send(&thrift.Response{Message: "server:" + req.Message + ":1"}); err != nil {
		return err
	}
	return stream.Send(&thrift.Response{Message: "server:" + req.Message + ":2"})
}

func (h *streamingHandler) EchoUnary(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	if streaming.GetStream(ctx) == nil {
		return nil, fmt.Errorf("stream context not set")
	}
	return &thrift.Response{Message: "stream-unary:" + req.Message}, nil
}

func (h *streamingHandler) EchoBizException(stream thrift.TestService_EchoBizExceptionServer) error {
	if _, err := stream.Recv(); err != nil && err != io.EOF {
		return err
	}
	return kerrors.NewBizStatusError(100, "stream biz error")
}

// --- variant handlers embedding baseHandler ---

type panicHandler struct{ baseHandler }

func (h *panicHandler) EchoPingPong(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	panic("boom")
}

type rpcInfoHandler struct {
	baseHandler
	t *testing.T
}

func (h *rpcInfoHandler) EchoPingPong(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil {
		return nil, fmt.Errorf("no rpcinfo in context")
	}
	if from := ri.From().ServiceName(); from != "test-caller" {
		return nil, fmt.Errorf("from service: got %q, want %q", from, "test-caller")
	}
	if ri.Invocation().ServiceName() != "TestService" {
		return nil, fmt.Errorf("service name: got %q, want %q", ri.Invocation().ServiceName(), "TestService")
	}
	if ri.Invocation().MethodName() != "EchoPingPong" {
		return nil, fmt.Errorf("method name: got %q, want %q", ri.Invocation().MethodName(), "EchoPingPong")
	}
	return &thrift.Response{}, nil
}

type errorHandler struct{ baseHandler }

func (h *errorHandler) EchoPingPong(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	return nil, errors.New("handler error")
}

type bizErrHandler struct{ baseHandler }

func (h *bizErrHandler) EchoPingPong(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	return nil, kerrors.NewBizStatusError(100, "biz error")
}

type ctxHandler struct {
	baseHandler
	key any
}

func (h *ctxHandler) EchoPingPong(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	v, ok := ctx.Value(h.key).(string)
	if !ok || v != "propagated" {
		return nil, fmt.Errorf("context value not propagated")
	}
	return &thrift.Response{}, nil
}

type deadlineHandler struct{ baseHandler }

func (h *deadlineHandler) EchoPingPong(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, fmt.Errorf("deadline not propagated")
	}
	return &thrift.Response{}, nil
}

type genericHandler struct{}

func (h *genericHandler) GenericCall(ctx context.Context, method string, request any) (any, error) {
	return nil, nil
}

type traceRecorder struct {
	finished bool
	lastErr  error
}

func (tr *traceRecorder) Start(ctx context.Context) context.Context {
	return ctx
}

func (tr *traceRecorder) Finish(ctx context.Context) {
	tr.finished = true
	if ri := rpcinfo.GetRPCInfo(ctx); ri != nil {
		tr.lastErr = ri.Stats().Error()
	}
}

type streamTraceRecorder struct {
	mu       sync.Mutex
	started  int
	finished int
	recv     int
	send     int
}

func (tr *streamTraceRecorder) Start(ctx context.Context) context.Context {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.started++
	return ctx
}

func (tr *streamTraceRecorder) Finish(ctx context.Context) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.finished++
}

func (tr *streamTraceRecorder) onRecv(ctx context.Context, ri rpcinfo.RPCInfo, evt rpcinfo.StreamRecvEvent) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.recv++
}

func (tr *streamTraceRecorder) onSend(ctx context.Context, ri rpcinfo.RPCInfo, evt rpcinfo.StreamSendEvent) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.send++
}

func (tr *streamTraceRecorder) snapshot() (started, finished, recv, send int) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.started, tr.finished, tr.recv, tr.send
}

func assertPanicErr(t *testing.T, err error) {
	t.Helper()
	test.Assert(t, err != nil)
	test.Assert(t, errors.Is(err, kerrors.ErrPanic), err)
}

// --- second service for multi-service tests ---

type testService2 interface {
	EchoPingPong(ctx context.Context, req *thrift.Request) (*thrift.Response, error)
	Ping(ctx context.Context, req *thrift.Request) (*thrift.Response, error)
}

type testService2Handler struct{}

func (h *testService2Handler) EchoPingPong(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	return &thrift.Response{Message: "svc2:" + req.Message}, nil
}

func (h *testService2Handler) Ping(ctx context.Context, req *thrift.Request) (*thrift.Response, error) {
	return &thrift.Response{Message: "pong"}, nil
}

func newTestService2Info() *serviceinfo.ServiceInfo {
	newArgs := func() any { return thrift.NewTestServiceEchoPingPongArgs() }
	newResult := func() any { return thrift.NewTestServiceEchoPingPongResult() }
	return &serviceinfo.ServiceInfo{
		ServiceName:  "TestService2",
		HandlerType:  (*testService2)(nil),
		PayloadCodec: serviceinfo.Thrift,
		Methods: map[string]serviceinfo.MethodInfo{
			"EchoPingPong": serviceinfo.NewMethodInfo(
				func(ctx context.Context, handler, arg, result any) error {
					resp, err := handler.(testService2).EchoPingPong(ctx, arg.(*thrift.TestServiceEchoPingPongArgs).Req)
					if err != nil {
						return err
					}
					result.(*thrift.TestServiceEchoPingPongResult).Success = resp
					return nil
				},
				newArgs, newResult, false,
				serviceinfo.WithStreamingMode(serviceinfo.StreamingNone),
			),
			"Ping": serviceinfo.NewMethodInfo(
				func(ctx context.Context, handler, arg, result any) error {
					resp, err := handler.(testService2).Ping(ctx, arg.(*thrift.TestServiceEchoPingPongArgs).Req)
					if err != nil {
						return err
					}
					result.(*thrift.TestServiceEchoPingPongResult).Success = resp
					return nil
				},
				newArgs, newResult, false,
				serviceinfo.WithStreamingMode(serviceinfo.StreamingNone),
			),
		},
		Extra: map[string]any{"PackageName": "thrift"},
	}
}

// --- tests ---

func TestLocalCaller_BasicUnary(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	args := &thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{Message: "hello"}}
	result := &thrift.TestServiceEchoPingPongResult{}
	err = lc.Call(context.Background(), "EchoPingPong", args, result)
	test.Assert(t, err == nil, err)
	test.Assert(t, result.Success != nil && result.Success.Message == "echo:hello", result)
}

func TestLocalCaller_MiddlewareExecution(t *testing.T) {
	var called bool
	mw := func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			called = true
			return next(ctx, req, resp)
		}
	}
	svr := server.NewServer(server.WithMiddleware(mw))
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err == nil, err)
	test.Assert(t, called, "middleware was not called")
}

func TestLocalCaller_MultipleMiddlewaresOrdering(t *testing.T) {
	var order []int
	mkMW := func(id int) endpoint.Middleware {
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, req, resp any) error {
				order = append(order, id)
				return next(ctx, req, resp)
			}
		}
	}
	svr := server.NewServer(server.WithMiddleware(mkMW(1)), server.WithMiddleware(mkMW(2)))
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err == nil, err)
	test.Assert(t, len(order) == 2 && order[0] == 1 && order[1] == 2, order)
}

func TestLocalCaller_MethodNotFound(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "NoSuchMethod",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "not found"), err)
}

func TestLocalCaller_ServiceNotFound(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "BadService/EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "not found"), err)
}

func TestLocalCaller_InvalidServerConfig(t *testing.T) {
	svr := server.NewServer()
	_, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err != nil)

	// empty caller
	svr2 := server.NewServer()
	svr2.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	_, err = server.NewLocalCaller("", svr2)
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "caller"), err)
}

func TestLocalCaller_HandlerPanic(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &panicHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err != nil)
	test.Assert(t, errors.Is(err, kerrors.ErrPanic), err)
}

func TestLocalCaller_RPCInfoAndMethodContext(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &rpcInfoHandler{t: t})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err == nil, err)
}

func TestLocalCaller_ACLRules(t *testing.T) {
	denied := errors.New("access denied")
	rule := func(ctx context.Context, req any) error { return denied }
	svr := server.NewServer(server.WithACLRules(acl.RejectFunc(rule)))
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "access denied"), err)
}

func TestLocalCaller_ErrorHandler(t *testing.T) {
	wrapped := errors.New("wrapped")
	errFn := func(ctx context.Context, err error) error { return wrapped }
	svr := server.NewServer(server.WithErrorHandler(errFn))
	svr.RegisterService(testservice.NewServiceInfo(), &errorHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, errors.Is(err, wrapped), err)
}

func TestLocalCaller_BizStatusError(t *testing.T) {
	tracer := &traceRecorder{}
	errHandlerCalled := false
	errFn := func(ctx context.Context, err error) error {
		errHandlerCalled = true
		return err
	}
	svr := server.NewServer(server.WithErrorHandler(errFn), server.WithTracer(stats.Tracer(tracer)))
	svr.RegisterService(testservice.NewServiceInfo(), &bizErrHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err != nil)
	be, ok := kerrors.FromBizStatusError(err)
	test.Assert(t, ok, "expected BizStatusError")
	test.Assert(t, be.BizStatusCode() == 100, be.BizStatusCode())
	test.Assert(t, !errHandlerCalled, "ErrorHandler should not be called for biz errors")
	test.Assert(t, tracer.finished, "tracer finish should run for biz errors")
	test.Assert(t, tracer.lastErr == nil, tracer.lastErr)
}

func TestLocalCaller_PanicRecovered(t *testing.T) {
	tracer := &traceRecorder{}
	panicMW := func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			panic("mw panic")
		}
	}
	svr := server.NewServer(server.WithMiddleware(panicMW), server.WithTracer(stats.Tracer(tracer)))
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	assertPanicErr(t, err)
	test.Assert(t, tracer.finished, "tracer finish should run after panic recovery")
}

func TestLocalCaller_MultiServiceExplicit(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{}, server.WithFallbackService())
	svr.RegisterService(newTestService2Info(), &testService2Handler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	// Call TestService/EchoPingPong
	r1 := &thrift.TestServiceEchoPingPongResult{}
	err = lc.Call(context.Background(), "TestService/EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{Message: "hi"}}, r1)
	test.Assert(t, err == nil, err)
	test.Assert(t, r1.Success != nil && r1.Success.Message == "echo:hi", r1)

	// Call TestService2/Ping
	r2 := &thrift.TestServiceEchoPingPongResult{}
	err = lc.Call(context.Background(), "TestService2/Ping",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}}, r2)
	test.Assert(t, err == nil, err)
	test.Assert(t, r2.Success != nil && r2.Success.Message == "pong", r2)
}

func TestLocalCaller_ShortMethodNamePrefersFallbackService(t *testing.T) {
	// Both TestService and TestService2 have "EchoPingPong"; bare method lookup
	// should pick the fallback service to match service-less server routing.
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{}, server.WithFallbackService())
	svr.RegisterService(newTestService2Info(), &testService2Handler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	r := &thrift.TestServiceEchoPingPongResult{}
	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{Message: "hi"}}, r)
	test.Assert(t, err == nil, err)
	test.Assert(t, r.Success != nil && r.Success.Message == "echo:hi", r)
}

func TestLocalCaller_UniqueShortMethodName(t *testing.T) {
	// "Ping" is unique to TestService2
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{}, server.WithFallbackService())
	svr.RegisterService(newTestService2Info(), &testService2Handler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	r := &thrift.TestServiceEchoPingPongResult{}
	err = lc.Call(context.Background(), "Ping",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}}, r)
	test.Assert(t, err == nil, err)
	test.Assert(t, r.Success != nil && r.Success.Message == "pong", r)
}

func TestLocalCaller_CallRejectsTrueStreamingMethods(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	// Echo is StreamingBidirectional, EchoClient is StreamingClient,
	// EchoServer is StreamingServer. EchoUnary is unary over streaming transport
	// and remains supported by Call.
	for _, method := range []string{"Echo", "EchoClient", "EchoServer"} {
		err = lc.Call(context.Background(), method,
			&streaming.Args{}, &streaming.Result{})
		test.Assert(t, err != nil, "expected error for streaming method %s", method)
		test.Assert(t, strings.Contains(err.Error(), "streaming"), err)
	}
}

func TestLocalCaller_StreamGRPC_Bidirectional(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &streamingHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	res := new(streaming.Result)
	err = lc.Stream(context.Background(), "Echo", nil, res)
	test.Assert(t, err == nil, err)
	test.Assert(t, res.Stream != nil)

	err = res.Stream.SendMsg(&thrift.Request{Message: "hello"})
	test.Assert(t, err == nil, err)
	got := new(thrift.Response)
	err = res.Stream.RecvMsg(got)
	test.Assert(t, err == nil, err)
	test.Assert(t, got.Message == "bidi:hello", got)
	err = res.Stream.Close()
	test.Assert(t, err == nil, err)
	err = res.Stream.RecvMsg(new(thrift.Response))
	test.Assert(t, err == io.EOF, err)
}

func TestLocalCaller_StreamX_Bidirectional(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &streamingHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	st, err := lc.StreamX(context.Background(), "Echo")
	test.Assert(t, err == nil, err)
	err = st.SendMsg(context.Background(), &thrift.Request{Message: "streamx"})
	test.Assert(t, err == nil, err)
	got := new(thrift.Response)
	err = st.RecvMsg(context.Background(), got)
	test.Assert(t, err == nil, err)
	test.Assert(t, got.Message == "bidi:streamx", got)
	err = st.CloseSend(context.Background())
	test.Assert(t, err == nil, err)
	err = st.RecvMsg(context.Background(), new(thrift.Response))
	test.Assert(t, err == io.EOF, err)
}

func TestLocalCaller_StreamXTraceEvents(t *testing.T) {
	tracer := &streamTraceRecorder{}
	svr := server.NewServer(
		server.WithTracer(stats.Tracer(tracer)),
		server.WithStreamOptions(server.WithStreamEventHandler(rpcinfo.ServerStreamEventHandler{
			HandleStreamRecvEvent: tracer.onRecv,
			HandleStreamSendEvent: tracer.onSend,
		})),
	)
	svr.RegisterService(testservice.NewServiceInfo(), &streamingHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	st, err := lc.StreamX(context.Background(), "Echo")
	test.Assert(t, err == nil, err)
	err = st.SendMsg(context.Background(), &thrift.Request{Message: "trace"})
	test.Assert(t, err == nil, err)
	got := new(thrift.Response)
	err = st.RecvMsg(context.Background(), got)
	test.Assert(t, err == nil, err)
	test.Assert(t, got.Message == "bidi:trace", got)
	err = st.CloseSend(context.Background())
	test.Assert(t, err == nil, err)
	err = st.RecvMsg(context.Background(), new(thrift.Response))
	test.Assert(t, err == io.EOF, err)

	started, finished, recv, send := tracer.snapshot()
	test.Assert(t, started == 1, started)
	test.Assert(t, finished == 1, finished)
	// LocalCaller acts as both local client and local server. For one bidi
	// request/response pair, stream events should be emitted on both sides:
	// client send + server send, client recv + server recv. EOF recv is filtered
	// by TraceController and should not be counted.
	test.Assert(t, send == 2, send)
	test.Assert(t, recv == 2, recv)
}

func TestLocalCaller_StreamGRPC_ClientStreaming(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &streamingHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	res := new(streaming.Result)
	err = lc.Stream(context.Background(), "EchoClient", nil, res)
	test.Assert(t, err == nil, err)
	err = res.Stream.SendMsg(&thrift.Request{Message: "a"})
	test.Assert(t, err == nil, err)
	err = res.Stream.SendMsg(&thrift.Request{Message: "b"})
	test.Assert(t, err == nil, err)
	err = res.Stream.Close()
	test.Assert(t, err == nil, err)

	got := new(thrift.Response)
	err = res.Stream.RecvMsg(got)
	test.Assert(t, err == nil, err)
	test.Assert(t, got.Message == "client:a,b", got)
}

func TestLocalCaller_StreamGRPC_ServerStreaming(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &streamingHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	res := new(streaming.Result)
	err = lc.Stream(context.Background(), "EchoServer", nil, res)
	test.Assert(t, err == nil, err)
	err = res.Stream.SendMsg(&thrift.Request{Message: "x"})
	test.Assert(t, err == nil, err)
	err = res.Stream.Close()
	test.Assert(t, err == nil, err)

	got1 := new(thrift.Response)
	err = res.Stream.RecvMsg(got1)
	test.Assert(t, err == nil, err)
	test.Assert(t, got1.Message == "server:x:1", got1)
	got2 := new(thrift.Response)
	err = res.Stream.RecvMsg(got2)
	test.Assert(t, err == nil, err)
	test.Assert(t, got2.Message == "server:x:2", got2)
	err = res.Stream.RecvMsg(new(thrift.Response))
	test.Assert(t, err == io.EOF, err)
}

func TestLocalCaller_StreamBizStatusError(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &streamingHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	res := new(streaming.Result)
	err = lc.Stream(context.Background(), "EchoBizException", nil, res)
	test.Assert(t, err == nil, err)
	err = res.Stream.SendMsg(&thrift.Request{Message: "x"})
	test.Assert(t, err == nil, err)
	err = res.Stream.Close()
	test.Assert(t, err == nil, err)

	// server returns a BizStatusError; client should observe it via RecvMsg,
	// matching normal Kitex streaming client behavior.
	err = res.Stream.RecvMsg(new(thrift.Response))
	bizErr, ok := kerrors.FromBizStatusError(err)
	test.Assert(t, ok, err)
	test.Assert(t, bizErr.BizStatusCode() == 100, bizErr)
}

func TestLocalCaller_CallStreamingUnary(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &streamingHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	r := &thrift.TestServiceEchoUnaryResult{}
	err = lc.Call(context.Background(), "EchoUnary",
		&thrift.TestServiceEchoUnaryArgs{Req: &thrift.Request{Message: "u"}}, r)
	test.Assert(t, err == nil, err)
	test.Assert(t, r.Success != nil && r.Success.Message == "stream-unary:u", r)
}

func TestLocalCaller_ArgsResultTypeMismatch(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	// wrong args type
	err = lc.Call(context.Background(), "EchoPingPong", "bad", &thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "args type mismatch"), err)

	// wrong result type
	err = lc.Call(context.Background(), "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}}, "bad")
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "result type mismatch"), err)
}

func TestLocalCaller_ResolveMethod(t *testing.T) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{}, server.WithFallbackService())
	svr.RegisterService(newTestService2Info(), &testService2Handler{})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	// short form — unique method
	mi, err := lc.ResolveMethod(context.Background(), "Ping")
	test.Assert(t, err == nil, err)
	test.Assert(t, mi != nil)

	// long form
	mi, err = lc.ResolveMethod(context.Background(), "TestService2/Ping")
	test.Assert(t, err == nil, err)
	test.Assert(t, mi != nil)

	// duplicate short form should still resolve via the fallback service
	mi, err = lc.ResolveMethod(context.Background(), "EchoPingPong")
	test.Assert(t, err == nil, err)
	test.Assert(t, mi != nil)

	// not found
	_, err = lc.ResolveMethod(context.Background(), "NoSuch")
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "not found"), err)

	// empty service name
	_, err = lc.ResolveMethod(context.Background(), "/Ping")
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "empty service name"), err)

	// empty method name
	_, err = lc.ResolveMethod(context.Background(), "TestService2/")
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "empty method name"), err)

	// cached: second call should also succeed
	mi, err = lc.ResolveMethod(context.Background(), "Ping")
	test.Assert(t, err == nil, err)
	test.Assert(t, mi != nil)
}

func TestLocalCaller_GenericMethodUnsupported(t *testing.T) {
	svr := genericserver.NewServer(&genericHandler{}, generic.BinaryThriftGenericV2("GenericSvc"))
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	_, err = lc.ResolveMethod(context.Background(), "GenericSvc/AnyMethod")
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "GenericMethod"), err)
}

func TestLocalCaller_ContextPropagation(t *testing.T) {
	type ctxKey struct{}
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &ctxHandler{key: ctxKey{}})
	lc, err := server.NewLocalCaller("test-caller", svr)
	test.Assert(t, err == nil, err)

	ctx := context.WithValue(context.Background(), ctxKey{}, "propagated")
	err = lc.Call(ctx, "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err == nil, err)

	// deadline propagation
	svr2 := server.NewServer()
	svr2.RegisterService(testservice.NewServiceInfo(), &deadlineHandler{})
	lc2, err := server.NewLocalCaller("test-caller", svr2)
	test.Assert(t, err == nil, err)

	ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = lc2.Call(ctx2, "EchoPingPong",
		&thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{}},
		&thrift.TestServiceEchoPingPongResult{})
	test.Assert(t, err == nil, err)
}

func BenchmarkLocalCaller_Call(b *testing.B) {
	svr := server.NewServer()
	svr.RegisterService(testservice.NewServiceInfo(), &baseHandler{})
	lc, err := server.NewLocalCaller("bench-caller", svr)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	args := &thrift.TestServiceEchoPingPongArgs{Req: &thrift.Request{Message: "hello"}}
	result := &thrift.TestServiceEchoPingPongResult{}
	for i := 0; i < b.N; i++ {
		if err := lc.Call(ctx, "EchoPingPong", args, result); err != nil {
			b.Fatal(err)
		}
	}
}
