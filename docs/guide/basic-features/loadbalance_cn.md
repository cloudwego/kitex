# 负载均衡

Kitex 默认提供了两种 LoadBalancer（下面简称 lb）：

* WeightedRandom
* ConsistentHash

Kitex 默认使用的是 WeightedRandom。

## WeightedRandom

顾名思义，这个 lb 使用的是基于权重的随机策略，也是 Kitex 的默认策略。

这个 lb 会依据实例的权重进行加权随机，并保证每个实例分配到的负载和自己的权重成比例。

如果所有的实例的权重都一样，Kitex 针对这个场景做了特殊优化，会使用一个纯随机的实现，来避免加权计算的一些额外开销，所以不需要担心这个场景下的性能。

## ConsistentHash

### 简介

一致性哈希主要适用于对上下文（如实例本地缓存）依赖程度高的场景，如希望同一个类型的请求打到同一台机器，则可使用该负载均衡方法。

**如果你不了解什么是一致性哈希，或者不知道带来的副作用，请勿使用一致性哈希。**

### 使用

如果要使用一致性哈希，可以在初始化 client 的时候传入`client.WithLoadBalancer(loadbalance.NewConsistBalancer(loadbalance.NewConsistentHashOption(keyFunc)))`。

`ConsistentHashOption` 定义如下：

```go
type ConsistentHashOption struct {
    GetKey KeyFunc

    // 是否使用 replica
    // 如果使用，当请求失败（连接失败）后会依次尝试 replica
    // 会带来额外内存和计算开销
    // 如果不设置，那么请求失败（连接失败）后直接返回
    Replica uint32

    // 虚拟节点数
    // 每个真实节点对应的虚拟节点的数量
    // 这个数值越大，内存和计算代价越大，负载越均衡
    // 当节点数多时，可以适当设小一些；反之可以适当设大一些
    // 推荐 VirtualFactor * Weight（如果 Weighted 为 true）的中位数在 1000 左右，负载应当已经很均衡了
    // 推荐 总虚拟节点数 在 2000W 以内（1000W 情况之下 build 一次需要 250ms，不过为后台 build 理论上 3s 内均无问题）
    VirtualFactor    uint32

    // 是否要遵循 Weight 进行负载均衡
    // 如果为 false，对于每个 instance 都会忽略 Weight，均生成 VirtualFactor 个虚拟节点，进行无差别负载均衡
    // 如果为 true，对于每个 instance 会生成 instance.Weight() * VirtualFactor 个虚拟节点
    // 需要注意，对于 weight 为 0 的 instance，无论 VirtualFactor 为多少，均不会生成虚拟节点
    // 建议设为 true，不过要注意适当调小 VirtualFactor
    Weighted bool

    // 是否进行过期处理
    // 实现会缓存所有的 Key
    // 如果永不过期会导致内存一直增长
    // 设置过期会导致额外性能开销
    // 目前的实现是每分钟扫描删除一次，以及实例发生变动 rebuild 时删除一次
    // 建议一定要设置，值不要小于一分钟
    ExpireDuration time.Duration
}
```

要注意，如果 GetKey 是 nil 或者 VirtualFactor 是 0，会 panic。

### 性能

经过测试，在 weight 为 10、VirtualFactor 为 100 的情况之下，不同 instance 数量的 build 性能如下：

```
BenchmarkNewConsistPicker_NoCache/10ins-16                 6565        160670 ns/op      164750 B/op          5 allocs/op
BenchmarkNewConsistPicker_NoCache/100ins-16                 571       1914666 ns/op     1611803 B/op          6 allocs/op
BenchmarkNewConsistPicker_NoCache/1000ins-16                 45      23485916 ns/op    16067720 B/op         10 allocs/op
BenchmarkNewConsistPicker_NoCache/10000ins-16                 4     251160920 ns/op    160405632 B/op        41 allocs/op
```

所以当有 10000 个 instance，每个 instance weight 为 10，VirtualFactor 为 100 的情况之下（总虚拟节点数 1000W），build 一次需要 251 ms。

build 和 请求 信息都会被缓存，所以一次正常请求（不需要 build）的时延和节点多少无关，如下：

````
BenchmarkNewConsistPicker/10ins-16             12557137            81.1 ns/op         0 B/op          0 allocs/op 
BenchmarkNewConsistPicker/100ins-16            13704381            82.3 ns/op         0 B/op          0 allocs/op 
BenchmarkNewConsistPicker/1000ins-16           14418103            81.3 ns/op         0 B/op          0 allocs/op 
BenchmarkNewConsistPicker/10000ins-16          13942186            81.0 ns/op         0 B/op          0 allocs/op
````

### 注意事项

1. 下游节点发生变动时，一致性哈希结果可能会改变，某些 key 可能会发生变化；
2. 如果下游节点非常多，第一次冷启动时 build 时间可能会较长，如果 rpc 超时短的话可能会导致超时；
3. 如果第一次请求失败，并且 Replica 不为 0，那么会请求到 Replica 上；而第二次及以后仍然会请求 第一个 实例。

### 关于负载均衡

经过测试，当下游实例为 10 个时，如果 VirtualFactor 设置为 1 并且不开启 Weighted 时，负载非常不均衡，如下：

```
addr2: 28629
addr7: 13489
addr3: 10469
addr9: 4554
addr0: 21550
addr6: 6516
addr8: 2354
addr4: 9413
addr5: 1793
addr1: 1233
```

当 VirtualFactor 设置为 10 时，负载如下：

```
addr7: 14426
addr8: 12469
addr3: 8115
addr4: 8165
addr0: 8587
addr1: 7193
addr6: 10512
addr9: 14054
addr2: 9307
addr5: 7172
```

可以看出比 VirtualFactor 为 1 时要好很多。

当 VirtualFactor 为 1000 时，负载如下：

```
addr7: 9697
addr5: 9933
addr6: 9955
addr4: 10361
addr8: 9828
addr0: 9729
addr9: 10528
addr2: 10121
addr3: 9888
addr1: 9960
```

可以看出此时负载基本均衡。

再来看看带 Weight 的情况，我们设置 addr0 的 weight 为 0，addr1 的 weight 为 1，addr2 的 weight 为 2……以此类推。

设置 VirtualFactor 为 1000，得到负载结果如下：

```
addr4: 8839
addr3: 6624
addr6: 13250
addr1: 2318
addr8: 17769
addr2: 4321
addr5: 11099
addr9: 20065
addr7: 15715
```

可以看到基本是和 weight 的分布一致。在这里没有 addr0 是因为 weight 为 0 是不会被调度到的。

综上，提高 VirtualFactor，可以使得负载更加均衡，但是也要注意会增加性能开销，需要找个平衡点。