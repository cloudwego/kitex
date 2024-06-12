/*
 * Copyright 2024 CloudWeGo Authors
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

package ttheaderstreaming

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func Test_coverBizStatusError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var err error
		ivk := rpcinfo.NewInvocation("svc", "method")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, nil)

		got := coverBizStatusError(err, ri)

		test.Assert(t, got == nil)
		test.Assert(t, ri.Invocation().BizStatusErr() == nil)
	})
	t.Run("not-biz-status-error", func(t *testing.T) {
		err := errors.New("XXX")
		ivk := rpcinfo.NewInvocation("svc", "method")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, nil)

		got := coverBizStatusError(err, ri)

		test.Assert(t, errors.Is(got, err))
		test.Assert(t, ri.Invocation().BizStatusErr() == nil)
	})
	t.Run("biz-status-err", func(t *testing.T) {
		err := kerrors.NewBizStatusError(1, "XXX")
		ivk := rpcinfo.NewInvocation("svc", "method")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, nil)

		got := coverBizStatusError(err, ri)

		test.Assert(t, got == nil)
		test.Assert(t, ri.Invocation().BizStatusErr() != nil)
	})
}

func Test_getRemoteAddr(t *testing.T) {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8888")
	unixAddr, _ := net.ResolveUnixAddr("unix", "demo.sock")
	t.Run("tcp-addr", func(t *testing.T) {
		conn := &mockConn{remoteAddr: tcpAddr}
		got := getRemoteAddr(context.Background(), conn)
		test.Assert(t, got.String() == "127.0.0.1:8888")
	})
	t.Run("unix-addr-without-rpcinfo", func(t *testing.T) {
		conn := &mockConn{remoteAddr: unixAddr}
		got := getRemoteAddr(context.Background(), conn)
		test.Assert(t, got == unixAddr)
	})
	t.Run("unix-addr-without-rpcinfo-from-addr", func(t *testing.T) {
		conn := &mockConn{remoteAddr: unixAddr}
		from := rpcinfo.NewEndpointInfo("svc", "method", nil, nil)
		ri := rpcinfo.NewRPCInfo(from, nil, nil, nil, nil)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		got := getRemoteAddr(ctx, conn)
		test.Assert(t, got == unixAddr)
	})
	t.Run("unix-addr-with-rpcinfo-from-addr", func(t *testing.T) {
		conn := &mockConn{remoteAddr: unixAddr}
		from := rpcinfo.NewEndpointInfo("svc", "method", tcpAddr, nil)
		ri := rpcinfo.NewRPCInfo(from, nil, nil, nil, nil)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		got := getRemoteAddr(ctx, conn)
		test.Assert(t, got == tcpAddr)
	})
}

func Test_injectGRPCMetadata(t *testing.T) {
	t.Run("no-grpc-metadata", func(t *testing.T) {
		ctx := context.Background()
		got, err := injectGRPCMetadata(ctx, map[string]string{})
		test.Assert(t, err == nil)
		test.Assert(t, got == ctx)
	})

	t.Run("grpc-metadata", func(t *testing.T) {
		ctx := context.Background()
		strInfo := map[string]string{
			codec.StrKeyMetaData: `{"k1":["v1","v2"],"k2":["v3","v4"]}`,
		}
		got, err := injectGRPCMetadata(ctx, strInfo)
		test.Assert(t, err == nil)
		test.Assert(t, got != ctx)
		md, ok := metadata.FromIncomingContext(got)
		test.Assert(t, ok)
		test.Assert(t, md["k1"][0] == "v1")
		test.Assert(t, md["k1"][1] == "v2")
		test.Assert(t, md["k2"][0] == "v3")
		test.Assert(t, md["k2"][1] == "v4")
	})

	t.Run("invalid-grpc-metadata", func(t *testing.T) {
		ctx := context.Background()
		strInfo := map[string]string{
			codec.StrKeyMetaData: `invalid`,
		}
		_, err := injectGRPCMetadata(ctx, strInfo)
		test.Assert(t, err != nil)
	})
}
