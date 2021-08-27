# 超时控制

Kitex 支持了两种超时，RPC 超时和连接超时，两种超时均支持 client 级别和调用级别的配置。

## RPC 超时

1. 在 client 初始化时配置，配置的 RPC 超时将对此 client 的所有调用生效

```go
import "github.com/cloudwego/kitex/client"
...
rpcTimeout := client.WithRPCTimeout(3*time.Second)
client, err := echo.NewClient("echo", rpcTimeout)
if err != nil {
	log.Fatal(err)
}
```

2. 在发起调用时配置，配置的 RPC 超时仅对此次调用生效

```go
import "github.com/cloudwego/kitex/client/callopt"
...
rpcTimeout := callopt.WithRPCTimeout(3*time.Second)
resp, err := client.Echo(context.Background(), req, rpcTimeout)
if err != nil {
	log.Fatal(err)
}
```

## 连接超时

1. 在 client 初始化时配置，配置的连接超时将对此 client 的所有调用生效

```go
import "github.com/cloudwego/kitex/client"
...
connTimeout := client.WithConnectTimeout(50*time.Millisecond)
client, err := echo.NewClient("echo", connTimeout)
if err != nil {
	log.Fatal(err)
}
```

2. 在发起调用时配置，配置的连接超时仅对此次调用生效

```go
import "github.com/cloudwego/kitex/client/callopt"
...
connTimeout := callopt.WithConnectTimeout(50*time.Millisecond)
resp, err := client.Echo(context.Background(), req, connTimeout)
if err != nil {
	log.Fatal(err)
}
```

