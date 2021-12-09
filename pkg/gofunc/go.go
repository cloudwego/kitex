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

package gofunc

import (
	"context"
	"net"
	"runtime/debug"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// GoTask is used to spawn a new task.
type GoTask func(context.Context, func())

// GoFunc is the default func used globally.
var GoFunc GoTask

func init() {
	GoFunc = func(ctx context.Context, f func()) {
		gopool.CtxGo(ctx, f)
	}
}

// GoFuncWithRecover is the go func with recover panic.
func GoFuncWithRecover(ctx context.Context, conn net.Conn, panicMsg string, handler func()) {
	GoFunc(ctx, func() {
		defer func() {
			if r := recover(); r != nil {
				rService, rAddr := getRemoteInfo(rpcinfo.GetRPCInfo(ctx), conn)
				klog.CtxErrorf(ctx, "%s, "+
					"remoteService=%s, remoteAddress=%s, recover=%v\nstack=%s", panicMsg, rService, rAddr.String(), r, string(debug.Stack()))
			}
		}()

		handler()
	})
}

func getRemoteInfo(ri rpcinfo.RPCInfo, conn net.Conn) (remoteService string, remoteAddr net.Addr) {
	rAddr := conn.RemoteAddr()
	if ri == nil || ri.From() == nil {
		return "", rAddr
	}

	from := ri.From()
	if rAddr != nil && rAddr.Network() == "unix" {
		if from.Address() != nil {
			rAddr = from.Address()
		}
	}
	return from.ServiceName(), rAddr
}
