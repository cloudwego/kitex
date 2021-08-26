# Customize LoadBalancer

Kitex provides two LoadBalancers:

1. WeightedRandom
2. ConsistentHash

These two LoadBalancers can cover most of the use cases, but you can also customize your own LoadBalancer if they doesn't meet your needs.

## Interface

`Loadbalancer` is defined at `pkg/loadbalance/loadbalancer.go`:

```go
// Loadbalancer generates pickers for the given service discovery result.
type Loadbalancer interface {
	GetPicker(discovery.Result) Picker
    // Name should be unique
    Name() string
}
```

As you see, LoadBalancer gets a Result and generates a Picker for the current request, the Picker is defined as follows:

```go
// Picker picks an instance for next RPC call.
type Picker interface {
	Next(ctx context.Context, request interface{}) discovery.Instance
}
```

In a single rpc request, the selected instance may not be connected and should to be retried, that's why it's been designed like that.

If there are no more instances to retry, the Next method should return nil.

There are another special interface, defined as follows:

```go
// Rebalancer is a kind of Loadbalancer that performs rebalancing when the result of service discovery changes.
type Rebalancer interface {
	Rebalance(discovery.Change)
	Delete(discovery.Change)
}
```

If LoadBalancer supports Cache, make sure to implement the Rebalancer interface, otherwise the service will not be notified when discovery results changes.

Kitex client will execute the following code during initialization to ensure that the Rebalancer is notified when discovery results changes.

```go
if rlb, ok := balancer.(loadbalance.Rebalancer); ok && bus ! = nil {
    bus.Watch(discovery.DiscoveryChangeEventName, func(e *event.Event) {
        change := e.Extra.(*discovery.Change)
        rlb.Rebalance(*change)
    })
}
```

## Attention

1. If you are using dynamic service discovery, you should be better to implement caching which can improve performance.
2. If you are using cache, you should be better to implement the Rebalancer interface, otherwise you will not be notified when discovery results changes.
3. Customize LoadBalancer is not supported in the case of Proxy.

## Example

You can refer to the implementation of WeightedRandom.
