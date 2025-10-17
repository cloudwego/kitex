//go:build linux
// +build linux

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

package shmipc

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/shmipc-go"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
)

var cliTransHdlr remote.ClientTransHandler

func TestMain(m *testing.M) {
	opt := &remote.ClientOption{
		Codec: &MockCodec{EncodeFunc: mockEncode, DecodeFunc: mockEncode},
	}
	cliF := NewCliTransHandlerFactory()
	cliTransHdlr, _ = cliF.NewTransHandler(opt)

	svr, errCh := initShmIPCServer()
	udsSvr, udsErrCh := initUDSServer()
	time.Sleep(1 * time.Second)
	select {
	case err := <-errCh:
		panic(err)
	case err := <-udsErrCh:
		panic(err)
	default:
	}
	m.Run()
	svr.Shutdown()
	udsSvr.Shutdown()
}

func testShmipcConfig(psm string, shmipcAddr net.Addr) *shmipc.SessionManagerConfig {
	c := shmipc.DefaultConfig()
	c.ShareMemoryBufferCap = uint32(32 << 20)
	sCfg := &shmipc.SessionManagerConfig{
		UnixPath:          shmUDS.String(),
		MaxStreamNum:      10000,
		StreamMaxIdleTime: 5 * time.Second,
		Config:            c,
		SessionNum:        1,
	}

	fmtShmPath := func(prefix string) string {
		return fmt.Sprintf("%s_%s_%d_client", prefix, "", uint64(time.Now().UnixNano())+rand.Uint64())
	}

	sCfg.ShareMemoryPathPrefix = fmtShmPath("shm.ipc")
	return sCfg
}

func TestWriteRead(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	shmUDS, _ := net.ResolveUnixAddr("unix", shmUDSAddrStr)

	cp := NewShmIPCPool(
		&Config{
			UnixPathBuilder: func(network, address string) string {
				return shmUDS.String()
			},
			SMConfigBuilder: func(network, address string) *shmipc.SessionManagerConfig {
				smc := testShmipcConfig("test", shmUDS)
				return smc
			},
		})

	conn, err := cp.Get(context.Background(), "", "", remote.ConnOption{
		Dialer: netpoll.NewDialer(),
	})
	test.Assert(t, err == nil, err)

	// the connection should be return by shmipc
	_, ok := conn.(*shmipc.Stream)
	test.Assert(t, ok == true)

	eInfo := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{ServiceName: mockServiceName, Method: mocks.MockMethod}, mocks.MockMethod)
	ri := rpcinfo.NewRPCInfo(eInfo, eInfo, rpcinfo.NewInvocation(mockServiceName, mocks.MockMethod), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())

	ctx := context.Background()
	var msg interface{}
	sendMsg := remote.NewMessage(msg, ri, remote.Call, remote.Client)
	ctx, err = cliTransHdlr.Write(ctx, conn, sendMsg)
	test.Assert(t, err == nil, err)

	recvMsg := remote.NewMessage(msg, ri, remote.Reply, remote.Client)
	ctx, err = cliTransHdlr.Read(ctx, conn, recvMsg)
	test.Assert(t, err == nil, err)

	cp.Close()
}
