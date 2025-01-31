/*
 * Copyright 2022 CloudWeGo Authors
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

package gonet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	internalRemote "github.com/cloudwego/kitex/internal/remote"

	"github.com/cloudwego/kitex/internal/mocks"
	mocksremote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

var (
	svrTransHdlr remote.ServerTransHandler
	rwTimeout    = time.Second
	addrStr      = "test addr"
	addr         = utils.NewNetAddr("tcp", addrStr)
	method       = "mock"
	transSvr     *transServer
	svrOpt       *remote.ServerOption
)

type MockGonetConn struct {
	mocks.Conn
	SetReadTimeoutFunc func(timeout time.Duration) error
}

func (m *MockGonetConn) Do() bool {
	// TODO implement me
	panic("implement me")
}

func TestMain(m *testing.M) {
	svcInfo := mocks.ServiceInfo()
	svrOpt = &remote.ServerOption{
		InitOrResetRPCInfoFunc: func(ri rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
			fromInfo := rpcinfo.EmptyEndpointInfo()
			rpcCfg := rpcinfo.NewRPCConfig()
			mCfg := rpcinfo.AsMutableRPCConfig(rpcCfg)
			mCfg.SetReadWriteTimeout(rwTimeout)
			ink := rpcinfo.NewInvocation("", method)
			rpcStat := rpcinfo.NewRPCStats()
			nri := rpcinfo.NewRPCInfo(fromInfo, nil, ink, rpcCfg, rpcStat)
			rpcinfo.AsMutableEndpointInfo(nri.From()).SetAddress(addr)
			return nri
		},
		Codec: &MockCodec{
			EncodeFunc: nil,
			DecodeFunc: func(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
				msg.SpecifyServiceInfo(mocks.MockServiceName, mocks.MockMethod)
				return nil
			},
		},
		SvcSearcher:   mocksremote.NewDefaultSvcSearcher(),
		TargetSvcInfo: svcInfo,
		TracerCtl:     &rpcinfo.TraceController{},
	}
	svrTransHdlr, _ = newSvrTransHandler(svrOpt)
	transSvr = NewTransServerFactory().NewTransServer(svrOpt, remote.NewTransPipeline(svrTransHdlr)).(*transServer)
	os.Exit(m.Run())
}

// TestCreateListener test trans_server CreateListener success
func TestCreateListener(t *testing.T) {
	// tcp init
	addrStr := "127.0.0.1:9093"
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

func TestBootStrapAndShutdown(t *testing.T) {
	// tcp init
	addrStr := "127.0.0.1:9093"
	addr = utils.NewNetAddr("tcp", addrStr)
	ln, err := transSvr.CreateListener(addr)
	test.Assert(t, err == nil, err)
	test.Assert(t, ln.Addr().String() == addrStr)
	exitOnRead := make(chan struct{})
	transSvr.transHdlr = &mocks.MockSvrTransHandler{
		OnActiveFunc: func(ctx context.Context, conn net.Conn) (context.Context, error) {
			ri := svrOpt.InitOrResetRPCInfoFunc(nil, conn.RemoteAddr())
			return rpcinfo.NewCtxWithRPCInfo(ctx, ri), nil
		},
		OnReadFunc: func(ctx context.Context, conn net.Conn) error {
			<-exitOnRead
			return errors.New("mock read error")
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		be := transSvr.BootstrapServer(ln)
		test.Assert(t, be == nil, be)
		wg.Done()
	}()
	// wait server starts
	time.Sleep(100 * time.Millisecond)
	test.Assert(t, transSvr.connCount.Value() == 0)

	// new conn
	c, err := net.Dial("tcp", addrStr)
	test.Assert(t, err == nil, err)
	c.Write([]byte("test")) // trigger onRead
	time.Sleep(10 * time.Millisecond)
	test.Assert(t, transSvr.connCount.Value() == 1)

	// shutdown
	// case: ctx done, since connections still in processing
	transSvr.Lock()
	transSvr.opt.ExitWaitTime = 10 * time.Millisecond
	transSvr.Unlock()
	err = transSvr.Shutdown() // context
	test.Assert(t, errors.Is(err, context.DeadlineExceeded), err)
	// BootstrapServer will return if listener closed
	wg.Wait()
	test.Assert(t, transSvr.connCount.Value() == 1)

	// case: normal
	close(exitOnRead)
	transSvr.Lock()
	transSvr.opt.ExitWaitTime = time.Second
	transSvr.Unlock()
	time.Sleep(10 * time.Millisecond)
	err = transSvr.Shutdown()
	test.Assert(t, err == nil, err)
	test.Assert(t, transSvr.connCount.Value() == 0)
}

func TestServeConn(t *testing.T) {
	isClosed := false
	mockConn := &MockGonetConn{
		Conn: mocks.Conn{
			RemoteAddrFunc: func() net.Addr {
				return addr
			},
			CloseFunc: func() (e error) {
				isClosed = true
				return nil
			},
		},
	}
	// OnActive
	connCnt := 0
	expectedErr := fmt.Errorf("OnActiveError")
	transSvr.transHdlr = &mocks.MockSvrTransHandler{
		OnActiveFunc: func(ctx context.Context, conn net.Conn) (context.Context, error) {
			connCnt = transSvr.connCount.Value()
			return nil, expectedErr
		},
		Opt: transSvr.opt,
	}
	err := transSvr.serveConn(context.Background(), mockConn)
	test.Assert(t, connCnt == 1)
	test.Assert(t, err == expectedErr)
	test.Assert(t, isClosed)
	test.Assert(t, transSvr.connCount.Value() == 0)
	isClosed = false

	// Peek error
	transSvr.transHdlr = &mocks.MockSvrTransHandler{
		OnActiveFunc: svrTransHdlr.OnActive,
		Opt:          transSvr.opt,
	}
	expectedErr = io.EOF
	mockConn.ReadFunc = func(b []byte) (n int, err error) {
		return 0, io.EOF
	}
	err = transSvr.serveConn(context.Background(), mockConn)
	test.Assert(t, err == expectedErr)
	test.Assert(t, isClosed)
	isClosed = false
	mockConn.ReadFunc = nil

	// OnRead Error
	expectedErr = fmt.Errorf("OnReadError")
	transSvr.transHdlr = &mocks.MockSvrTransHandler{
		OnActiveFunc: svrTransHdlr.OnActive,
		OnReadFunc: func(ctx context.Context, conn net.Conn) error {
			return expectedErr
		},
		Opt: transSvr.opt,
	}
	err = transSvr.serveConn(context.Background(), mockConn)
	test.Assert(t, err == expectedErr)
	test.Assert(t, isClosed)
	isClosed = false

	// OnRead, once
	cnt := 0
	transSvr.transHdlr = &mocks.MockSvrTransHandler{
		OnActiveFunc: svrTransHdlr.OnActive,
		OnReadFunc: func(ctx context.Context, conn net.Conn) error {
			if e, ok := conn.(internalRemote.OnceExecutor); ok {
				if !e.Do() {
					return nil
				}
			}
			cnt++
			return nil
		},
		Opt: transSvr.opt,
	}
	err = transSvr.serveConn(context.Background(), mockConn)
	test.Assert(t, err == nil, err)
	test.Assert(t, cnt == 1)

	// Panic, recovered
	transSvr.transHdlr = &mocks.MockSvrTransHandler{
		Opt: transSvr.opt,
		OnReadFunc: func(ctx context.Context, conn net.Conn) error {
			panic("on read")
		},
	}
	mockConn.ReadFunc = func(b []byte) (n int, err error) {
		panic("xxx panic read")
	}
	transSvr.serveConn(context.Background(), mockConn)
	test.Assert(t, isClosed)
}
