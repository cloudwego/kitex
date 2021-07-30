# 熔断器

Kitex 提供了熔断器的实现，但是没有默认开启，需要用户主动使用。

下面简单介绍一下如何使用以及 Kitex 熔断器的策略。

## 如何使用

- 使用示例：

```go
// build a new CBSuite
cbs := circuitbreak.NewCBSuite(GenServiceCBKeyFunc)

// add service circuit breaker with middleware
opts = append(opts, client.WithMiddleware(cbs.ServiceCBMW()))
// add instance circuit breaker with instance middleware
opts = append(opts, client.WithInstanceMW(cbs.InstanceCBMW()))

// init client
cli, err := xxxservice.NewClient(targetService, opts)
```

- 使用说明

Kitex 大部分服务治理模块都是通过 middleware 集成，熔断也是一样。Kitex 提供了一套 CBSuite，封装了服务粒度的熔断器和实例粒度的熔断器。

    - 服务粒度熔断：

      按照服务粒度进行熔断统计，通过 WithMiddleware 添加。服务粒度的具体划分取决于 Circuit Breaker Key，既熔断统计的 key，初始化CBSuite时需要传入 **GenServiceCBKeyFunc**，默认提供的是circuitbreaker.RPCInfo2Key，改 key 的格式是 `fromServiceName/toServiceName/method`，既按照方法级别的异常做熔断统计。

    - 实例粒度熔断

      按照实例粒度进行熔断统计，主要用于解决单实例异常问题，如果触发了实例级别熔断，框架会自动重试。

      注意，框架自动重试的前提是需要通过 **WithInstanceMW** 添加，WithInstanceMW 添加的 middleware 会在负载均衡后执行。

- 熔断阈值及**阈值变更**

  默认的熔断阈值是`ErrRate: 0.5, MinSample: 200`，错误率达到50%触发熔断，同时要求统计量>200。若要调整阈值，调用CBSuite的 `UpdateServiceCBConfig` 和 `UpdateInstanceCBConfig`来更新Key的阈值。

***

## 熔断器作用

在进行 RPC 调用时，下游服务难免会出错；

当下游出现问题时，如果上游继续对其进行调用，既妨碍了下游的恢复，也浪费了上游的资源；

为了解决这个问题，你可以设置一些动态开关，当下游出错时，手动的关闭对下游的调用；

然而更好的办法是使用熔断器，自动化的解决这个问题。

这里是一篇更详细的[熔断器介绍](https://msdn.microsoft.com/zh-cn/library/dn589784.aspx)。

比较出名的熔断器当属 hystrix 了，这里是它的[设计文档](https://github.com/Netflix/Hystrix/wiki)。

## 熔断策略

**熔断器的思路很简单：根据 RPC 的成功失败情况，限制对下游的访问；**

通常熔断器分为三个时期： CLOSED、OPEN、HALFOPEN；

RPC 正常时，为 CLOSED；

当 RPC 错误增多时，熔断器会被触发，进入 OPEN；

OPEN 后经过一定的冷却时间，熔断器变为 HALFOPEN；

HALFOPEN 时会对下游进行一些有策略的访问，然后根据结果决定是变为 CLOSED，还是 OPEN；

总的来说三个状态的转换大致如下图：

```
 [CLOSED] ---> tripped ----> [OPEN]<-------+
    ^                          |           ^
    |                          v           |
    +                          |      detect fail
    |                          |           |
    |                    cooling timeout   |
    ^                          |           ^
    |                          v           |
    +--- detect succeed --<-[HALFOPEN]-->--+
```

### 触发策略

Kitex 默认提供了三个基本的熔断触发策略：

- 连续错误数达到阈值(ConsecutiveTripFunc)

- 错误数达到阈值(ThresholdTripFunc)

- 错误率达到阈值(RateTripFunc)

当然，你可以通过实现TripFunc函数来写自己的熔断触发策略；

Circuitbreaker会在每次Fail或者Timeout时，去调用TripFunc，来决定是否触发熔断；

### 冷却策略

进入OPEN状态后，熔断器会冷却一段时间，默认是10秒，当然该参数可配置(CoolingTimeout)；

在这段时期内，所有的IsAllowed()请求将会被返回false；

冷却完毕后进入HALFOPEN；

### 半打开时策略

在HALFOPEN时，熔断器每隔"一段时间"便会放过一个请求，当连续成功"若干数目"的请求后，熔断器将变为CLOSED； 如果其中有任意一个失败，则将变为OPEN；

该过程是一个逐渐试探下游，并打开的过程；

上述的"一段时间"(DetectTimeout)和"若干数目"(DEFAULT_HALFOPEN_SUCCESSES)都是可以配置的；

## 统计

### 默认参数

熔断器会统计一段时间窗口内的成功，失败和超时，默认窗口大小是10S；

时间窗口可以通过两个参数设置，不过通常情况下你可以不用关心.

### 统计方法

统计方法是将该段时间窗口分为若干个桶，每个桶记录一定固定时长内的数据；

比如统计10秒内的数据，于是可以将10秒的时间段分散到100个桶，每个桶统计100ms时间段内的数据；

Options中的BucketTime和BucketNums，就分别对应了每个桶维护的时间段，和桶的个数；

如将BucketTime设置为100ms，将BucketNums设置为100，则对应了10秒的时间窗口；

### 抖动

随着时间的移动，窗口内最老的那个桶会过期，当最后那个桶过期时，则会出现了抖动；

举个例子：

- 你将10秒分为了10个桶，0号桶对应了[0S，1S)的时间，1号桶对应[1S，2S)，...，9号桶对应[9S，10S)；

- 在10.1S时，执行一次Succ，则circuitbreaker内会发生下述的操作；

- (1)检测到0号桶已经过期，将其丢弃； (2)创建新的10号桶，对应[10S，11S)； (3)将该次Succ放入10号桶内；

- 在10.2S时，你执行Successes()查询窗口内成功数，则你得到的实际统计值是[1S，10.2S)的数据，而不是[0.2S，10.2S)；

如果使用分桶计数的办法，这样的抖动是无法避免的，比较折中的一个办法是将桶的个数增多，可以降低抖动的影响；

如划分2000个桶，则抖动对整体的数据的影响最多也就1/2000； 在该包中，默认的桶个数也是2000，桶时间为5ms，总体窗口为10S；

当时曾想过多种技术办法来避免这种问题，但是都会引入跟多其他的问题，如果你有好的思路，请issue或者MR.