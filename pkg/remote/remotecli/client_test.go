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

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/connpool"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/pkg/utils"
)

// TestNewClientNoAddr test new client return err because no addr
func TestNewClientNoAddr(t *testing.T) {
	// 1. prepare mock data
	ctx := context.Background()

	dialer := &remote.SynthesizedDialer{}
	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
	conf := rpcinfo.NewRPCConfig()
	ri := rpcinfo.NewRPCInfo(nil, to, rpcinfo.NewInvocation("", ""), conf, rpcinfo.NewRPCStats())

	hdlr := &mocks.MockCliTransHandler{}
	connPool := connpool.NewLongPool("destService", poolCfg)
	opt := &remote.ClientOption{
		ConnPool: connPool,
		Dialer:   dialer,
	}

	// 2. test
	_, err := NewClient(ctx, ri, hdlr, opt)
	test.Assert(t, err != nil, err)
	test.Assert(t, errors.Is(err, kerrors.ErrGetConnection))
}

// TestNewClient test new a client
func TestNewClient(t *testing.T) {
	addr := utils.NewNetAddr("tcp", "to")
	ri := newMockRPCInfo(addr)
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

	hdlr := &mocks.MockCliTransHandler{}

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

	connPool := connpool.NewLongPool("destService", poolCfg)

	opt := &remote.ClientOption{
		ConnPool: connPool,
		Dialer:   dialer,
		Option: remote.Option{
			StreamingMetaHandlers: []remote.StreamingMetaHandler{
				transmeta.ClientHTTP2Handler,
			},
		},
	}

	cli, err := NewClient(ctx, ri, hdlr, opt)
	test.Assert(t, err == nil, err)
	test.Assert(t, cli != nil, cli)
	test.Assert(t, dialCount == 1)
}

func newMockRPCInfo(addr net.Addr) rpcinfo.RPCInfo {
	from := rpcinfo.NewEndpointInfo("from", "method", nil, nil)
	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
	to.SetInstance(discovery.NewInstance(addr.Network(), addr.String(), discovery.DefaultWeight, nil))
	conf := rpcinfo.NewRPCConfig()
	ri := rpcinfo.NewRPCInfo(from, to, rpcinfo.NewInvocation("", ""), conf, rpcinfo.NewRPCStats())

	return ri
}

func newMockOption(addr net.Addr) *remote.ClientOption {
	conn := &mocks.Conn{
		RemoteAddrFunc: func() (r net.Addr) {
			return addr
		},
	}
	dialer := &remote.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			return conn, nil
		},
	}

	connPool := connpool.NewLongPool("destService", poolCfg)

	opt := &remote.ClientOption{
		ConnPool: connPool,
		Dialer:   dialer,
	}

	return opt
}

// TestSend test send msg
func TestSend(t *testing.T) {
	// 1. prepare mock data
	ctx := context.Background()
	addr := utils.NewNetAddr("tcp", "to")
	ri := newMockRPCInfo(addr)

	isWrite := false
	var writeMsg remote.Message
	hdlr := &mocks.MockCliTransHandler{
		WriteFunc: func(ctx context.Context, conn net.Conn, send remote.Message) error {
			isWrite = true
			writeMsg = send
			return nil
		},
		ReadFunc: nil,
	}

	opt := newMockOption(addr)

	cli, err := NewClient(ctx, ri, hdlr, opt)
	test.Assert(t, err == nil, err)
	test.Assert(t, cli != nil, cli)

	var sendMsg remote.Message

	// 2. test
	err = cli.Send(ctx, ri, sendMsg)
	test.Assert(t, err == nil, err)
	test.Assert(t, isWrite)
	test.Assert(t, writeMsg == sendMsg)
}

// TestSendErr test send return err because write fail
func TestSendErr(t *testing.T) {
	// 1. prepare mock data
	addr := utils.NewNetAddr("tcp", "to")
	ri := newMockRPCInfo(addr)
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

	isWrite := false
	var writeMsg remote.Message
	errMsg := "mock test send err"
	hdlr := &mocks.MockCliTransHandler{
		WriteFunc: func(ctx context.Context, conn net.Conn, send remote.Message) error {
			isWrite = true
			writeMsg = send
			return errors.New(errMsg)
		},
		ReadFunc: nil,
	}

	opt := newMockOption(addr)

	isClose := false
	conn := &mocks.Conn{
		RemoteAddrFunc: func() (r net.Addr) {
			return addr
		},
		CloseFunc: func() (err error) {
			isClose = true
			return err
		},
	}
	opt.Dialer = &remote.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			return conn, nil
		},
	}

	// 2. test
	cli, err := NewClient(ctx, ri, hdlr, opt)
	test.Assert(t, err == nil, err)
	test.Assert(t, cli != nil, cli)

	var sendMsg remote.Message
	err = cli.Send(ctx, ri, sendMsg)
	test.Assert(t, err != nil, err)
	test.Assert(t, err.Error() == errMsg)
	test.Assert(t, isWrite)
	test.Assert(t, writeMsg == sendMsg)
	test.Assert(t, isClose)
}

// TestRecv test recv msg
func TestRecv(t *testing.T) {
	ctx := context.Background()

	addr := utils.NewNetAddr("tcp", "to")
	ri := newMockRPCInfo(addr)

	svcInfo := mocks.ServiceInfo()
	var resp interface{}
	msg := remote.NewMessage(resp, svcInfo, ri, remote.Call, remote.Client)

	var readMsg remote.Message
	isRead := false
	hdlr := &mocks.MockCliTransHandler{
		WriteFunc: nil,
		ReadFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) error {
			isRead = true
			readMsg = msg
			return nil
		},
	}

	opt := newMockOption(addr)

	cli, err := NewClient(ctx, ri, hdlr, opt)
	test.Assert(t, err == nil, err)
	test.Assert(t, cli != nil, cli)

	err = cli.Recv(ctx, ri, msg)
	test.Assert(t, err == nil, err)
	test.Assert(t, isRead)
	test.Assert(t, readMsg == msg)
}

// TestRecvOneWay test recv onw way msg
func TestRecvOneWay(t *testing.T) {
	ctx := context.Background()

	addr := utils.NewNetAddr("tcp", "to")
	ri := newMockRPCInfo(addr)

	hdlr := &mocks.MockCliTransHandler{
		WriteFunc: nil,
		ReadFunc:  nil,
	}

	opt := newMockOption(addr)
	opt.ConnPool = nil

	isClose := false
	conn := &mocks.Conn{
		RemoteAddrFunc: func() (r net.Addr) {
			return addr
		},
		CloseFunc: func() (err error) {
			isClose = true
			return err
		},
	}
	opt.Dialer = &remote.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			return conn, nil
		},
	}

	cli, err := NewClient(ctx, ri, hdlr, opt)
	test.Assert(t, err == nil, err)
	test.Assert(t, cli != nil, cli)

	err = cli.Recv(ctx, ri, nil)
	test.Assert(t, err == nil, err)
	test.Assert(t, isClose)
}

// TestRecycle test recycle
func TestRecycle(t *testing.T) {
	ctx := context.Background()

	addr := utils.NewNetAddr("tcp", "to")
	ri := newMockRPCInfo(addr)

	hdlr := &mocks.MockCliTransHandler{
		WriteFunc: nil,
		ReadFunc:  nil,
	}

	opt := newMockOption(addr)

	cli, err := NewClient(ctx, ri, hdlr, opt)
	test.Assert(t, err == nil, err)
	test.Assert(t, cli != nil, cli)

	cli.Recycle()
	poolCli := clientPool.Get()
	test.Assert(t, cli == poolCli)
}
