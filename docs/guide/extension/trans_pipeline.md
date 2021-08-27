# Extension of Transport Pipeline-Bound

![remote_module](../../images/remote_module.png)

Transport Pipeline refers to Netty ChannelPipeline and provides `Inbound` and `Outbound` interfaces to support message or I/O event extensions. `TLS`,  `Traffic Limit`, `Transparent Transmission Processing` can be extended based on `In/OutboundHandler`. As shown in the figure below, each BoundHandler is executed in series.

![trans_pipeline](../../images/trans_pipeline.png)

## Extension API

```go
// OutboundHandler is used to process write event.
type OutboundHandler interface {
	Write(ctx context.Context, conn net.Conn, send Message) (context.Context, error)
}

// InboundHandler is used to process read event.
type InboundHandler interface {
	OnActive(ctx context.Context, conn net.Conn) (context.Context, error)

	OnInactive(ctx context.Context, conn net.Conn) context.Context

	OnRead(ctx context.Context, conn net.Conn) (context.Context, error)

	OnMessage(ctx context.Context, args, result Message) (context.Context, error)
}
```

### Default Extensions

- Traffic Limit Handler of Server Side

  Kitex supports connection level and request level limiting. The purpose of limiting is to ensure service availability. When the threshold is reached, the request should be limited in time. And the purpose of implementing limit in transport layer is to limit traffic in a timely manner. The implementation is in limiter_inbound.go.

  - Limiting of Connection level implements OnActive(), OnInactive()
  - Limiting of Request level implements OnRead()

- MetaInfo Transparent Transmission Handler

  Meta information transparent transmission is to transmit some RPC additional information to the downstream based on the transport protocol, and read the upstream transparent transmission information carried by transport protocol. The implementation is in transmeta_bound.go.

  - Write metainfo implements Write()
  - Read metainfo  implements OnMessage()
  
  In order to make it more convenient to extend MetaInfo Transparent Transmission for users, Kitex defines the separately extension API `MetaHandler`.
  
   ```go
   // MetaHandler reads or writes metadata through certain protocol.
   type MetaHandler interface {
   	WriteMeta(ctx context.Context, msg Message) (context.Context, error)
   	ReadMeta(ctx context.Context, msg Message) (context.Context, error)
   }
   ```
  
  

## Customized BoundHandler Usage

- Server Side

  option: `WithBoundHandler`

  ```go
  svr := xxxservice.NewServer(handler, server.WithBoundHandler(yourBoundHandler))
  ```

- Client Side

  option: `WithBoundHandler`

  ```go
  cli, err := xxxservice.NewClient(targetService, client.WithBoundHandler(yourBoundHandler))
  ```

  

