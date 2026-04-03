/*
 * Copyright 2021 CloudWeGo Authors
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
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/cloudwego/gopkg/protocol/thrift"

	"github.com/cloudwego/kitex/internal/mocks"
	mocksdiscovery "github.com/cloudwego/kitex/internal/mocks/discovery"
	mocksproxy "github.com/cloudwego/kitex/internal/mocks/proxy"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/event"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
)

var (
	instance404 = []discovery.Instance{
		discovery.NewInstance("tcp", "localhost:404", 1, make(map[string]string)),
	}
	instance505 = discovery.NewInstance("tcp", "localhost:505", 1, make(map[string]string))

	ctx = func() context.Context {
		ctx := context.Background()
		ctx = context.WithValue(ctx, endpoint.CtxEventBusKey, event.NewEventBus())
		ctx = context.WithValue(ctx, endpoint.CtxEventQueueKey, event.NewQueue(10))
		return ctx
	}()
	tcpAddrStr = "127.0.0.1:9909"
)

func resolver404(ctrl *gomock.Controller) discovery.Resolver {
	resolver := mocksdiscovery.NewMockResolver(ctrl)
	resolver.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(discovery.Result{
		Cacheable: true,
		CacheKey:  "test",
		Instances: instance404,
	}, nil).AnyTimes()
	resolver.EXPECT().Diff(gomock.Any(), gomock.Any(), gomock.Any()).Return(discovery.Change{}, false).AnyTimes()
	resolver.EXPECT().Name().Return("middlewares_test").AnyTimes()
	resolver.EXPECT().Target(gomock.Any(), gomock.Any()).AnyTimes()
	return resolver
}

func TestNoResolver(t *testing.T) {
	svcInfo := mocks.ServiceInfo()
	cli, err := NewClient(svcInfo, WithDestService("destService"))
	test.Assert(t, err == nil)

	err = cli.Call(context.Background(), "mock", mocks.NewMockArgs(), mocks.NewMockResult())
	test.Assert(t, errors.Is(err, kerrors.ErrNoResolver))
}

func TestResolverMW(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var invoked bool
	cli := newMockClient(t, ctrl).(*kcFinalizerClient)
	mw := newResolveMWBuilder(cli.lbf)(ctx)
	ep := func(ctx context.Context, request, response interface{}) error {
		invoked = true
		return nil
	}

	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
	ri := rpcinfo.NewRPCInfo(nil, to, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())

	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	req := new(MockTStruct)
	res := new(MockTStruct)
	err := mw(ep)(ctx, req, res)
	test.Assert(t, err == nil)
	test.Assert(t, invoked)
	test.Assert(t, to.GetInstance() == instance404[0])
}

func TestResolverMWErrGetConnection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cli := newMockClient(t, ctrl).(*kcFinalizerClient)
	mw := newResolveMWBuilder(cli.lbf)(ctx)
	ep := func(ctx context.Context, request, response interface{}) error {
		return kerrors.ErrGetConnection
	}

	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "mock_to_service"}, "")
	ri := rpcinfo.NewRPCInfo(nil, to, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())

	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	req := new(MockTStruct)
	res := new(MockTStruct)
	err := mw(ep)(ctx, req, res)
	test.Assert(t, err == kerrors.ErrGetConnection)
}

func TestResolverMWOutOfInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := &discovery.SynthesizedResolver{
		ResolveFunc: func(ctx context.Context, key string) (discovery.Result, error) {
			return discovery.Result{}, nil
		},
		NameFunc: func() string { return t.Name() },
	}
	var invoked bool
	cli := newMockClient(t, ctrl, WithResolver(resolver)).(*kcFinalizerClient)
	mw := newResolveMWBuilder(cli.lbf)(ctx)
	ep := func(ctx context.Context, request, response interface{}) error {
		invoked = true
		return nil
	}

	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
	ri := rpcinfo.NewRPCInfo(nil, to, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())

	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	req := new(MockTStruct)
	res := new(MockTStruct)

	err := mw(ep)(ctx, req, res)
	test.Assert(t, err != nil, err)
	test.Assert(t, errors.Is(err, kerrors.ErrNoMoreInstance))
	test.Assert(t, to.GetInstance() == nil)
	test.Assert(t, !invoked)
}

func TestDefaultErrorHandler(t *testing.T) {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", tcpAddrStr)
	ri := rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("mockService", "mockMethod", tcpAddr, nil),
		rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())
	reqCtx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

	// Test TApplicationException
	err := DefaultClientErrorHandler(context.Background(), thrift.NewApplicationException(100, "mock"))
	test.Assert(t, err.Error() == "remote or network error[remote]: mock", err.Error())
	var te *thrift.ApplicationException
	ok := errors.As(err, &te)
	test.Assert(t, ok)
	test.Assert(t, te.TypeID() == 100)
	// Test TApplicationException with remote addr
	err = ClientErrorHandlerWithAddr(reqCtx, thrift.NewApplicationException(100, "mock"))
	test.Assert(t, err.Error() == "remote or network error[remote-"+tcpAddrStr+"]: mock", err.Error())
	ok = errors.As(err, &te)
	test.Assert(t, ok)
	test.Assert(t, te.TypeID() == 100)

	// Test PbError
	err = DefaultClientErrorHandler(context.Background(), protobuf.NewPbError(100, "mock"))
	test.Assert(t, err.Error() == "remote or network error[remote]: mock")
	var pe protobuf.PBError
	ok = errors.As(err, &pe)
	test.Assert(t, ok)
	test.Assert(t, te.TypeID() == 100)
	// Test PbError with remote addr
	err = ClientErrorHandlerWithAddr(reqCtx, protobuf.NewPbError(100, "mock"))
	test.Assert(t, err.Error() == "remote or network error[remote-"+tcpAddrStr+"]: mock", err.Error())
	ok = errors.As(err, &pe)
	test.Assert(t, ok)
	test.Assert(t, te.TypeID() == 100)

	// Test status.Error
	err = DefaultClientErrorHandler(context.Background(), status.Err(100, "mock"))
	test.Assert(t, err.Error() == "remote or network error: rpc error: code = 100 desc = mock", err.Error())
	// Test status.Error with remote addr
	err = ClientErrorHandlerWithAddr(reqCtx, status.Err(100, "mock"))
	test.Assert(t, err.Error() == "remote or network error["+tcpAddrStr+"]: rpc error: code = 100 desc = mock", err.Error())

	// Test other error
	err = DefaultClientErrorHandler(context.Background(), errors.New("mock"))
	test.Assert(t, err.Error() == "remote or network error: mock")
	// Test other error with remote addr
	err = ClientErrorHandlerWithAddr(reqCtx, errors.New("mock"))
	test.Assert(t, err.Error() == "remote or network error["+tcpAddrStr+"]: mock")
}

func TestNewProxyMW(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := mocksproxy.NewMockForwardProxy(ctrl)
	p.EXPECT().ResolveProxyInstance(gomock.Any()).Return(nil).Times(1)

	mw := newProxyMW(p)

	err := mw(func(ctx context.Context, req, resp interface{}) (err error) {
		return nil
	})(context.Background(), nil, nil)
	test.Assert(t, err == nil)

	tp := mocksproxy.NewMockWithMiddleware(ctrl)
	tp.EXPECT().ProxyMiddleware().DoAndReturn(func() endpoint.Middleware {
		return func(e endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, req, resp interface{}) (err error) {
				return errors.New("execute proxy middleware")
			}
		}
	}).Times(1)
	mw = newProxyMW(&struct {
		proxy.ForwardProxy
		proxy.WithMiddleware
	}{
		WithMiddleware: tp,
	})
	err = mw(func(ctx context.Context, req, resp interface{}) (err error) {
		return nil
	})(context.Background(), nil, nil)
	test.Assert(t, err.Error() == "execute proxy middleware")
}

func BenchmarkResolverMW(b *testing.B) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	cli := newMockClient(b, ctrl).(*kcFinalizerClient)
	mw := newResolveMWBuilder(cli.lbf)(ctx)
	ep := func(ctx context.Context, request, response interface{}) error { return nil }
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())

	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	req := new(MockTStruct)
	res := new(MockTStruct)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mw(ep)(ctx, req, res)
	}
}

func BenchmarkResolverMWParallel(b *testing.B) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	cli := newMockClient(b, ctrl).(*kcFinalizerClient)
	mw := newResolveMWBuilder(cli.lbf)(ctx)
	ep := func(ctx context.Context, request, response interface{}) error { return nil }
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())

	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	req := new(MockTStruct)
	res := new(MockTStruct)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mw(ep)(ctx, req, res)
		}
	})
}

func TestDiscoveryEventHandler(t *testing.T) {
	t.Run("Added", func(t *testing.T) {
		bus, queue := event.NewEventBus(), event.NewQueue(200)
		h := discoveryEventHandler(discovery.ChangeEventName, bus, queue)
		ins := []discovery.Instance{discovery.NewInstance("tcp", "addr", 10, nil)}
		cacheKey := "testCacheKey"
		c := &discovery.Change{
			Result: discovery.Result{
				Cacheable: true,
				CacheKey:  cacheKey,
			},
			Added: ins,
		}
		h(c)
		events := queue.Dump().([]*event.Event)
		test.Assert(t, len(events) == 1)
		extra, ok := events[0].Extra.(map[string]interface{})
		test.Assert(t, ok)
		test.Assert(t, extra["Cacheable"] == true)
		test.Assert(t, extra["CacheKey"] == cacheKey)
		added := extra["Added"].([]*instInfo)
		test.Assert(t, len(added) == 1)
		test.Assert(t, added[0].Address == "tcp://addr")
		test.Assert(t, added[0].Weight == 10)
		test.Assert(t, len(extra["Updated"].([]*instInfo)) == 0)
		test.Assert(t, len(extra["Removed"].([]*instInfo)) == 0)
	})

	t.Run("Updated", func(t *testing.T) {
		bus, queue := event.NewEventBus(), event.NewQueue(200)
		h := discoveryEventHandler(discovery.ChangeEventName, bus, queue)
		c := &discovery.Change{
			Result:  discovery.Result{Cacheable: true, CacheKey: "key"},
			Updated: []discovery.Instance{discovery.NewInstance("tcp", "10.0.0.1:8080", 20, nil)},
		}
		h(c)
		extra := queue.Dump().([]*event.Event)[0].Extra.(map[string]interface{})
		test.Assert(t, len(extra["Added"].([]*instInfo)) == 0)
		updated := extra["Updated"].([]*instInfo)
		test.Assert(t, len(updated) == 1)
		test.Assert(t, updated[0].Address == "tcp://10.0.0.1:8080")
		test.Assert(t, updated[0].Weight == 20)
		test.Assert(t, len(extra["Removed"].([]*instInfo)) == 0)
	})

	t.Run("Removed", func(t *testing.T) {
		bus, queue := event.NewEventBus(), event.NewQueue(200)
		h := discoveryEventHandler(discovery.ChangeEventName, bus, queue)
		c := &discovery.Change{
			Result:  discovery.Result{Cacheable: false, CacheKey: "key"},
			Removed: []discovery.Instance{discovery.NewInstance("tcp", "10.0.0.2:9090", 30, nil)},
		}
		h(c)
		extra := queue.Dump().([]*event.Event)[0].Extra.(map[string]interface{})
		test.Assert(t, extra["Cacheable"] == false)
		test.Assert(t, len(extra["Added"].([]*instInfo)) == 0)
		test.Assert(t, len(extra["Updated"].([]*instInfo)) == 0)
		removed := extra["Removed"].([]*instInfo)
		test.Assert(t, len(removed) == 1)
		test.Assert(t, removed[0].Address == "tcp://10.0.0.2:9090")
		test.Assert(t, removed[0].Weight == 30)
	})

	t.Run("AllEmpty", func(t *testing.T) {
		bus, queue := event.NewEventBus(), event.NewQueue(200)
		h := discoveryEventHandler(discovery.ChangeEventName, bus, queue)
		c := &discovery.Change{
			Result: discovery.Result{Cacheable: true, CacheKey: "empty"},
		}
		h(c)
		extra := queue.Dump().([]*event.Event)[0].Extra.(map[string]interface{})
		test.Assert(t, extra["CacheKey"] == "empty")
		test.Assert(t, len(extra["Added"].([]*instInfo)) == 0)
		test.Assert(t, len(extra["Updated"].([]*instInfo)) == 0)
		test.Assert(t, len(extra["Removed"].([]*instInfo)) == 0)
	})

	t.Run("Mixed", func(t *testing.T) {
		bus, queue := event.NewEventBus(), event.NewQueue(200)
		h := discoveryEventHandler(discovery.ChangeEventName, bus, queue)
		c := &discovery.Change{
			Result: discovery.Result{Cacheable: true, CacheKey: "mixed"},
			Added:  []discovery.Instance{discovery.NewInstance("tcp", "10.0.0.1:9090", 1, nil)},
			Updated: []discovery.Instance{
				discovery.NewInstance("tcp", "10.0.0.2:9090", 2, nil),
				discovery.NewInstance("tcp", "10.0.0.3:9090", 3, nil),
			},
			Removed: []discovery.Instance{discovery.NewInstance("tcp", "10.0.0.4:9090", 4, nil)},
		}
		h(c)
		extra := queue.Dump().([]*event.Event)[0].Extra.(map[string]interface{})
		added := extra["Added"].([]*instInfo)
		updated := extra["Updated"].([]*instInfo)
		removed := extra["Removed"].([]*instInfo)
		test.Assert(t, len(added) == 1)
		test.Assert(t, len(updated) == 2)
		test.Assert(t, len(removed) == 1)
		test.Assert(t, updated[0].Address == "tcp://10.0.0.2:9090")
		test.Assert(t, updated[1].Address == "tcp://10.0.0.3:9090")
	})
}

// oldWrapInstances is the original implementation for benchmark comparison.
func oldWrapInstances(insts []discovery.Instance) []*instInfo {
	if len(insts) == 0 {
		return nil
	}
	instInfos := make([]*instInfo, 0, len(insts))
	for i := range insts {
		inst := insts[i]
		addr := fmt.Sprintf("%s://%s", inst.Address().Network(), inst.Address().String())
		instInfos = append(instInfos, &instInfo{Address: addr, Weight: inst.Weight()})
	}
	return instInfos
}

func makeInstances(n int) []discovery.Instance {
	insts := make([]discovery.Instance, n)
	for i := 0; i < n; i++ {
		tags := map[string]string{
			"cluster":      "default",
			"dc":           "dc",
			"zone":         "zone-a",
			"env":          "production",
			"canary":       "false",
			"version":      "v1.2.3",
			"container_id": fmt.Sprintf("container-%06d", i),
			"pod_name":     fmt.Sprintf("svc-abcdef-%06d", i),
			"namespace":    "default",
			"deploy_stage": "stable",
		}
		insts[i] = discovery.NewInstance("tcp", fmt.Sprintf("10.0.%d.%d:8080", i/256, i%256), 100, tags)
	}
	return insts
}

func makeChange(insts []discovery.Instance) *discovery.Change {
	return &discovery.Change{
		Result: discovery.Result{
			Cacheable: true,
			CacheKey:  "bench-service",
			Instances: insts,
		},
		Updated: insts,
	}
}

func BenchmarkDiscoveryEventPush(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, n := range sizes {
		insts := makeInstances(n)
		change := makeChange(insts)

		b.Run(fmt.Sprintf("Old/%d", n), func(b *testing.B) {
			queue := event.NewQueue(event.GetDefaultEventNum())
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				queue.Push(&event.Event{
					Name: "test",
					Time: time.Now(),
					Extra: map[string]interface{}{
						"Cacheable": change.Result.Cacheable,
						"CacheKey":  change.Result.CacheKey,
						"Added":     oldWrapInstances(change.Added),
						"Updated":   oldWrapInstances(change.Updated),
						"Removed":   oldWrapInstances(change.Removed),
					},
				})
			}
		})
		b.Run(fmt.Sprintf("New/%d", n), func(b *testing.B) {
			queue := event.NewQueue(event.GetDefaultEventNum())
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				queue.Push(&event.Event{
					Name: "test",
					Time: time.Now(),
					Extra: &discoveryEventExtra{
						cacheable: change.Result.Cacheable,
						cacheKey:  change.Result.CacheKey,
						added:     snapshotInstances(change.Added),
						updated:   snapshotInstances(change.Updated),
						removed:   snapshotInstances(change.Removed),
					},
				})
			}
		})
	}
}

func BenchmarkDiscoveryEventDump(b *testing.B) {
	insts := makeInstances(1000)
	change := makeChange(insts)

	b.Run("Old/1000", func(b *testing.B) {
		defEvtNum := event.GetDefaultEventNum()
		queue := event.NewQueue(defEvtNum)
		for i := 0; i < defEvtNum; i++ {
			queue.Push(&event.Event{
				Name: "test",
				Time: time.Now(),
				Extra: map[string]interface{}{
					"Cacheable": change.Result.Cacheable,
					"CacheKey":  change.Result.CacheKey,
					"Added":     oldWrapInstances(change.Added),
					"Updated":   oldWrapInstances(change.Updated),
					"Removed":   oldWrapInstances(change.Removed),
				},
			})
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			queue.Dump()
		}
	})
	b.Run("New/1000", func(b *testing.B) {
		defEvtNum := event.GetDefaultEventNum()
		queue := event.NewQueue(defEvtNum)
		for i := 0; i < defEvtNum; i++ {
			queue.Push(&event.Event{
				Name: "test",
				Time: time.Now(),
				Extra: &discoveryEventExtra{
					cacheable: change.Result.Cacheable,
					cacheKey:  change.Result.CacheKey,
					added:     snapshotInstances(change.Added),
					updated:   snapshotInstances(change.Updated),
					removed:   snapshotInstances(change.Removed),
				},
			})
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			queue.Dump()
		}
	})
}
