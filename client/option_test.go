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
	"fmt"
	"github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/warmup"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/diagnosis"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/http"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/transport"
)

func TestRetryOptionDebugInfo(t *testing.T) {
	fp := retry.NewFailurePolicy()
	fp.WithDDLStop()
	expectPolicyStr := "WithFailureRetry({StopPolicy:{MaxRetryTimes:2 MaxDurationMS:0 DisableChainStop:false DDLStop:true CBPolicy:{ErrorRate:0.1}} BackOffPolicy:&{BackOffType:none CfgItems:map[]} RetrySameNode:false})"
	policyStr := fmt.Sprintf("WithFailureRetry(%+v)", *fp)
	test.Assert(t, policyStr == expectPolicyStr, policyStr)

	fp.WithFixedBackOff(10)
	expectPolicyStr = "WithFailureRetry({StopPolicy:{MaxRetryTimes:2 MaxDurationMS:0 DisableChainStop:false DDLStop:true CBPolicy:{ErrorRate:0.1}} BackOffPolicy:&{BackOffType:fixed CfgItems:map[fix_ms:10]} RetrySameNode:false})"
	policyStr = fmt.Sprintf("WithFailureRetry(%+v)", *fp)
	test.Assert(t, policyStr == expectPolicyStr, policyStr)

	fp.WithRandomBackOff(10, 20)
	fp.DisableChainRetryStop()
	expectPolicyStr = "WithFailureRetry({StopPolicy:{MaxRetryTimes:2 MaxDurationMS:0 DisableChainStop:true DDLStop:true CBPolicy:{ErrorRate:0.1}} BackOffPolicy:&{BackOffType:random CfgItems:map[max_ms:20 min_ms:10]} RetrySameNode:false})"
	policyStr = fmt.Sprintf("WithFailureRetry(%+v)", *fp)
	test.Assert(t, policyStr == expectPolicyStr, policyStr)

	fp.WithRetrySameNode()
	expectPolicyStr = "WithFailureRetry({StopPolicy:{MaxRetryTimes:2 MaxDurationMS:0 DisableChainStop:true DDLStop:true CBPolicy:{ErrorRate:0.1}} BackOffPolicy:&{BackOffType:random CfgItems:map[max_ms:20 min_ms:10]} RetrySameNode:true})"
	policyStr = fmt.Sprintf("WithFailureRetry(%+v)", *fp)
	test.Assert(t, policyStr == expectPolicyStr, policyStr)

	bp := retry.NewBackupPolicy(20)
	expectPolicyStr = "WithBackupRequest({RetryDelayMS:20 StopPolicy:{MaxRetryTimes:1 MaxDurationMS:0 DisableChainStop:false DDLStop:false CBPolicy:{ErrorRate:0.1}} RetrySameNode:false})"
	policyStr = fmt.Sprintf("WithBackupRequest(%+v)", *bp)
	test.Assert(t, policyStr == expectPolicyStr, policyStr)
	WithBackupRequest(bp)
}

func TestRetryOption(t *testing.T) {
	defer func() {
		err := recover()
		test.Assert(t, err != nil)
	}()
	fp := retry.NewFailurePolicy()
	var options []client.Option
	options = append(options, WithFailureRetry(fp))
	bp := retry.NewBackupPolicy(20)
	options = append(options, WithBackupRequest(bp))

	client.NewOptions(options)
	// both setup failure and backup retry, panic should happen
	test.Assert(t, false)
}

func TestTransportProtocolOption(t *testing.T) {
	var options []client.Option
	options = append(options, WithTransportProtocol(transport.GRPC))
	client.NewOptions(options)
}

func TestWithHostPorts(t *testing.T) {
	var options []client.Option
	options = append(options, WithHostPorts("127.0.0.1:8080"))
	opts := client.NewOptions(options)
	test.Assert(t, opts.Resolver.Name() == "127.0.0.1:8080")
	res, err := opts.Resolver.Resolve(context.Background(), "")
	test.Assert(t, err == nil)
	test.Assert(t, res.Instances[0].Address().String() == "127.0.0.1:8080")
}

func TestForwardProxy(t *testing.T) {
	fp := &mockForwardProxy{
		ConfigureFunc: func(cfg *proxy.Config) error {
			test.Assert(t, cfg.Resolver == nil, cfg.Resolver)
			test.Assert(t, cfg.Balancer == nil, cfg.Balancer)
			return nil
		},
		ResolveProxyInstanceFunc: func(ctx context.Context) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			re := remoteinfo.AsRemoteInfo(ri.To())
			in := re.GetInstance()
			test.Assert(t, in == nil, in)
			re.SetInstance(instance505)
			return nil
		},
	}

	var opts []Option
	opts = append(opts, WithTransHandlerFactory(mocks.NewMockCliTransHandlerFactory(hdlr)))
	opts = append(opts, WithDialer(dialer))
	opts = append(opts, WithDestService("destService"))
	opts = append(opts, WithProxy(fp))

	svcInfo := mocks.ServiceInfo()
	cli, err := NewClient(svcInfo, opts...)
	test.Assert(t, err == nil)

	mtd := mocks.MockMethod
	ctx := context.Background()
	req := new(MockTStruct)
	res := new(MockTStruct)

	err = cli.Call(ctx, mtd, req, res)
	test.Assert(t, err == nil, err)
}

func TestProxyWithResolver(t *testing.T) {
	fp := &mockForwardProxy{
		ConfigureFunc: func(cfg *proxy.Config) error {
			test.Assert(t, cfg.Resolver == resolver404)
			test.Assert(t, cfg.Balancer == nil, cfg.Balancer)
			return nil
		},
		ResolveProxyInstanceFunc: func(ctx context.Context) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			re := remoteinfo.AsRemoteInfo(ri.To())
			in := re.GetInstance()
			test.Assert(t, in == instance404[0])
			re.SetInstance(instance505)
			return nil
		},
	}

	var opts []Option
	opts = append(opts, WithTransHandlerFactory(mocks.NewMockCliTransHandlerFactory(hdlr)))
	opts = append(opts, WithDialer(dialer))
	opts = append(opts, WithDestService("destService"))
	opts = append(opts, WithProxy(fp))
	opts = append(opts, WithResolver(resolver404))

	svcInfo := mocks.ServiceInfo()
	cli, err := NewClient(svcInfo, opts...)
	test.Assert(t, err == nil)

	mtd := mocks.MockMethod
	ctx := context.Background()
	req := new(MockTStruct)
	res := new(MockTStruct)

	err = cli.Call(ctx, mtd, req, res)
	test.Assert(t, err == nil, err)
}

func TestProxyWithBalancer(t *testing.T) {
	lb := &loadbalance.SynthesizedLoadbalancer{
		GetPickerFunc: func(entry discovery.Result) loadbalance.Picker {
			return &loadbalance.SynthesizedPicker{
				NextFunc: func(ctx context.Context, request interface{}) discovery.Instance {
					if len(entry.Instances) > 0 {
						return entry.Instances[0]
					}
					return nil
				},
			}
		},
	}
	fp := &mockForwardProxy{
		ConfigureFunc: func(cfg *proxy.Config) error {
			test.Assert(t, cfg.Resolver == nil, cfg.Resolver)
			test.Assert(t, cfg.Balancer == lb)
			return nil
		},
		ResolveProxyInstanceFunc: func(ctx context.Context) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			re := remoteinfo.AsRemoteInfo(ri.To())
			in := re.GetInstance()
			test.Assert(t, in == nil)
			re.SetInstance(instance505)
			return nil
		},
	}

	var opts []Option
	opts = append(opts, WithTransHandlerFactory(mocks.NewMockCliTransHandlerFactory(hdlr)))
	opts = append(opts, WithDialer(dialer))
	opts = append(opts, WithDestService("destService"))
	opts = append(opts, WithProxy(fp))
	opts = append(opts, WithLoadBalancer(lb))

	svcInfo := mocks.ServiceInfo()
	cli, err := NewClient(svcInfo, opts...)
	test.Assert(t, err == nil)

	mtd := mocks.MockMethod
	ctx := context.Background()
	req := new(MockTStruct)
	res := new(MockTStruct)

	err = cli.Call(ctx, mtd, req, res)
	test.Assert(t, err == nil)
}

func TestProxyWithResolverAndBalancer(t *testing.T) {
	lb := &loadbalance.SynthesizedLoadbalancer{
		GetPickerFunc: func(entry discovery.Result) loadbalance.Picker {
			return &loadbalance.SynthesizedPicker{
				NextFunc: func(ctx context.Context, request interface{}) discovery.Instance {
					return instance505
				},
			}
		},
	}
	fp := &mockForwardProxy{
		ConfigureFunc: func(cfg *proxy.Config) error {
			test.Assert(t, cfg.Resolver == resolver404)
			test.Assert(t, cfg.Balancer == lb)
			return nil
		},
		ResolveProxyInstanceFunc: func(ctx context.Context) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			re := remoteinfo.AsRemoteInfo(ri.To())
			in := re.GetInstance()
			test.Assert(t, in == instance505)
			return nil
		},
	}

	var opts []Option
	opts = append(opts, WithTransHandlerFactory(mocks.NewMockCliTransHandlerFactory(hdlr)))
	opts = append(opts, WithDialer(dialer))
	opts = append(opts, WithDestService("destService"))
	opts = append(opts, WithProxy(fp))
	opts = append(opts, WithResolver(resolver404))
	opts = append(opts, WithLoadBalancer(lb))

	svcInfo := mocks.ServiceInfo()
	cli, err := NewClient(svcInfo, opts...)
	test.Assert(t, err == nil)

	mtd := mocks.MockMethod
	ctx := context.Background()
	req := new(MockTStruct)
	res := new(MockTStruct)

	err = cli.Call(ctx, mtd, req, res)
	test.Assert(t, err == nil)
}

var (
	// mock middleware, do nothing
	md = func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) (err error) {
			return nil
		}
	}
	httpResolver = http.NewDefaultResolver()
)

type mockCodec struct {
	EncodeFunc func(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error
	DecodeFunc func(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error
	NameFunc   func() string
}

func (m *mockCodec) Encode(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	if m.EncodeFunc != nil {
		return m.EncodeFunc(ctx, msg, out)
	}
	return nil
}

func (m *mockCodec) Decode(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if m.DecodeFunc != nil {
		return m.DecodeFunc(ctx, msg, in)
	}
	return nil
}

func (m *mockCodec) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return ""
}

// mock pay load codec
type mockPayloadCodec struct{}

// do nothing
func (m *mockPayloadCodec) Marshal(ctx context.Context, message remote.Message, out remote.ByteBuffer) error {
	return nil
}

// do nothing
func (m *mockPayloadCodec) Unmarshal(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	return nil
}

// return default name
func (m *mockPayloadCodec) Name() string {
	return "mock"
}

type mockTimeoutProvider struct{}

func (m *mockTimeoutProvider) Timeouts(ri rpcinfo.RPCInfo) rpcinfo.Timeouts {
	return rpcinfo.NewRPCConfig()
}

type mockDiagnosis struct {
	probes map[diagnosis.ProbeName]diagnosis.ProbeFunc
}

func (m *mockDiagnosis) RegisterProbeFunc(name diagnosis.ProbeName, probeFunc diagnosis.ProbeFunc) {
	m.probes[name] = probeFunc
}

func (m *mockDiagnosis) ProbePairs() map[diagnosis.ProbeName]diagnosis.ProbeFunc {
	return m.probes
}

type mockOutboundHandler struct {
	remote.BoundHandler
}

func (m *mockOutboundHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) (context.Context, error) {
	// do nothing
	return ctx, nil
}

type mockMetaHandler struct{}

func (m *mockMetaHandler) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	return ctx, nil
}

func (m *mockMetaHandler) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	return ctx, nil
}

func TestWithInstanceMW(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithInstanceMW(md)})
	test.Assert(t, len(opts.IMWBs) == 1, len(opts.IMWBs))
}

func TestWithHTTPResolver(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithHTTPResolver(httpResolver)})
	test.DeepEqual(t, opts.HTTPResolver, httpResolver)
}

func TestShortConnection(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithShortConnection()})
	test.Assert(t, opts.PoolCfg != nil, opts.PoolCfg)
}

func TestWithMuxConnection(t *testing.T) {
	connNum := 100
	opts := client.NewOptions([]client.Option{WithMuxConnection(connNum)})
	test.Assert(t, opts.RemoteOpt.ConnPool != nil)
	test.Assert(t, opts.RemoteOpt.CliHandlerFactory != nil)
	test.Assert(t, opts.Configs.TransportProtocol() == transport.TTHeader, opts.Configs.TransportProtocol())
}

func TestWithTimeoutProvider(t *testing.T) {
	mockTimeoutProvider := &mockTimeoutProvider{}
	opts := client.NewOptions([]client.Option{WithTimeoutProvider(mockTimeoutProvider)})
	test.DeepEqual(t, opts.Timeouts, mockTimeoutProvider)
}

func TestWithStatsLevel(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithStatsLevel(stats.LevelDisabled)})
	test.Assert(t, opts.StatsLevel != nil && *opts.StatsLevel == stats.LevelDisabled, opts.StatsLevel)
}

func TestWithCodec(t *testing.T) {
	mockCodec := &mockCodec{}
	opts := client.NewOptions([]client.Option{WithCodec(mockCodec)})
	test.DeepEqual(t, opts.RemoteOpt.Codec, mockCodec)
}

func TestWithPayloadCodec(t *testing.T) {
	mockPayloadCodec := &mockPayloadCodec{}
	opts := client.NewOptions([]client.Option{WithPayloadCodec(mockPayloadCodec)})
	test.DeepEqual(t, opts.RemoteOpt.PayloadCodec, mockPayloadCodec)
}

func TestWithConnReporterEnabled(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithConnReporterEnabled()})
	test.Assert(t, opts.RemoteOpt.EnableConnPoolReporter)
}

func TestWithCircuitBreaker(t *testing.T) {
	opts := client.NewOptions([]client.Option{
		WithCircuitBreaker(circuitbreak.NewCBSuite(func(ri rpcinfo.RPCInfo) string { return "" })),
	})
	test.Assert(t, opts.CBSuite != nil)
}

const mockUint32Size uint32 = 0

func TestWithGRPCConnPoolSize(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithGRPCConnPoolSize(mockUint32Size)})
	test.Assert(t, opts.GRPCConnPoolSize == mockUint32Size, opts.GRPCConnPoolSize)
}

func TestWithGRPCInitialWindowSize(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithGRPCInitialWindowSize(mockUint32Size)})
	test.Assert(t, opts.GRPCConnectOpts.InitialWindowSize == mockUint32Size, opts.GRPCConnectOpts.InitialWindowSize)
}

func TestWithGRPCInitialConnWindowSize(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithGRPCInitialConnWindowSize(mockUint32Size)})
	test.Assert(t, opts.GRPCConnectOpts.InitialConnWindowSize == mockUint32Size,
		opts.GRPCConnectOpts.InitialConnWindowSize)
}

func TestWithGRPCMaxHeaderListSize(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithGRPCMaxHeaderListSize(mockUint32Size)})
	test.Assert(t,
		opts.GRPCConnectOpts.MaxHeaderListSize != nil &&
			*opts.GRPCConnectOpts.MaxHeaderListSize == mockUint32Size,
		opts.GRPCConnectOpts.MaxHeaderListSize)
}

func TestWithGRPCKeepaliveParams(t *testing.T) {
	opts := client.NewOptions(
		[]client.Option{
			WithGRPCKeepaliveParams(grpc.ClientKeepalive{
				Time:                5 * time.Second, // less than  grpc.KeepaliveMinPingTime(5s)
				Timeout:             20 * time.Second,
				PermitWithoutStream: true,
			}),
		})
	test.Assert(t, opts.GRPCConnectOpts.KeepaliveParams.Time == grpc.KeepaliveMinPingTime,
		opts.GRPCConnectOpts.KeepaliveParams.Time)
	test.Assert(t, opts.GRPCConnectOpts.KeepaliveParams.Timeout == 20*time.Second,
		opts.GRPCConnectOpts.KeepaliveParams.Timeout)
	test.Assert(t, opts.GRPCConnectOpts.KeepaliveParams.PermitWithoutStream)
}

func TestWithHTTPConnection(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithHTTPConnection()})
	test.Assert(t, opts.RemoteOpt.CliHandlerFactory != nil)
}

func TestWithClientBasicInfo(t *testing.T) {
	mockEBI := &rpcinfo.EndpointBasicInfo{}
	opts := client.NewOptions([]client.Option{WithClientBasicInfo(mockEBI)})
	test.Assert(t, opts.Cli == mockEBI)
}

func TestWithDiagnosisService(t *testing.T) {
	mockDS := &mockDiagnosis{
		make(map[diagnosis.ProbeName]diagnosis.ProbeFunc),
	}
	opts := client.NewOptions([]client.Option{
		WithDiagnosisService(mockDS),
	})
	test.Assert(t, opts.DebugService == mockDS, opts.DebugService)
}

func TestWithACLRules(t *testing.T) {
	_ = client.NewOptions([]client.Option{WithACLRules()})
}

func TestWithFirstMetaHandler(t *testing.T) {
	mockMetaHandler := &mockMetaHandler{}
	opts := client.NewOptions([]client.Option{WithFirstMetaHandler(mockMetaHandler)})
	test.DeepEqual(t, opts.MetaHandlers[0], mockMetaHandler)
}

func TestWithMetaHandler(t *testing.T) {
	mockMetaHandler := &mockMetaHandler{}
	opts := client.NewOptions([]client.Option{WithMetaHandler(mockMetaHandler)})
	test.DeepEqual(t, opts.MetaHandlers[0], mockMetaHandler)
}

type mockConnPool struct{}

func (m *mockConnPool) Get(ctx context.Context, network, address string, opt remote.ConnOption) (net.Conn, error) {
	return nil, nil
}
func (m *mockConnPool) Put(conn net.Conn) error     { return nil }
func (m *mockConnPool) Discard(conn net.Conn) error { return nil }
func (m *mockConnPool) Close() error                { return nil }

func TestWithConnPool(t *testing.T) {
	mockPool := &mockConnPool{}
	opts := client.NewOptions([]client.Option{WithConnPool(mockPool)})
	test.Assert(t, opts.RemoteOpt.ConnPool == mockPool)
}

func TestWithRetryContainer(t *testing.T) {
	mockRetryContainer := &retry.Container{}
	opts := client.NewOptions([]client.Option{WithRetryContainer(mockRetryContainer)})
	test.Assert(t, opts.RetryContainer == mockRetryContainer)
}

func TestWithGeneric(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithGeneric(generic.BinaryThriftGeneric())})
	test.DeepEqual(t, opts.RemoteOpt.PayloadCodec, generic.BinaryThriftGeneric().PayloadCodec())
}

func TestWithCloseCallbacks(t *testing.T) {
	opts := client.NewOptions([]client.Option{WithCloseCallbacks(func() error { return nil })})
	test.Assert(t, len(opts.CloseCallbacks) > 0)
}

func TestWithErrorHandler(t *testing.T) {
	errHandler := func(err error) error { return nil }
	opts := client.NewOptions([]client.Option{WithErrorHandler(errHandler)})
	test.Assert(t, reflect.ValueOf(opts.ErrHandle).Pointer() == reflect.ValueOf(errHandler).Pointer())
}

func TestWithBoundHandler(t *testing.T) {
	mockOutboundHandler := &mockOutboundHandler{}
	opts := client.NewOptions([]client.Option{WithBoundHandler(mockOutboundHandler)})
	test.Assert(t, len(opts.RemoteOpt.Outbounds) > 0)
	test.Assert(t, len(opts.RemoteOpt.Inbounds) == 0)
}

// collection of options
type mockSuite struct{}

func (m *mockSuite) Options() []Option {
	return []Option{
		WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{}),
		WithDiagnosisService(&mockDiagnosis{make(map[diagnosis.ProbeName]diagnosis.ProbeFunc)}),
		WithACLRules(),
		WithRetryContainer(&retry.Container{}),
	}
}

func TestWithSuite(t *testing.T) {
	var suite *mockSuite
	options := []client.Option{
		WithSuite(suite),
	}
	_ = client.NewOptions(options)
}

type mockForwardProxy struct {
	ConfigureFunc            func(*proxy.Config) error
	ResolveProxyInstanceFunc func(ctx context.Context) error
}

func (m *mockForwardProxy) Configure(cfg *proxy.Config) error {
	if m.ConfigureFunc != nil {
		return m.ConfigureFunc(cfg)
	}
	return nil
}

func (m *mockForwardProxy) ResolveProxyInstance(ctx context.Context) error {
	if m.ResolveProxyInstanceFunc != nil {
		return m.ResolveProxyInstanceFunc(ctx)
	}
	return nil
}

func TestWithLongConnectionOption(t *testing.T) {
	idleCfg := connpool.IdleConfig{}
	options := []client.Option{
		WithLongConnection(idleCfg),
	}
	opts := client.NewOptions(options)
	test.Assert(t, opts.PoolCfg.MaxIdleTimeout == 30*time.Second) // defaultMaxIdleTimeout
	test.Assert(t, opts.PoolCfg.MaxIdlePerAddress == 1)           // default
	test.Assert(t, opts.PoolCfg.MaxIdleGlobal == 1)               //default
}

func TestWithWarmingUpOption(t *testing.T) {
	options := []client.Option{
		WithWarmingUp(&warmup.ClientOption{}),
	}
	_ = client.NewOptions(options)
}
