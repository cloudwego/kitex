# Service Discovery Extension

[kitex-contrib](https://github.com/kitex-contrib/resolver-dns) has provided the DNS service discovery extensions.

If you want to adopt other service discovery protocol, such as ETCD, you can implement the `Resolver` interface, and clients can inject it by `WithResolver` Option.

## Interface Definition

The interface is defined in `pkg/discovery/discovery.go` and is defined as follows:

```go
type Resolver interface {
    Target(ctx context.Context, target rpcinfo.EndpointInfo) string
    Resolve(ctx context.Context, key string) (Result, error)
    Diff(key string, prev, next Result) (Change, bool)
    Name() string
}

type Result struct {
    Cacheable bool // if can be cached
    CacheKey  string // the unique key of cached result
    Instances []Instance // the result of service discovery
}

// the diff result
type Change struct {
    Result  Result
    Added   []Instance
    Updated []Instance
    Removed []Instance
}
```

`Resolver` interface detail:

- `Resolve`: as the core method of `Resolver`, it obtains the service discovery result from target key
- `Target`:   it resolves the unique target endpoint that from the downstream endpoints provided by `Resolve`, and the result will be used as the unique key of the cache
- `Diff`:  it is used to compare  the discovery results with the last time. The differences in results are used to notify other components, such as [loadbalancer](https://github.com/jackedelic/kitex/internal/blob/develop/docs/guide/extension/loadbalance.md) and circuitbreaker, etc
- `Name`:  it is used to specify a unique name for `Resolver`, and will use it to cache and reuse `Resolver`

## Usage Example

You need to implement the the `Resolver` interface, and using it by Option:

```go
import (
    "xx/kitex/client"
)

func main() {
    opt := client.WithResolver(YOUR_RESOLVER)

    // new client
    xxx.NewClient("p.s.m", opt)
}
```

## Attention

To improve performance,  Kitex reusing `Resolver`, so the `Resolver` method implementation must be concurrent security.

 