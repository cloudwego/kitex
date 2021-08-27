# 服务发现扩展

[kitex-contrib](https://github.com/kitex-contrib) 中提供了 dns 服务发现扩展。

用户如果需要更换其他的服务发现，例如 ETCD，用户可以根据需求实现 `Resolver ` 接口，client 通过 `WithResolver` Option 来注入。

## 接口定义

接口在 pkg/discovery/discovery.go 中，具体定义如下：

```go
// 服务发现接口定义
type Resolver interface {
    Target(ctx context.Context, target rpcinfo.EndpointInfo) string
    Resolve(ctx context.Context, key string) (Result, error)
    Diff(key string, prev, next Result) (Change, bool)
    Name() string
}

type Result struct {
    Cacheable bool // 是否可以缓存
    CacheKey  string // 缓存的唯一 key
    Instances []Instance // 服务发现结果
}

// diff 的结果
type Change struct {
    Result  Result
    Added   []Instance
    Updated []Instance
    Removed []Instance
}
```

Resolver 定义如下 :

- `Target` 方法是从 Kitex 提供的对端 EndpointInfo 中解析出 `Resolve` 需要使用的唯一 target, 同时这个 target 将作为缓存的唯一 key
- `Resolve` 方法作为 Resolver 的核心方法， 从 target key 中获取我们需要的服务发现结果 `Result`
- `Diff` 方法用于计算两次服务发现的变更， 计算结果一般用于通知其他组件， 如 [loadbalancer](./loadbalance_cn.md) 和熔断等， 返回的是变更 `Change`
- `Name` 方法用于指定 Resolver 的唯一名称， 同时 Kitex 会用它来缓存和复用 Resolver

## 自定义 Resolver

首先需要实现 Resolver 接口需要的方法， 通过配置项指定 Resolver

Kitex 提供了 Client 初始化配置项 :

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

## 注意事项

- 我们通过复用 Resolver 的方式来提高性能， 要求 Resolver 的方法实现需要是并发安全的