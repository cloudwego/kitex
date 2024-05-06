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

	"github.com/bytedance/sonic"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

type netpollReader interface {
	Reader() netpoll.Reader
}

func getReader(conn net.Conn) netpoll.Reader {
	return conn.(netpollReader).Reader()
}

// coverBizStatusError saves the BizStatusErr (if any) into the RPCInfo and returns nil since it's not an RPC error
// server.invokeHandleEndpoint already processed BizStatusError, so it's only used by unknownMethodHandler
func coverBizStatusError(err error, ri rpcinfo.RPCInfo) error {
	var bizErr kerrors.BizStatusErrorIface
	if errors.As(err, &bizErr) {
		err = nil
		if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
			setter.SetBizStatusErr(bizErr)
		}
	}
	return err
}

// getRemoteAddr returns the remote address of the connection.
func getRemoteAddr(ctx context.Context, conn net.Conn) net.Addr {
	addr := conn.RemoteAddr()
	if addr != nil && addr.Network() != "unix" {
		// unix socket: local sidecar proxy
		return addr
	}
	// likely from service mesh: return the addr read from mesh ttheader
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil {
		return addr
	}
	if ri.From().Address() != nil {
		return ri.From().Address()
	}
	return addr
}

// injectGRPCMetadata parses grpc style metadata into ctx, helpful for projects migrating from kitex-grpc
func injectGRPCMetadata(ctx context.Context, strInfo map[string]string) (context.Context, error) {
	value, exists := strInfo[codec.StrKeyMetaData]
	if !exists {
		return ctx, nil
	}
	md := metadata.MD{}
	if err := sonic.Unmarshal([]byte(value), &md); err != nil {
		return ctx, err
	}
	return metadata.NewIncomingContext(ctx, md), nil
}
