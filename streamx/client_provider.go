package streamx

import (
	"context"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

/* Hot it works
- NewClient 时，初始化 ClientProvider
- 生成代码调用 client.NewStream() 方法
- client.NewStream => clientProvider.NewStream
- client 执行中间件逻辑

后续读写都发生在 stream 提供的方法中
*/

type ClientProvider interface {
	NewStream(ctx context.Context, ri rpcinfo.RPCInfo, callOptions ...streamxcallopt.CallOption) (ClientStream, error)
}
