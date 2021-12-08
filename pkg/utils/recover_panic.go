package utils

import (
	"context"
	"net"
	"runtime/debug"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func GoWithRecover(ctx context.Context, conn net.Conn, errMsg string, handler func()) {
	rService, rAddr := getRemoteInfo(rpcinfo.GetRPCInfo(ctx), conn)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				klog.CtxErrorf(ctx, "%s, "+
					"remoteAddress=%s, remoteService=%s, recover=%v\nstack=%s", errMsg, rAddr.String(), rService, r, string(debug.Stack()))
			}
		}()

		handler()
	}()
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
