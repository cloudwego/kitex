# Monitoring Extension

[kitex-contrib](https://github.com/kitex-contrib/monitor-prometheus) has provided the prometheus monitoring extensions.

If you want to get the more detailed monitoring, such as message packet size, or want to adopt other data source, such as InfluxDB, you can implement the `Trace` interface according to your requirements and inject by `WithTracer` Option.

```go
// Tracer is executed at the start and finish of an RPC.
type Tracer interface {
    Start(ctx context.Context) context.Context
    Finish(ctx context.Context)
}
```

RPCInfo can be obtained from ctx, and further request time cost, package size, and error information returned by the request can be obtained from RPCInfo, for example:

```go
type clientTracer struct {
    // contain entities which recording metric
}

// Start record the beginning of an RPC invocation.
func (c *clientTracer) Start(ctx context.Context) context.Context {
    // do nothing
        return ctx
}

// Finish record after receiving the response of server.
func (c *clientTracer) Finish(ctx context.Context) {
        ri := rpcinfo.GetRPCInfo(ctx)
        rpcStart := ri.Stats().GetEvent(stats.RPCStart)
        rpcFinish := ri.Stats().GetEvent(stats.RPCFinish)
        cost := rpcFinish.Time().Sub(rpcStart.Time())
        // TODO: record the cost of request
}
```

