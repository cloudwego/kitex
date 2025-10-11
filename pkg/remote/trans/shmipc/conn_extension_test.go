/*
 * Copyright 2021 CloudWeGo authors.
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
	"reflect"
	"syscall"
	"testing"

	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/shmipc-go"
	"github.com/golang/mock/gomock"

	mocksnetpoll "github.com/cloudwego/kitex/internal/mocks/netpoll"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	knetpoll "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
)

func TestNewReadByteBuffer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ext := newShmIPCExtension()
	buf := ext.NewReadByteBuffer(context.Background(), &shmipc.Stream{}, &MockMessage{})
	test.Assert(t, reflect.TypeOf(buf) == reflect.TypeOf(&shmIPCByteBuffer{}))
	conn := mocksnetpoll.NewMockConnection(ctrl)
	conn.EXPECT().Reader().Return(nil).AnyTimes()
	buf = ext.NewReadByteBuffer(context.Background(), conn, &MockMessage{})
	test.Assert(t, reflect.TypeOf(buf) == reflect.TypeOf(knetpoll.NewReaderByteBuffer(&MockNetpollReader{})))
}

func TestNewWriteByteBuffer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	from := rpcinfo.NewEndpointInfo("", "", nil, make(map[string]string))
	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "mock")
	ri := rpcinfo.NewRPCInfo(from, to, rpcinfo.NewInvocation("", ""), nil, nil)

	ext := newShmIPCExtension()
	cliMsg := &MockMessage{RPCRoleFunc: func() remote.RPCRole {
		return remote.Client
	}, RPCInfoFunc: func() rpcinfo.RPCInfo {
		return ri
	}}
	buf := ext.NewWriteByteBuffer(context.Background(), &shmipc.Stream{}, cliMsg)
	test.Assert(t, reflect.TypeOf(buf) == reflect.TypeOf(&shmIPCByteBuffer{}))
	val, _ := cliMsg.RPCInfo().To().Tag(rpcinfo.ShmIPCTag)
	test.Assert(t, val == shmIPCOn)

	conn := mocksnetpoll.NewMockConnection(ctrl)
	conn.EXPECT().Writer().AnyTimes()
	buf = ext.NewWriteByteBuffer(context.Background(), conn, cliMsg)
	test.Assert(t, reflect.TypeOf(buf) == reflect.TypeOf(knetpoll.NewWriterByteBuffer(&MockNetpollWriter{})))
	val, _ = cliMsg.RPCInfo().To().Tag(rpcinfo.ShmIPCTag)
	test.Assert(t, val == shmIPCOff)

	svrMsg := &MockMessage{RPCRoleFunc: func() remote.RPCRole {
		return remote.Server
	}, RPCInfoFunc: func() rpcinfo.RPCInfo {
		return ri
	}}
	buf = ext.NewWriteByteBuffer(context.Background(), &shmipc.Stream{}, svrMsg)
	test.Assert(t, reflect.TypeOf(buf) == reflect.TypeOf(&shmIPCByteBuffer{}))
	val, _ = cliMsg.RPCInfo().From().Tag(rpcinfo.ShmIPCTag)
	test.Assert(t, val == shmIPCOn)

	buf = ext.NewWriteByteBuffer(context.Background(), conn, svrMsg)
	test.Assert(t, reflect.TypeOf(buf) == reflect.TypeOf(knetpoll.NewWriterByteBuffer(&MockNetpollWriter{})))
	val, _ = cliMsg.RPCInfo().From().Tag(rpcinfo.ShmIPCTag)
	test.Assert(t, val == shmIPCOff)
}

func TestIsTimeoutErr(t *testing.T) {
	ext := newShmIPCExtension()
	test.Assert(t, !ext.IsTimeoutErr(nil))
	test.Assert(t, ext.IsTimeoutErr(fmt.Errorf("%w", shmipc.ErrTimeout)))
	test.Assert(t, ext.IsTimeoutErr(fmt.Errorf("%w", netpoll.ErrReadTimeout)))
}

func TestIsRemoteClosedErr(t *testing.T) {
	ext := newShmIPCExtension()
	test.Assert(t, !ext.IsRemoteClosedErr(nil))
	test.Assert(t, ext.IsRemoteClosedErr(fmt.Errorf("%w", shmipc.ErrStreamClosed)))
	test.Assert(t, ext.IsRemoteClosedErr(fmt.Errorf("%w", netpoll.ErrConnClosed)))
	test.Assert(t, ext.IsRemoteClosedErr(fmt.Errorf("%w", syscall.EPIPE)))
}
