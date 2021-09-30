# Metainfo

## 元信息

作为一个 RPC 框架，Kitex 服务之间的通信都是基于 IDL（thrift、protobuf 等）描述的协议进行的。IDL 定义的服务接口决定了客户端和服务端之间可以传输的数据结构。

然而在实际生产环境，我们偶尔会有特殊的信息需要传递给对端服务，而又不希望将这些可能是临时或者格式不确定的内容显式定义在 IDL 里面，这就需要框架能够支持一定的元信息传递能力。

如果底层传输协议支持（例如 TTheader、HTTP），那么 Kitex 可以进行元信息的透传。

为了和底层的协议解耦，同时也为了支持与不同框架之间的互通，Kitex 并没有直接提供读写底层传输协议的元信息的 API，而是通过一个独立维护的基础库 [metainfo][metainfo] 来支持元信息的传递。


## 正向元信息传递

包 [metainfo][metainfo] 提供了两种类型的正向元信息传递 API：临时的（transient）和持续的（persistent）。前者适用于通常的元信息传递的需求；后者是在对元信息有持续传递需求的场合下使用，例如日志 ID、染色等场合，当然，持续传递的前提是下游以及更下游的服务都是支持这一套数据透传的约定，例如都是 Kitex 服务。

客户端的例子：

```golang
import "github.com/bytedance/gopkg/cloud/metainfo"

func main() {
    ...
    ctx := context.Background()
    cli := myservice.MustNewCilent(...)
    req := myservice.NewSomeRequest()

    ctx = metainfo.WithValue(ctx, "temp", "temp-value")       // 附加元信息到 context 里
    ctx = metainfo.WithPersistentValue(ctx, "logid", "12345") // 附加能持续透传的元信息
    resp, err := cli.SomeMethod(ctx, req)                     // 将得到的 context 作为客户端的调用参数
    ...
}
```

服务端的例子：

```golang
import (
    "context"

    "github.com/bytedance/gopkg/cloud/metainfo"
)

var cli2 = myservice2.MustNewCilent(...) // 更下游的服务的客户端

func (MyServiceImpl) SomeMethod(ctx context.Context, req *SomeRequest) (res *SomeResponse, err error) {
    temp, ok1 := metainfo.GetValue(ctx, "temp")
    logid, ok2 := metainfo.GetPersistentValue(ctx, "logid")

    if !(ok1 && ok2) {
        panic("It looks like the protocol does not support transmitting meta information")
    }
    println(temp)  // "temp-value"
    println(logid) // "12345"

    // 如果需要调用其他服务的话
    req2 := myservice2.NewRequset()
    res2, err2 := cli2.SomeMethod2(ctx, req2) // 在调用其他服务时继续传递收到的 context，可以让持续的元信息继续传递下去
    ...
}
```

## 反向元信息传递

一些传输协议还支持反向的元数据传递，因此 Kitex 也利用 [metainfo][metainfo] 做了支持。

客户端的例子：

```golang
import "github.com/bytedance/gopkg/cloud/metainfo"

func main() {
    ...
    ctx := context.Background()
    cli := myservice.MustNewCilent(...)
    req := myservice.NewSomeRequest()

    ctx = metainfo.WithBackwardValues(ctx) // 标记要接收反向传递的数据的 context
    resp, err := cli.SomeMethod(ctx, req)  // 将得到的 context 作为客户端的调用参数

    if err == nil {
        val, ok := metainfo.RecvBackwardValue(ctx, "something-from-server") // 获取服务端传回的元数据
        println(val, ok)
    }
    ...
}
```

服务端的例子：

```golang
import (
    "context"

    "github.com/bytedance/gopkg/cloud/metainfo"
)

func (MyServiceImpl) SomeMethod(ctx context.Context, req *SomeRequest) (res *SomeResponse, err error) {
    ok := metainfo.SendBackwardValue(ctx, "something-from-server")

    if !ok) {
        panic("It looks like the protocol does not support transmitting meta information backward")
    }
    ...
}
```


[metainfo]: https://pkg.go.dev/github.com/bytedance/gopkg/cloud/metainfo

