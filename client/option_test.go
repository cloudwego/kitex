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
	"testing"

	"github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
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
