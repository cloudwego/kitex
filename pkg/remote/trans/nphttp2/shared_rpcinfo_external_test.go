/*
 * Copyright 2026 CloudWeGo Authors
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

package nphttp2_test

import (
	"errors"
	"net"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/cloudwego/kitex/internal/mocks"
	mockremote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
)

func TestSharedRPCInfoIsCleanAtStreamStart(t *testing.T) {
	nphttp2.CheckPooledRPCInfoAtStreamStart(t, serverRPCInfoReset(t))
}

func TestSharedRPCInfoHandoffPreservesActiveRequest(t *testing.T) {
	nphttp2.CheckSharedRPCInfoHandoff(t, serverRPCInfoReset(t))
}

func serverRPCInfoReset(t *testing.T) func(rpcinfo.RPCInfo, net.Addr) rpcinfo.RPCInfo {
	t.Helper()
	var reset func(rpcinfo.RPCInfo, net.Addr) rpcinfo.RPCInfo
	factory := mockremote.NewMockServerTransHandlerFactory(gomock.NewController(t))
	stop := errors.New("server options captured before opening a listener")
	factory.EXPECT().NewTransHandler(gomock.Any()).DoAndReturn(func(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
		reset = opt.InitOrResetRPCInfoFunc
		return nil, stop
	})
	svr := server.NewServer(server.WithTransHandlerFactory(factory))
	if err := svr.RegisterService(mocks.ServiceInfo(), mocks.MyServiceHandler()); err != nil {
		t.Fatal(err)
	}
	if err := svr.Run(); err != stop {
		t.Fatalf("server initialization returned %v, want %v", err, stop)
	}
	return reset
}
