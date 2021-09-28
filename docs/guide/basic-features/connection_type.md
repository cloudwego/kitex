# Connection Type

Kitex provides **Short Connection**,  **Long Connection Pool** and **Connection Multiplexing** for different business scenarios. Kitex uses Long Connection Pool by default after v0.0.2, but adjusting the Pool Config according to your need is suggested.

## Short Connection

Every request needs to create a connection, the performance is bad, so it is not suggested normally.

Enable Short Connection：

```go
xxxCli := xxxservice.NewClient("destServiceName", client.WithShortConnection())
```

## Long Connection Pool

Kitex enable Long Connection Pool after >= v0.0.2, default config params are as below：

```
connpool2.IdleConfig{
   MaxIdlePerAddress: 10,
   MaxIdleGlobal:     100,
   MaxIdleTimeout:    time.Minute,
}
```

Adjusting the Pool Config according to your need is suggested, config as below:

```
xxxCli := xxxservice.NewClient("destServiceName", client.WithLongConnection(connpool.IdleConfig{10, 1000, time.Minute}))
```

Parameter description:

- `MaxIdlePerAddress`: the maximum number of idle connections per downstream instance
- `MaxIdleGlobal`: the global maximum number of idle connections
- `MaxIdleTimeout`: the idle duration of the connection. A connection that has been idle for more than MaxIdleTimeout will be closed (minimum value is 3s, the default value is 30s)

### Internal Implementation

Each downstream address corresponds to a connection pool, the connection pool is a ring composed of connections, and the size of the ring is `MaxIdlePerAddress`.

When getting a connection of downstream address, proceed as follows:
1. Try to fetch a connection from the ring, if fetching failed (no idle connections remained), then try to establish a new connection. In other words, the number of connections may exceed `MaxIdlePerAddress`
2. If fetching succeed, then check whether the idle time of the connection (since the last time it was placed in the connection pool) has exceeded MaxIdleTimeout. If yes, this connection will be closed and a new connection will be created.

When the connection is ready to be returned after used, proceed as follows:

1. Check whether the connection is normal, if not, close it directly
2. Check whether the idle connection number exceeds  `MaxIdleGlobal`, and if yes, close it directly
3. Check whether free space remained in the ring of the target connection pool, if yes, put it into the pool, otherwise close it directly

### Parameter Setting Suggestion

The setting of parameters is suggested as follows:
- `MaxIdlePerAddress`: the minimum value is 1, otherwise long connections would degenerate to short connections
  - What value should be set should be determined according to the throughput of downstream address. The approximate estimation formula is: `MaxIdlePerAddress = qps_per_dest_host*avg_response_time_sec`
  - For example, the cost of each request is 100ms, and the request spread to each downstream address is 100QPS, the value is suggested to set to 10, because each connection handles 10 requests per second, 100QPS requires 10 connections to handle
  - In the actual scenario, the fluctuation of traffic is also necessary to be considered. Pay attention, the connection within MaxIdleTimeout will be recycled if it is not used
  - Summary: this value be set too large or too small would lead to degenerating to the short connection
- `MaxIdleGlobal`: should be larger than the total number of `downstream targets number * MaxIdlePerAddress`
  - Notice: this value is not very valuable, it is suggested to set it to a super large value. In subsequent versions, considers discarding this parameter and providing a new interface
- `MaxIdleTimeout`: since the server will clean up inactive connections within 10min, the client also needs to clean up long-idle connections in time to avoid using invalid connections. This value cannot exceed 10min when the downstream is also a Kitex service

## Connection Multiplexing

The client invokes the Server only need one connection normally when enabling Connection Multiplexing. Connection Multiplexing not only reduces the number of connections but also performs better than Connection Pool.

Special Note:

1. Connection Multiplexing here is just for Thrift and Kitex Protobuf protocol. If you choose the gRPC protocol, it utilizes Connection Multiplexing mode by default.
2. When the client enables connection multiplexing, the server must also be enabled, otherwise, it will lead to request timeout. The server side has no restrictions on the client to enable connection multiplexing, it can accept requests for short connection, long connection pool, and connection multiplexing.

- Server Side Enable:

  option: `WithMuxTransport`

  ```go
  svr := xxxservice.NewServer(handler, server.WithMuxTransport())
  ```

- Client Side Enable:
  option: `WithMuxConnection`

  1-2 connection is enough normally, it is no need to config more.

  ```go
  xxxCli := NewClient("destServiceName", client.WithMuxConnection(1))
  ```

## 

## Status Monitoring

Connection pooling defines the `Reporter` interface for connection pool status monitoring, such as the reuse rate of long connections.
Users should implement the interface themselves and inject it by `SetReporter`.

```go
// Reporter report status of the connection pool.
type Reporter interface {
   ConnSucceed(poolType ConnectionPoolType, serviceName string, addr net.Addr)
   ConnFailed(poolType ConnectionPoolType, serviceName string, addr net.Addr)
   ReuseSucceed(poolType ConnectionPoolType, serviceName string, addr net.Addr)
}

// SetReporter set the common reporter of a connection pool, that can only be set once.
func SetReporter(r Reporter)
```
