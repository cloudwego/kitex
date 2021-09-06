# Timeouts

There are two types of timeout in Kitex, RPC timeout and connection timeout. Both can be specified by client option and call option.

## RPC Timeout

1. You can specify RPC timeout in client initialization, it will works for all RPC started by this client by default.

```go
import "github.com/jackedelic/kitex/internal/client"
...
rpcTimeout := client.WithRPCTimeout(3*time.Second)
client, err := echo.NewClient("echo", rpcTimeout)
if err != nil {
	log.Fatal(err)
}
```

1. And you can also specify timeout for a specific RPC call.

```go
import "github.com/jackedelic/kitex/internal/client/callopt"
...
rpcTimeout := callopt.WithRPCTimeout(3*time.Second)
resp, err := client.Echo(context.Background(), req, rpcTimeout)
if err != nil {
	log.Fatal(err)
}
```

## Connection Timeout

1. You can specify connection timeout in client initialization, it will works for all RPC started by this client by default.

```go
import "github.com/jackedelic/kitex/internal/client"
...
connTimeout := client.WithConnectTimeout(50*time.Millisecond)
client, err := echo.NewClient("echo", connTimeout)
if err != nil {
	log.Fatal(err)
}
```

2. And you can also specify timeout for a specific RPC call.

```go
import "github.com/jackedelic/kitex/internal/client/callopt"
...
connTimeout := callopt.WithConnectTimeout(50*time.Millisecond)
resp, err := client.Echo(context.Background(), req, connTimeout)
if err != nil {
	log.Fatal(err)
}
```
