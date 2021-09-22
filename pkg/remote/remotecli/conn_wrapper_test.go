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

package remotecli

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	connpool2 "github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/connpool"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

var (
	poolCfg = connpool2.IdleConfig{MaxIdlePerAddress: 100, MaxIdleGlobal: 100, MaxIdleTimeout: time.Second}
	logger  = klog.DefaultLogger()
)

func TestDialerMWNoAddr(t *testing.T) {
	dialer := &remote.SynthesizedDialer{}
	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
	conf := rpcinfo.NewRPCConfig()
	ri := rpcinfo.NewRPCInfo(nil, to, rpcinfo.NewInvocation("", ""), conf, rpcinfo.NewRPCStats())
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

	connW := NewConnWrapper(connpool.NewLongPool("psm", poolCfg), logger)
	_, err := connW.GetConn(ctx, dialer, ri)
	test.Assert(t, err != nil)
	test.Assert(t, errors.Is(err, kerrors.ErrNoDestAddress))
}

func TestGetConnDial(t *testing.T) {
	addr := utils.NewNetAddr("tcp", "to")
	conn := &mocks.Conn{}
	dialer := &remote.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			test.Assert(t, network == addr.Network())
			test.Assert(t, address == addr.String())
			return conn, nil
		},
	}
	from := rpcinfo.NewEndpointInfo("from", "method", nil, nil)
	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
	to.SetInstance(discovery.NewInstance(addr.Network(), addr.String(), discovery.DefaultWeight, nil))
	conf := rpcinfo.NewRPCConfig()
	ri := rpcinfo.NewRPCInfo(from, to, rpcinfo.NewInvocation("", ""), conf, rpcinfo.NewRPCStats())

	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	connW := NewConnWrapper(nil, logger)
	conn2, err := connW.GetConn(ctx, dialer, ri)
	test.Assert(t, err == nil, err)
	test.Assert(t, conn == conn2)
}

func TestGetConnByPool(t *testing.T) {
	addr := utils.NewNetAddr("tcp", "to")
	conn := &mocks.Conn{
		RemoteAddrFunc: func() (r net.Addr) {
			return addr
		},
	}
	dialCount := 0
	dialer := &remote.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			test.Assert(t, network == addr.Network())
			test.Assert(t, address == addr.String())
			dialCount++
			return conn, nil
		},
	}
	from := rpcinfo.NewEndpointInfo("from", "method", nil, nil)
	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
	to.SetInstance(discovery.NewInstance(addr.Network(), addr.String(), discovery.DefaultWeight, nil))

	conf := rpcinfo.NewRPCConfig()
	ri := rpcinfo.NewRPCInfo(from, to, rpcinfo.NewInvocation("", ""), conf, rpcinfo.NewRPCStats())
	connPool := connpool.NewLongPool("psm", poolCfg)
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	// 释放连接, 连接复用
	for i := 0; i < 10; i++ {
		connW := NewConnWrapper(connPool, logger)
		conn2, err := connW.GetConn(ctx, dialer, ri)
		test.Assert(t, err == nil, err)
		test.Assert(t, conn == conn2)
		connW.ReleaseConn(nil, ri)

	}
	test.Assert(t, dialCount == 1)

	dialCount = 0
	connPool.Clean(addr.Network(), addr.String())
	// 未释放连接, 连接重建
	for i := 0; i < 10; i++ {
		connW := NewConnWrapper(connPool, logger)
		conn2, err := connW.GetConn(ctx, dialer, ri)
		test.Assert(t, err == nil, err)
		test.Assert(t, conn == conn2)
	}
	test.Assert(t, dialCount == 10)
}

func BenchmarkGetConn(b *testing.B) {
	addr := utils.NewNetAddr("tcp", "to")
	conn := &mocks.Conn{
		RemoteAddrFunc: func() (r net.Addr) {
			return addr
		},
	}
	shortConnDialCount := 0
	shortConnDialer := &remote.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			shortConnDialCount++
			return conn, nil
		},
	}
	longConnDialCount := 0
	longConnDialer := &remote.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			longConnDialCount++
			return conn, nil
		},
	}

	from := rpcinfo.NewEndpointInfo("from", "method", nil, nil)
	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
	to.SetInstance(discovery.NewInstance(addr.Network(), addr.String(), discovery.DefaultWeight, nil))

	conf := rpcinfo.NewRPCConfig()
	ri := rpcinfo.NewRPCInfo(from, to, rpcinfo.NewInvocation("", ""), conf, rpcinfo.NewRPCStats())
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	connPool := connpool.NewLongPool("psm", poolCfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connW := NewConnWrapper(connPool, logger)
		conn2, err := connW.GetConn(ctx, longConnDialer, ri)
		test.Assert(b, err == nil, err)
		test.Assert(b, conn == conn2)
		connW.ReleaseConn(nil, ri)

		connW2 := NewConnWrapper(nil, logger)
		conn2, err = connW2.GetConn(ctx, shortConnDialer, ri)
		test.Assert(b, err == nil, err)
		test.Assert(b, conn == conn2)
		connW.ReleaseConn(nil, ri)
	}
	test.Assert(b, shortConnDialCount == b.N)
	test.Assert(b, longConnDialCount == 1)
}
