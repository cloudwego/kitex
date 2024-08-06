package streamx

import (
	"context"
	"net"
)

/* Hot it works
- NewServer 时，初始化 ServerProvider，并注册 streamx.ServerTransHandler
- 连接进来的时候，detection trans handler 会转发调用 streamx.ServerTransHandler
- streamx.ServerTransHandler 负责调用 ServerProvider 的相关方法

后续读写都发生在 stream 提供的方法中
*/

type ServerProvider interface {
	Available(ctx context.Context, conn net.Conn) bool // ProtocolMath
	OnActive(ctx context.Context, conn net.Conn) (context.Context, error)
	OnStream(ctx context.Context, conn net.Conn) (context.Context, ServerStream, error)
	OnStreamFinish(ctx context.Context, ss ServerStream) (context.Context, error)
}
