# Metainfo

## Meta Information

As an RPC framework, Kitex services communicate with each other through protocols described by IDL (thrift, protobuf, etc.). The interface defined in an IDL determines the data structures that could be transmitted between the client and the server.

However, in the production environment, we somehow may need to send special information to a remote server and that information is temporary or has an unstable format which can not be explictly defined in the IDL. Such a situation requests the framework to be capable of sending meta information.

When the underlying transport protocol supports (such as TTHeader, HTTP), then Kitex can transmit meta information.

To decouple with the underlying transport protocols, and interoperate with other frameworks, Kitex does not provide APIs to read or write meta information directly. Instead, it uses a stand-alone library [metainfo][metainfo] to support meta inforamtion transmitting.

## Forward Meta Information Transmitting

Pacakge [metainfo][metainfo] provides two kinds of API for sending meta information forward -- transient and persistent. The former is for ordinary needs of sending meta information; while the later is used when the meta information needs to be kept and sent to the next service and on, like a log ID or a dying tag. Of course, the persistent APIs works only when the next service and its successors all supports the meta information tarnsmitting convention.

A client side example:

```golang
import "github.com/bytedance/gopkg/cloud/metainfo"

func main() {
    ...
    ctx := context.Background()
    cli := myservice.MustNewCilent(...)
    req := myservice.NewSomeRequest()

    ctx = metainfo.WithValue(ctx, "temp", "temp-value")       // attach the meta information to the context
    ctx = metainfo.WithPersistentValue(ctx, "logid", "12345") // attach persistent meta information
    resp, err := cli.SomeMethod(ctx, req)                     // pass the context as an argument
    ...
}
```

A server side example:

```golang
import (
    "context"

    "github.com/bytedance/gopkg/cloud/metainfo"
)

var cli2 = myservice2.MustNewCilent(...) // the client for next service

func (MyServiceImpl) SomeMethod(ctx context.Context, req *SomeRequest) (res *SomeResponse, err error) {
    temp, ok1 := metainfo.GetValue(ctx, "temp")
    logid, ok2 := metainfo.GetPersistentValue(ctx, "logid")

    if !(ok1 && ok2) {
        panic("It looks like the protocol does not support transmitting meta information")
    }
    println(temp)  // "temp-value"
    println(logid) // "12345"

    // if we need to call another service
    req2 := myservice2.NewRequset()
    res2, err2 := cli2.SomeMethod2(ctx, req2) // pass the context to other service for the persistent meta information to be transmitted continuously
    ...
}
```

## Backward Meta Information Transmitting

Some transport protocols also support backward meta information transmitting. So Kitex supports that through [metainfo][metainfo], too.

A client side example:

```golang
import "github.com/bytedance/gopkg/cloud/metainfo"

func main() {
    ...
    ctx := context.Background()
    cli := myservice.MustNewCilent(...)
    req := myservice.NewSomeRequest()

    ctx = metainfo.WithBackwardValues(ctx) // mark the context to receive backward meta inforamtion
    resp, err := cli.SomeMethod(ctx, req)  // pass the context as an argument

    if err == nil {
        val, ok := metainfo.RecvBackwardValue(ctx, "something-from-server") // receive the meta information from server side
        println(val, ok)
    }
    ...
}
```

A server side example:

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

