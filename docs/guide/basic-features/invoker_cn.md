# Server SDK 化

SDK 化（invoker）允许用户将 Kitex server 当作一个本地 SDK 调用。

调用通过 `message` 完成，初始化 `message` 需要 `local` 和 `remote` 两个 `net.Addr` ，分别表示本地地址和远端（客户端）地址（此处的地址主要用于日志监控），初始化后通过 `SetRequestBytes(buf []byte) error` 设置请求的二进制数据。最后调用 `invoker` 的 `Call` 方法即可完成调用。调用完成后可通过 `message` 的 `GetResponseBytes() ([]byte, error)` 获取响应的二进制数据。

```go
import (
		...
    "github.com/cloudwego/kitex/sdk/message"
  	...
)

func main() {
    var reqPayload, respPayload []byte
    var local, remote net.Addr
    ...
    // init local/remote
    ...
    ivk := echo.NewInvoker(new(EchoImpl))
    msg := message.NewMessage(local, remote)
    // 装载payload
    msg.SetRequestBytes(reqPayload)
    // 发起调用
    err := ivk.Call(msg)
    if err != nil {
        ...
    }
    respPayload, err = msg.GetResponseBytes()
    if err != nil {
        ...
    }
}
```