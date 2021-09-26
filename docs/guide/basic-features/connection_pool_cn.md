# 连接池

Kitex 提供了短连接和长连接两种连接池，用户可以根据自己的业务场景来选择。

## 短连接

在不做任何设置的情况下，默认就是短连接。

## 长连接

在初始化 client 的时候，增加一个 Option：

```
client.WithLongConnection(connpool.IdleConfig{
    MaxIdlePerAddress: 10,
    MaxIdleGlobal:     1000,
    MaxIdleTimeout:    60 * time.Second,
})
```

其中：

- `MaxIdlePerAddress` 表示每个后端实例可允许的最大闲置连接数
- `MaxIdleGlobal` 表示全局最大闲置连接数
- `MaxIdleTimeout` 表示连接的闲置时长，超过这个时长的连接会被关闭（最小值 3s，默认值 30s ）

### 实现

长连接池的实现方案是每个 address 对应一个连接池，这个连接池是一个由连接构成的 ring，ring 的大小为 MaxIdlePerAddress。

当选择好目标地址并需要获取一个连接时，按以下步骤处理 :

1. 首先尝试从这个 ring 中获取，如果获取失败（没有空闲连接），则发起新的连接建立请求，即连接数量可能会超过 MaxIdlePerAddress
2. 如果从 ring 中获取成功，则检查该连接的空闲时间（自上次放入连接池后）是否超过了 MaxIdleTimeout，如果超过则关闭该连接并新建
3. 全部成功后返回给上层使用

在连接使用完毕准备归还时，按以下步骤依次处理：

1. 检查连接是否正常，如果不正常则直接关闭
2. 查看空闲连接是否超过全局的 MaxIdleGlobal，如果超过则直接关闭
3. 待归还到的连接池的 ring 中是否还有空闲空间，如果有则直接放入，否则直接关闭

### 参数设置建议

下面是参数设置的一些建议：

- `MaxIdlePerAddress` 表示池化的连接数量，最小为 1，否则长连接会退化为短连接
    - 具体的值与每个目标地址的吞吐量有关，近似的估算公式为：`MaxIdlePerAddress = qps_per_dest_host*avg_response_time_sec `
    - 举例如下，假设每个请求的响应时间为 100ms，平摊到每个下游地址的请求为 100QPS，该值建议设置为10，因为每条连接每秒可以处理 10 个请求, 100QPS 则需要 10 个连接进行处理
    - 在实际场景中，也需要考虑到流量的波动。需要特别注意的是，即 MaxIdleTimeout 内该连接没有被使用则会被回收
    - 总而言之，该值设置过大或者过小，都会导致连接复用率低，长连接退化为短连接
- `MaxIdleGlobal` 表示总的空闲连接数应大于 `下游目标总数*MaxIdlePerAddress`，超出部分是为了限制未能从连接池中获取连接而主动新建连接的总数量
    - 注意：该值存在的价值不大，建议设置为一个较大的值，在后续版本中考虑废弃该参数并提供新的接口
- `MaxIdleTimeout` 表示连接空闲时间，由于 server 在 10min 内会清理不活跃的连接，因此 client 端也需要及时清理空闲较久的连接，避免使用无效的连接，该值在下游也为 Kitex 时不可超过 10min

## 状态监控

连接池定义了 `Reporter` 接口，用于连接池状态监控，例如长连接的复用率。

如有需求，用户需要自行实现该接口，并通过 `SetReporter` 注入。

```go
// Reporter report status of connection pool.
type Reporter interface {
   ConnSucceed(poolType ConnectionPoolType, serviceName string, addr net.Addr)
   ConnFailed(poolType ConnectionPoolType, serviceName string, addr net.Addr)
   ReuseSucceed(poolType ConnectionPoolType, serviceName string, addr net.Addr)
}

// SetReporter set the common reporter of connection pool, that can only be set once.
func SetReporter(r Reporter)
```