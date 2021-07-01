# 监控扩展

用户如果需要更详细的打点，例如包大小；或更换其他数据源，例如 influxDB 的需求，用户可以根据需求实现 `Trace` 接口，并通过 `WithTracer` Option来注入。

```go
// Tracer is executed at the start and finish of an RPC.
type Tracer interface {
    Start(ctx context.Context) context.Context
    Finish(ctx context.Context)
}
```

从 ctx 中可以获得 RPCInfo，从而可以得到请求耗时、包大小和请求返回的错误信息等，举例：

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
