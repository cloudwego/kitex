# 传输模块扩展

![remote_module](../../images/remote_module.png)

Kitex 默认集成了自研的高性能网络库 Netpoll，同时也支持使用者扩展其他网络库按需选择。Kitex 还提供了 Shm IPC 进一步提升 IPC 性能，该扩展会在后续开源。

## 扩展接口

传输模块主要的扩展接口如下：

```go
type TransServer interface {...}

type ServerTransHandler interface {...}

type ClientTransHandler interface {...}

type ByteBuffer interface {...}

type Extension interface {...}

// -------------------------------------------------------------
// TransServerFactory is used to create TransServer instances.
type TransServerFactory interface {
	NewTransServer(opt *ServerOption, transHdlr ServerTransHandler) TransServer
}

// ClientTransHandlerFactory to new TransHandler for client
type ClientTransHandlerFactory interface {
	NewTransHandler(opt *ClientOption) (ClientTransHandler, error)
}

// ServerTransHandlerFactory to new TransHandler for server
type ServerTransHandlerFactory interface {
	NewTransHandler(opt *ServerOption) (ServerTransHandler, error)
}
```

TransServer 是服务端的启动接口，ServerTransHandler和ClientTransHandler分别是服务端和调用端对消息的处理接口，ByteBuffer 是读写接口。相同的 IO 模型下 TransHandler 的逻辑通常是一致的，Kitex对同步IO提供了默认实现 defaultTransHandler，针对不一样的地方抽象出了Extension接口，所以在同步IO的场景下不需要实现完整的TransHandler接口，只需实现Extension即可。

### Netpoll 的扩展

如下是Kitex对Netpoll 同步 IO 的扩展，分别实现了Extension、ByteBuffer、TransServer接口。

![netpoll_extension](../../images/netpoll_extension.pn)

## 指定自定义的传输模块

- 服务端

  option: `WithTransServerFactory`,  `WithTransHandlerFactory`

  ```go
  var opts []server.Option
  opts = append(opts, server.WithTransServerFactory(yourTransServerFactory)
  opts = append(opts, server.WithTransHandlerFactory(yourTransHandlerFactory)
                
  svr := xxxservice.NewServer(handler, opts...)
  ```

- 调用端

  option: `WithTransHandlerFactory`

  ```go
  cli, err := xxxservice.NewClient(targetService, client.WithTransHandlerFactory(yourTransHandlerFactory)
  ```

