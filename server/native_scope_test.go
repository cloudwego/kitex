package server_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
)

type nativeScopeFixtureKey struct{}

type nativeScopeFixtureRequest struct {
	*nativeFixtureRequest
	ctx              context.Context
	cancel           context.CancelFunc
	peer             *nativePeerCancellationContext
	supported        bool
	mu               sync.Mutex
	cancels          []context.CancelFunc
	deadlines        *atomic.Int32
	failDeadlineCall int
	deadlineCalls    int
	deadlineError    error
	finished         chan struct{}
}

func (r *nativeScopeFixtureRequest) Context() context.Context { return r.peer }
func (r *nativeScopeFixtureRequest) RequestContext() (context.Context, bool) {
	return r.ctx, r.supported
}
func (r *nativeScopeFixtureRequest) WithRequestDeadline(ctx context.Context, deadline time.Time) (context.Context, error) {
	r.deadlineCalls++
	if r.deadlineCalls == r.failDeadlineCall {
		return nil, r.deadlineError
	}
	if current, ok := ctx.Deadline(); ok && !deadline.Before(current) {
		return ctx, nil
	}
	r.deadlines.Add(1)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	r.mu.Lock()
	r.cancels = append(r.cancels, cancel)
	r.mu.Unlock()
	return ctx, nil
}
func (r *nativeScopeFixtureRequest) Release(reuse bool) error {
	defer close(r.finished)
	r.mu.Lock()
	for _, cancel := range r.cancels {
		cancel()
	}
	r.mu.Unlock()
	r.cancel()
	return r.nativeFixtureRequest.Release(reuse)
}

func nativeFixtureScope(bridge *nativeServerFixture, supported bool, deadlines *atomic.Int32, requests chan<- *nativeScopeFixtureRequest) {
	bridge.request = func(request *nativeFixtureRequest) remote.NativeServerRequest {
		ctx, cancel := context.WithCancel(context.WithValue(bridge.requestContext, nativeScopeFixtureKey{}, "bound-scope"))
		r := &nativeScopeFixtureRequest{nativeFixtureRequest: request, ctx: ctx, cancel: cancel, supported: supported, deadlines: deadlines,
			peer: &nativePeerCancellationContext{Context: context.Background(), done: make(chan struct{})}, finished: make(chan struct{})}
		requests <- r
		return r
	}
}

func TestNativeServerOptionalScopePreservesValuesAndLegacy(t *testing.T) {
	for _, supported := range []bool{true, false} {
		t.Run(map[bool]string{true: "scoped", false: "legacy"}[supported], func(t *testing.T) {
			bridge := new(nativeServerFixture)
			requests := make(chan *nativeScopeFixtureRequest, 1)
			var deadlines atomic.Int32
			nativeFixtureScope(bridge, supported, &deadlines, requests)
			cli, _ := startNativeFixture(t, bridge, new(nativeFixtureEcho), server.WithMiddleware(func(next endpoint.Endpoint) endpoint.Endpoint {
				return func(ctx context.Context, req, resp interface{}) error {
					r := <-requests
					if (ctx.Value(nativeScopeFixtureKey{}) == "bound-scope") != supported {
						return errors.New("optional scope selection lost bound request values")
					}
					if ctx.Value(endpoint.CtxEventBusKey) == nil || ctx.Value(endpoint.CtxEventQueueKey) == nil {
						return errors.New("bound request lost server context values")
					}
					if r.peer.active.Load() == supported {
						return errors.New("scope installed a duplicate peer cancellation bridge or legacy skipped its bridge")
					}
					return next(ctx, req, resp)
				}
			}))
			if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "scope-values"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNativeServerScopedDeadlineSurvivesProjection(t *testing.T) {
	bridge := &nativeServerFixture{admissionTimeout: 100 * time.Millisecond, projectBase: true}
	requests := make(chan *nativeScopeFixtureRequest, 1)
	var deadlines atomic.Int32
	nativeFixtureScope(bridge, true, &deadlines, requests)
	cli, _ := startNativeFixture(t, bridge, new(nativeFixtureEcho), server.WithMiddleware(func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) error {
			if deadline, ok := ctx.Deadline(); !ok || !deadline.Equal(remote.UnaryCallFromContext(ctx).Begin.Add(bridge.admissionTimeout)) || deadlines.Load() == 0 {
				return errors.New("scoped budget was not installed as the admitted absolute deadline")
			}
			return next(ctx, req, resp)
		}
	}))
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "scope-deadline"}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeServerScopedForeignCancellationSurvivesProjection(t *testing.T) {
	foreign, cancel := context.WithCancelCause(context.Background())
	defer cancel(context.Canceled)
	want := errors.New("custom metadata canceled")
	bridge := &nativeServerFixture{projectBase: true, completed: make(chan nativeFixtureCompletion, 1)}
	bridge.readContext = func(ctx context.Context) context.Context {
		return rpcinfo.NewCtxWithRPCInfo(context.WithValue(foreign, nativeMetaKey{}, "read"), rpcinfo.GetRPCInfo(ctx))
	}
	requests := make(chan *nativeScopeFixtureRequest, 1)
	var deadlines atomic.Int32
	nativeFixtureScope(bridge, true, &deadlines, requests)
	observed := make(chan error, 1)
	cli, _ := startNativeFixture(t, bridge, new(nativeFixtureEcho), server.WithMiddleware(func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) error {
			cancel(want)
			select {
			case <-ctx.Done():
				if ctx.Err() != context.Canceled || context.Cause(ctx) != want {
					observed <- errors.New("foreign context cancellation or cause was lost during projection")
				} else {
					observed <- nil
				}
			case <-time.After(time.Second):
				observed <- errors.New("foreign context cancellation did not reach business context")
			}
			return ctx.Err()
		}
	}))
	_, _ = cli.EchoPingPong(context.Background(), &kt.Request{Message: "foreign-cancel"})
	select {
	case err := <-observed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("foreign cancellation request did not reach the handler")
	}
	select {
	case completion := <-bridge.completed:
		if !errors.Is(completion.err, context.Canceled) {
			t.Fatalf("foreign cancellation did not complete governance: %v", completion.err)
		}
	case <-time.After(time.Second):
		t.Fatal("foreign cancellation did not finish the request")
	}
}

func TestNativeServerScopedForeignDeadlineTightensBeforeProjection(t *testing.T) {
	var want time.Time
	bridge := &nativeServerFixture{projectBase: true, completed: make(chan nativeFixtureCompletion, 1)}
	bridge.readContext = func(ctx context.Context) context.Context {
		foreign, cancel := context.WithDeadline(context.Background(), time.Now().Add(50*time.Millisecond))
		t.Cleanup(cancel)
		want, _ = foreign.Deadline()
		return rpcinfo.NewCtxWithRPCInfo(context.WithValue(foreign, nativeMetaKey{}, "read"), rpcinfo.GetRPCInfo(ctx))
	}
	requests := make(chan *nativeScopeFixtureRequest, 1)
	var deadlines atomic.Int32
	nativeFixtureScope(bridge, true, &deadlines, requests)
	observed := make(chan error, 1)
	cli, _ := startNativeFixture(t, bridge, new(nativeFixtureEcho), server.WithMiddleware(func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) error {
			if actual, ok := ctx.Deadline(); !ok || !actual.Equal(want) || deadlines.Load() == 0 {
				observed <- errors.New("foreign deadline was lost or native tightening was skipped")
				return errors.New("invalid scoped deadline")
			}
			select {
			case <-ctx.Done():
				if ctx.Err() != context.DeadlineExceeded || context.Cause(ctx) != context.DeadlineExceeded {
					observed <- errors.New("foreign deadline exceeded reason changed")
				} else {
					observed <- nil
				}
			case <-time.After(time.Second):
				observed <- errors.New("foreign deadline was not enforced")
			}
			return ctx.Err()
		}
	}))
	_, _ = cli.EchoPingPong(context.Background(), &kt.Request{Message: "foreign-deadline"})
	select {
	case err := <-observed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("foreign deadline request did not reach the handler")
	}
}

func TestNativeServerScopedDeadlineFailureStillReleasesRequest(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		for _, nilError := range []bool{false, true} {
			t.Run(fmt.Sprintf("call%d-nilError%t", failAt, nilError), func(t *testing.T) {
				bridge := new(nativeServerFixture)
				requests := make(chan *nativeScopeFixtureRequest, 1)
				var deadlines atomic.Int32
				nativeFixtureScope(bridge, true, &deadlines, requests)
				makeRequest := bridge.request
				bridge.request = func(request *nativeFixtureRequest) remote.NativeServerRequest {
					r := makeRequest(request).(*nativeScopeFixtureRequest)
					r.failDeadlineCall = failAt
					if !nilError {
						r.deadlineError = errors.New("native deadline installation failed")
					}
					ctx, cancel := context.WithDeadline(r.ctx, time.Now().Add(time.Second))
					r.ctx = ctx
					r.cancels = append(r.cancels, cancel)
					return r
				}
				tracer := &nativeFixtureTrace{finished: make(chan nativeFixtureTraceFacts, 1)}
				handler := new(nativeFixtureEcho)
				cli, _ := startNativeFixture(t, bridge, handler, server.WithTracer(tracer))
				if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "deadline-installation-failure"}); err == nil {
					t.Fatal("scope deadline failure did not fail the RPC")
				}
				r := <-requests
				select {
				case <-r.finished:
				case <-time.After(time.Second):
					t.Fatal("scope deadline failure leaked Finish")
				}
				if bridge.released.Load() != 1 || len(r.frame.Bytes()) != 0 || handler.calls.Load() != 0 {
					t.Fatalf("scope deadline failure leaked request or RX: releases=%d RX=%d calls=%d", bridge.released.Load(), len(r.frame.Bytes()), handler.calls.Load())
				}
				select {
				case <-tracer.finished:
				case <-time.After(time.Second):
					t.Fatal("scope deadline failure skipped tracer completion")
				}
			})
		}
	}
}
