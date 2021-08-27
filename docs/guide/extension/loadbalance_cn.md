# 自定义 LoadBalancer

在 Kitex 中，提供了两种 LoadBalancer：

1. WeightedRandom
2. ConsistentHash

这两种 LoadBalancer 能覆盖绝大多数的应用场景，如果业务有一些 Corner Case 无法覆盖到，可以选择自己自定义 LoadBalancer。

## 接口

LoadBalancer 接口在 `pkg/loadbalance/loadbalancer.go` 中，具体定义如下：

```go
// Loadbalancer generates pickers for the given service discovery result.
type Loadbalancer interface {
	GetPicker(discovery.Result) Picker
    // 名称需要唯一
    Name() string
}
```

可以看到 LoadBalancer 获取到一个 Result 并且生成一个针对当次请求的 Picker，Picker 定义如下：

```go
// Picker picks an instance for next RPC call.
type Picker interface {
	Next(ctx context.Context, request interface{}) discovery.Instance
}
```

有可能在一次的 rpc 请求中，所选择的实例连接不上，而要进行重试，所以设计成这样。

如果说已经没有实例可以重试了，Next 方法应当返回 nil。

除了以上接口之外，还有一个比较特殊的接口，定义如下：

```go
// Rebalancer is a kind of Loadbalancer that performs rebalancing when the result of service discovery changes.
type Rebalancer interface {
	Rebalance(discovery.Change)
	Delete(discovery.Change)
}
```

如果 LoadBalancer 支持 Cache，务必实现 Rebalancer 接口，否则服务发现变更就无法被通知到了。

Kitex client 会在初始化的时候执行以下代码来保证当服务发现变更时，能通知到 Rebalancer：

```go
if rlb, ok := balancer.(loadbalance.Rebalancer); ok && bus != nil {
    bus.Watch(discovery.DiscoveryChangeEventName, func(e *event.Event) {
        change := e.Extra.(*discovery.Change)
        rlb.Rebalance(*change)
    })
}
```

## 注意事项

1. 如果是动态服务发现，那么尽可能实现缓存，可以提高性能；
2. 如果使用了缓存，那么务必实现 Rebalancer 接口，否则当服务发现变更时无法收到通知；
3. 使用了 Proxy 场景下，不支持自定义 LoadBalancer。

## Example

可以参考一下 Kitex 默认的 WeightedRandom 实现。