## Kitex 请求重试文档

### 1. 重试功能说明

目前有三类重试：超时重试、Backup Request，建连失败重试（默认）。其中建连失败是网络层面问题，由于请求未发出，框架会默认重试。 本文档介绍前两类重试的使用：
 - 超时重试：提高服务整体的成功率
 - Backup Request：减少服务的延迟波动

因为很多的业务请求不具有幂等性，这两类重试不会作为默认策略。

#### 1.1 注意：
- 确认你的服务具有幂等性，再开启重试
- 超时重试会增加延迟

### 2. 重试策略
超时重试和 Backup Request 策略只能配置其中之一。
- 超时重试

| 配置项             | 默认值 | 说明                                                                                                                                    | 限制            |
| ------------------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------- | --------------- |
| 最大重试次数       | 2      | 不包含首次请求。如果配置为 0 表示停止重试。                                                                                               | 合法值：[0-5]   |
| 累计最大耗时       | 0      | 包括首次失败请求和重试请求耗时，如果耗时达到了限制的时间则停止后续的重试。0 表示无限制。注意：如果配置，该配置项必须大于请求超时时间。           |                 |
| 重试熔断错误率阈值 | 10%    | 方法级别请求错误率超过阈值则停止重试。                                                                                                  | 合法值：(0-30%] |
| 链路中止       | -  | 默认启用链路中止，如果上游请求是重试请求，超时后不会重试。                                                                              |     >= v0.0.5 后作为默认策略            |
| DDL 中止            | false  | 链路超时中止，该策略是从链路的超时时间判断是否需要重试。注意，Kitex 未内置该实现，需通过 retry.RegisterDDLStop(ddlStopFunc) 注册 DDL func，结合链路超时判断，实现上建议基于上游的发起调用的时间戳和超时时间判断。​​ |                 |
| 退避策略           | None   | 重试等待策略，默认立即重试。可选：固定时长退避 (FixedBackOff)、随机时长退避 (RandomBackOff)                                               |                 |
| 重试同一节点       | false  | 框架默认选择其他节点重试，若需要同节点重试，可配置为 true                                                                                |                 |

- Backup Request

| 配置项             | 默认值 | 说明                                                                                                   | 限制            |
| ------------------ | ------ | ------------------------------------------------------------------------------------------------------ | --------------- |
| 重试等待时间       | -      | Backup Request 的等待时间，若该时间内若请求未返回，会发送新的请求。必须手动配置，建议参考 TP99。 |                 |
| 最大重试次数       | 1      | 不包含首次请求。 如果配置为 0 表示停止重试。                                                             | 合法值：[0-2]   |
| 重试熔断错误率阈值 | 10%    | 方法级别请求错误率超过阈值则停止重试。                                                                 | 合法值：(0-30%] |
| 链路中止           | -  | 默认启用链路中止，如果上游请求是重试请求，不会发送Backup Request。                                              |   >= v0.0.5 后作为默认策略              |
| 重试同一节点       | false  | 框架默认选择其他节点重试，若需要同节点重试，可配置为 true                                               |                 |

### 3. 使用方式
#### 3.1 代码配置开启
注意：若通过代码配置开启重试，动态配置 (见 3.3) 则无法生效。
##### 3.1.1 超时重试配置
- 配置示例：
```go
// import "github.com/cloudwego/kitex/pkg/retry"
fp := retry.NewFailurePolicy()
fp.WithMaxRetryTimes(3) // 配置最多重试3次
xxxCli := xxxservice.NewClient("psm", client.WithFailureRetry(fp))
```

- 策略选择：

```go
fp := retry.NewFailurePolicy()

// 重试次数, 默认2，不包含首次请求
fp.WithMaxRetryTimes(xxx)

// 总耗时，包括首次失败请求和重试请求耗时达到了限制的duration，则停止后续的重试。
fp.WithMaxDurationMS(xxx)

// 关闭链路中止
fp.DisableChainRetryStop()

// 开启DDL中止
fp.WithDDLStop()

// 退避策略，默认无退避策略
fp.WithFixedBackOff(fixMS int) // 固定时长退避
fp.WithRandomBackOff(minMS int, maxMS int) // 随机时长退避

// 开启重试熔断
fp.WithRetryBreaker(errRate float64)

// 同一节点重试
fp.WithRetrySameNode()
```

##### 3.1.2 Backup Request 配置
- Retry Delay 建议

建议配置为 TP99，则 1% 请求会触发 Backup Request。
- 配置示例：
```go
// 首次请求 xxx ms未返回，发起 backup 请求，并开启链路中止
bp := retry.NewBackupPolicy(xxx)
xxxCli := xxxservice.NewClient("psm", client.WithBackupRequest(bp))
```
- 策略选择：
```go
bp := retry.NewBackupPolicy(xxx)

// 重试次数, 默认1，不包含首次请求
bp.WithMaxRetryTimes(xxx)

// 关闭链路中止
bp.DisableChainRetryStop()

// 开启重试熔断
bp.WithRetryBreaker(errRate float64)

// 同一节点重试
bp.WithRetrySameNode()
```

#### 3.2 复用熔断器
当开启了服务的熔断配置可以复用熔断的统计减少额外的 CPU 消耗，注意重试的熔断阈值须低于服务的熔断阈值，使用如下：
```go
// 1. 初始化kitex内置的 cbsuite 
cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
// 2. 初始化retryContainer，传入ServiceControl和ServicePanel
retryC := retry.NewRetryContainerWithCB(cs.cbs.ServiceControl(), cs.cbs.ServicePanel())

var opts []client.Option
// 3. 配置 retryContainer
opts = append(opts, client.WithRetryContainer(retryC))
// 4. 配置 Service circuit breaker
opts = append(opts, client.WithMiddleware(cbs.ServiceCBMW()))

// 5. 初始化Client, 传入配置 option
cli, err := xxxservice.NewClient(targetService, opts...)
```

#### 3.3 动态开启或调整策略
若需要结合远程配置，动态开启重试或运行时调整策略，可以通过 retryContainer 的 NotifyPolicyChange 方法生效，目前 Kitex 开源版本暂未提供远程配置模块，使用者可集成自己的配置中心。注意：若已通过代码配置开启，动态配置则无法生效。
使用示例：
```go
retryC := retry.NewRetryContainer()
// demo
// 1. define your change func 
// 2. exec yourChangeFunc in your config module
yourChangeFunc := func(key string, oldData, newData interface{}) {
    newConf := newData.(*retry.Policy)
    method := parseMethod(key)
    retryC.NotifyPolicyChange(method, policy)
}


// configure retryContainer
cli, err := xxxservice.NewClient(targetService, client.WithRetryContainer(retryC))
```

### 4. 监控埋点
Kitex 对重试的请求在 rpcinfo 中记录了重试次数和之前请求的耗时，可以在Client侧的 metric 或日志中根据 retry tag 区分上报或输出。获取方式：
```go
var retryCount string
var lastCosts string

toInfo := rpcinfo.GetRPCInfo(ctx).To()
if retryTag, ok := toInfo.Tag(rpcinfo.RetryTag); ok {
   retryCount = retryTag
   if lastCostTag, ok := toInfo.Tag(rpcinfo.RetryLastCostTag); ok {
      lastCosts = lastCostTag
   }
}
````
### 5. 下游识别重试请求       
如果使用 TTHeader 作为传输协议，下游 handler 可以通过如下方式判断当前是否是重试请求，自行决定是否继续处理。
```go
retryReqCount, exist := metainfo.GetPersistentValue(ctx,retry.TransitKey)
```
比如 retryReqCount = 2，表示第二次重试请求（不包括首次请求），则采取业务降级策略返回部分或 mock 数据返回（非重试请求没有该信息）。

> Q: 框架默认开启链路中止，业务是否还有必要识别重试请求？
> 
> 链路中止是指链路上的重试请求不会重试，比如 A->B->C，A 向 B 发送的是重试请求，如果 B->C 超时了或者配置了 Backup，则 B 不会再发送重试请求到 C。如果业务自行识别重试请求，可以直接决定是否继续请求到 C。简言之链路中止避免了 B 向 C 发送重试请求导致重试放大，业务自己控制可以完全避免 B 到 C 的请求。