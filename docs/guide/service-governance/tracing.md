# Tracing

Kitex supports original opentracing tracer, and also customized tracer.

## opentracing

client side, use opentracing `GlobalTracer` by default

```go
import (
	internal_opentracing "github.com/kitex-contrib/tracer-opentracing"
)
...
client, err := echo.NewClient("echo", internal_opentracing.DefaultClientOption())
if err != nil {
	log.Fatal(err)
}
```

server side, use opentracing `GlobalTracer` by default

```go
import (
	internal_opentracing "github.com/kitex-contrib/tracer-opentracing"
)
...
svr, err := echo.NewServer(internal_opentracing.DefaultServerOption())
if err := svr.Run(); err != nil {
	log.Println("server stopped with error:", err)
} else {
	log.Println("server stopped")
}
```

### Customize opentracing tracer and operation name

client side

```go
import (
	...
	ko "github.com/kitex-contrib/opentracing"
	"github.com/opentracing/opentracing-go"
	"github.com/jackedelic/kitex/internal/pkg/endpoint"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
  ...
)
...
myTracer := opentracing.GlobalTracer()
operationNameFunc := func(ctx context.Context) string {
	endpoint := rpcinfo.GetRPCInfo(ctx).To()
	return endpoint.ServiceName() + "::" + endpoint.Method()
}
...
client, err := echo.NewClient("echo", ko.ClientOption(myTracer, operationNameFunc))
if err != nil {
	log.Fatal(err)
}
```

server side

```go
import (
	...
	ko "github.com/kitex-contrib/opentracing"
	"github.com/opentracing/opentracing-go"
	"github.com/jackedelic/kitex/internal/pkg/endpoint"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
	...
)
...
myTracer := opentracing.GlobalTracer()
operationNameFunc := func(ctx context.Context) string {
	endpoint := rpcinfo.GetRPCInfo(ctx).To()
	return endpoint.ServiceName() + "::" + endpoint.Method()
}
...
svr, err := echo.NewServer(ko.ClientOption(myTracer, operationNameFunc))
if err := svr.Run(); err != nil {
	log.Println("server stopped with error:", err)
} else {
	log.Println("server stopped")
}
```

## Customize tracer

tracer interface:

```go
type Tracer interface {
	Start(ctx context.Context) context.Context
	Finish(ctx context.Context)
}
```

Example

client side:

```go
import "github.com/jackedelic/kitex/internal/client"
...
type myTracer struct {}

func (m *myTracer) Start(ctx context.Context) context.Context {
	_, ctx = opentracing.StartSpanFromContextWithTracer(ctx, o.tracer, "RPC call")
	return ctx
}

func (m *myTracer) Finish(ctx context.Context) {
	span := opentracing.SpanFromContext(ctx)
	span.Finish()
}
...
client, err := echo.NewClient("echo", client.WithTracer(&myTracer{}))
if err != nil {
	log.Fatal(err)
}
```

server side:

```go
import "github.com/jackedelic/kitex/internal/server"
...
type myTracer struct {}

func (m *myTracer) Start(ctx context.Context) context.Context {
	_, ctx = opentracing.StartSpanFromContextWithTracer(ctx, o.tracer, "RPC handle")
	return ctx
}

func (m *myTracer) Finish(ctx context.Context) {
	span := opentracing.SpanFromContext(ctx)
	span.Finish()
}
...
svr, err := echo.NewServer(server.WithTracer(&myTracer{}))
if err := svr.Run(); err != nil {
	log.Println("server stopped with error:", err)
} else {
	log.Println("server stopped")
}
```
