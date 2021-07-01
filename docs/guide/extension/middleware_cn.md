# Middleware 扩展

# 介绍

Middleware 是扩展 Kitex 框架的一个主要的方法，大部分基于 Kitex 的扩展和二次开发的功能都是基于 middleware 来实现的。

在扩展过程中，要记得两点原则：

1. 中间件和套件都只允许在初始化 Server、Client 的时候设置，不允许动态修改。
2. 后设置的会覆盖先设置的。

Kitex 的中间件定义在`pkg/endpoint/endpoint.go`中，其中最主要的是两个类型：

1. `Endpoint`是一个函数，接受 ctx、req、resp，返回 err，可参考下方示例；
2. `Middleware`（下称 MW）也是一个函数，接收同时返回一个`Endpoint`。
3. 实际上一个中间件就是一个输入是`Endpoint`，输出也是`Endpoint`的函数，这样保证了对应用的透明性，应用本身并不会知道是否被中间件装饰的。由于这个特性，中间件可以嵌套使用。
4. 中间件是串连使用的，通过调用传入的 next，可以得到后一个中间件返回的 response（如果有）和 err，据此作出相应处理后，向前一个中间件返回 err（务必判断 next err 返回，勿吞了 err）或者设置 response。

# 客户端中间件

有两种方法可以添加客户端中间件：

1. `client.WithMiddleware`对当前 client 增加一个中间件，在其余所有中间件之前执行；
2. `client.WithInstanceMW`对当前 client 增加一个中间件，在服务发现和负载均衡之后执行（如果使用了`Proxy`则不会调用到）。
3. 注意，上述函数都应该在创建 client 时作为传入的`Option`。
4. 客户端中间件调用顺序；
5. `client.WithMiddleware`设置的中间件
6. ACLMiddleware
7. (ResolveMW + client.WithInstanceMW + PoolMW / DialerMW) / ProxyMW
8. IOErrorHandleMW
9. 调用返回的顺序则相反。
10. 客户端所有中间件的调用顺序可以看`client/client.go`。

# 服务端中间件

服务端的中间件和客户端有一定的区别。

可以通过`server.WithMiddleware`来增加 server 端的中间件，使用方式和 client 一致，在创建 server 时通过`Option`传入。

总的服务端中间件的调用顺序可以看`server/server.go`。

# 示例

我们可以通过以下这个例子来看一下如何使用中间件。

假如我们现在有需求，需要在请求前打印出 request 内容，再请求后打印出 response 内容，可以编写如下的 MW：

```go
func PrintRequestResponseMW(next endpoint.Endpoint) endpoint.Endpoint {
    return func(ctx context.Context, request, response interface{}) error {
        fmt.Printf("request: %v\n", request)
        err := next(ctx, request, response)
        fmt.Printf("response: %v", response)
        return err
    }
}
```

假设我们是 Server 端，就可以使用`server.WithMiddleware(PrintRequestResponseMW)`来使用这个 MW 了。

**以上方案仅为示例，不可用于生产，会有性能问题。**

# 注意事项

如果自定义 middleware 中用到了 RPCInfo，要注意 RPCInfo 在 rpc 结束之后会被回收，所以如果在 middleware 中起了 goroutine 操作 RPCInfo 会出问题，不能这么做。
