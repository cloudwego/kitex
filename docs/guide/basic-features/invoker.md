# Server SDK Mode

SDK Mode（invoker）provides a way to call Kitex server just like a SDK.

`message` is used to start a call, you should use `local` and `remote` two `net.Addr` to initialize `message`. 
Which indicates local and remote address (used in logging and tracing). 
After initialization, you can use `SetRequestBytes(buf []byte) error` to setup request binary.
Then call `Call` method of `invoker` to start a call. After call, you can use `GetResponseBytes() ([]byte, error)` of `message` to get response binary.

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
    // setup request payload
    msg.SetRequestBytes(reqPayload)
    // start a call
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
