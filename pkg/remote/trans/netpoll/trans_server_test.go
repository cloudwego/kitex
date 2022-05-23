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

package netpoll

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/mocks"
	internal_stats "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

var transSvr *transServer

func init() {
	fromInfo := rpcinfo.EmptyEndpointInfo()
	rpcCfg := rpcinfo.NewRPCConfig()
	mCfg := rpcinfo.AsMutableRPCConfig(rpcCfg)
	mCfg.SetReadWriteTimeout(rwTimeout)
	ink := rpcinfo.NewInvocation("", method)
	rpcStat := rpcinfo.NewRPCStats()

	opt := &remote.ServerOption{
		InitRPCInfoFunc: func(ctx context.Context, addr net.Addr) (rpcinfo.RPCInfo, context.Context) {
			ri := rpcinfo.NewRPCInfo(fromInfo, nil, ink, rpcCfg, rpcStat)
			rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(addr)
			ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
			return ri, ctx
		},
		Codec: &MockCodec{
			EncodeFunc: nil,
			DecodeFunc: nil,
		},
		SvcInfo:   mocks.ServiceInfo(),
		TracerCtl: &internal_stats.Controller{},
	}
	svrTransHdlr, _ = newSvrTransHandler(opt)

	transSvr = NewTransServerFactory().NewTransServer(opt, svrTransHdlr).(*transServer)
}

// TestCreateListener test trans_server CreateListener success
func TestCreateListener(t *testing.T) {
	// tcp init
	addrStr := "127.0.0.1:9090"
	addr = utils.NewNetAddr("tcp", addrStr)

	// test
	ln, err := transSvr.CreateListener(addr)
	test.Assert(t, err == nil, err)
	test.Assert(t, ln.Addr().String() == addrStr)
	ln.Close()

	// uds init
	addrStr = "server.addr"
	addr, err = net.ResolveUnixAddr("unix", addrStr)
	test.Assert(t, err == nil, err)

	// test
	ln, err = transSvr.CreateListener(addr)
	test.Assert(t, err == nil, err)
	test.Assert(t, ln.Addr().String() == addrStr)
	ln.Close()
}

// TestBootStrap test trans_server BootstrapServer success
func TestBootStrap(t *testing.T) {
	// tcp init
	addrStr := "127.0.0.1:9090"
	addr = utils.NewNetAddr("tcp", addrStr)

	// test
	ln, err := transSvr.CreateListener(addr)
	test.Assert(t, err == nil, err)
	test.Assert(t, ln.Addr().String() == addrStr)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = transSvr.BootstrapServer()
		test.Assert(t, err == nil, err)
		wg.Done()
	}()
	time.Sleep(10 * time.Millisecond)

	transSvr.Shutdown()
	wg.Wait()
}

// TestOnConnActive test trans_server onConnActive success
func TestOnConnActive(t *testing.T) {
	// 1. prepare mock data
	conn := &MockNetpollConn{
		SetReadTimeoutFunc: func(timeout time.Duration) (e error) {
			return nil
		},
		Conn: mocks.Conn{
			RemoteAddrFunc: func() (r net.Addr) {
				return addr
			},
		},
	}

	// 2. test
	connCount := 100
	for i := 0; i < connCount; i++ {
		transSvr.onConnActive(conn)
	}
	ctx := context.Background()

	currConnCount := transSvr.ConnCount()
	test.Assert(t, currConnCount.Value() == connCount)

	for i := 0; i < connCount; i++ {
		transSvr.onConnInactive(ctx, conn)
	}

	currConnCount = transSvr.ConnCount()
	test.Assert(t, currConnCount.Value() == 0)
}

// TestOnConnRead test trans_server onConnRead success
func TestOnConnRead(t *testing.T) {
	// 1. prepare mock data
	conn := &MockNetpollConn{
		Conn: mocks.Conn{
			RemoteAddrFunc: func() (r net.Addr) {
				return addr
			},
		},
	}
	mockErr := errors.New("mock error")
	transSvr.transHdlr = &mocks.MockSvrTransHandler{
		OnReadFunc: func(ctx context.Context, conn net.Conn) error {
			return mockErr
		},
		Opt: transSvr.opt,
	}

	// 2. test
	err := transSvr.onConnRead(context.Background(), conn)
	test.Assert(t, err == nil, err)
}
