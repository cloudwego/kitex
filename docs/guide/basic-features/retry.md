## Request Retry 

### 1. Introduction

There are currently three types of retries: `Timeout Retry`, `Backup Request` and `Connection Failed Retry`. Among them, `Connection Failed Retry` is a network-level problem, since the request is not sent, the framework will retry by default. Here we only present the use of the first two types of retries:

- `Timeout Retry`: Improve the overall success rate of the service.
- `Backup Request`: Reduce delay jitter of request.

Because many requests are not idempotent, these two types of retries are not used as the default policy.

#### 1.1 Attention

- Confirm that your service is idempotent before enable retry.
- `Timeout Retry` will increase overall latency.

### 2. Retry Policy

Only one of the `Timeout Retry` and `Backup Request` policies can be configured.

- `Timeout Retry`

Configuration Item|Default value|Description|Limit
----|----|----|----
`MaxRetryTimes`|2|The first request is not included. If it is configured as 0, it means to stop retrying.|Value: [0-5]
`MaxDurationMS`|0|Including the time-consuming of the first failed request and the retry request. If the limit is reached, the subsequent retry will be stopped. 0 means unlimited. Note: if configured, the configuration item must be greater than the request timeout.
`EERThreshold`|10%|If the method-level request error rate exceeds the threshold, retry stops.|Value: (0-30%]
`ChainStop`|-|`Chain Stop` is enabled by default. If the upstream request is a retry request, it will not be retried after timeout.|>= v0.0.5 as the default policy.
`DDLStop`|false|If the timeout period of overall request chain is reached, the retry request won't be sent with this policy. Notice, Kitex doesn't provide build-in implementation, use `retry.RegisterDDLStop(ddlStopFunc)` to register is needed. 
`BackOff`|None|Retry waiting strategy, `NoneBackOff` by default. Optional: `FixedBackOff`, `RandomBackOff`.
`RetrySameNode`|false|By default, Kitex selects another node to retry. If you want to retry on the same node, set this parameter to true.

- `Backup Request`

Configuration Item|Default value|Description|Limit
----|----|----|----
`RetryDelayMS`|-|Duration of waiting for initiating a Backup Requset when the first request is not returned. This parameter must be set manually. It is suggested to set as TP99.
`MaxRetryTimes`|1|The first request is not included. If it is configured as 0, it means to stop retrying.|Value: [0-2]
`EERThreshold`|10%|If the method-level request error rate exceeds the threshold, retry stops.|Value: (0-30%]
`ChainStop`|false|`Chain Stop` is enabled by default. If the upstream request is a retry request, it will not be retried after timeout.|>= v0.0.5 as the default policy.
`RetrySameNode`|false|By default, Kitex selects another node to retry. If you want to retry on the same node, set this parameter to true.

### 3. How to use

#### 3.1 Enable by Code Configuration

Note: Dynamic configuration (see 3.3) cannot take effect if retry is enabled by code configuration.

##### 3.1.1 Timeout Retry Configuration

- Configuration e.g.

```go
// import "github.com/jackedelic/kitex/internal/pkg/retry"
fp := retry.NewFailurePolicy()
fp.WithMaxRetryTimes(3) // set the maximum number of retries to 3
xxxCli := xxxservice.NewClient("psm", client.WithFailureRetry(fp))
```

- Strategy selection:

```go
fp := retry.NewFailurePolicy()

// Number of retries. The default value is 2, excluding the first request.
fp.WithMaxRetryTimes(xxx)

// Total time consuming. Including the time-consuming for the first failed request and retry request. If the duration limit is reached, the subsequent retries are stopped.
fp.WithMaxDurationMS(xxx)

// Disable `Chain Stop`
fp.DisableChainRetryStop()

// Enable DDL abort
fp.WithDDLStop()

// Backoff policy. No backoff strategy by default.
fp.WithFixedBackOff(fixMS int) // Fixed backoff
fp.WithRandomBackOff(minMS int, maxMS int) // Random backoff

// Set errRate for retry circuit breaker
fp.WithRetryBreaker(errRate float64)

// Retry on the same node
fp.WithRetrySameNode()
```

##### 3.1.2 Backup Request Configuration

- Retry Delay recommendations

It is recommended to configure as TP99, then 1% request will trigger `Backup Request`.

- Configuration e.g.

```go
// If the first request is not returned after XXX ms, the backup request will be initiated and the `Chain Retry Stop` is enabled
bp := retry.NewBackupPolicy(xxx)
xxxCli := xxxservice.NewClient("psm", client.WithBackupRequest(bp))
```

- Strategy selection:

```go
bp := retry.NewBackupPolicy(xxx)

// Number of retries. The default value is 1, excluding the first request.
bp.WithMaxRetryTimes(xxx)

// Disable `Chain Stop`
bp.DisableChainRetryStop()

// Set errRate for retry circuit breaker
bp.WithRetryBreaker(errRate float64)

// Retry on the same node
bp.WithRetrySameNode()
```

#### 3.2 Circuit Breaker Reuse

When circuit breaker is enabled for a service, you can reuse the breaker's statistics to reduce additional CPU consumption. Note that the error rate threshold for retries must be lower than the threshold for a service, as follows:

```go
// 1. Initialize kitex's built-in cbsuite
cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)// 2. Initialize retryContainer, passing in ServiceControl and ServicePanel
retryC := retry.NewRetryContainerWithCB(cs.cbs.ServiceControl(), cs.cbs.ServicePanel())

var opts []client.Option
// 3. Set retryContainer
opts = append(opts, client.WithRetryContainer(retryC))
// 4. Set Service circuit breaker
opts = append(opts, client.WithMiddleware(cbs.ServiceCBMW()))

// 5. Initialize Client and pass in the configuration option
cli, err := xxxservice.NewClient(targetService, opts...)
```

#### 3.3 Dynamic open or adjust strategy

If you want to adjust the policy in combination with remote configuration, dynamic open retry, or runtime, you can take effect through the `NotifyPolicyChange` method of `retryContainer`. Currently, the open source version of Kitex does not provide a remote configuration module, and users can integrate their own configuration center. Note: If it is turned on through code configuration, dynamic configuration cannot take effect.

Use case:

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

#### 4. Tracking

Kitex records the retry times and previous request time in `rpcInfo`. You can report or output a retry request based on the `retry Tag` in Client's `metric` or log through:

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
```

#### 5. Downstream identification

If using `TTHeader` as the transport protocol, you can determine if the downstream `handler` is currently a retry request and decide whether to continue processing.

```go
retryReqCount, exist := metainfo.GetPersistentValue(ctx,retry.TransitKey)
```

For example, `retryReqCount = 2`, which means the second retry request (excluding the first request), then the business degradation strategy can be adopted(non-retry requests do not have this information).

>Question: `Chain Stop` is enabled by default, is it necessary for services to identify retry requests?

>Answer：`Chain Stop` means that the retry request on the chain will not be retried. Assuming that there is a request chain `A->B->C`, `A` sends a retry request to `B`, while during `B->C`, if a timeout occurs or `Backup` is configured, `B` will not send a retry request to `C`. If the service can identify the retry request, it can directly decide whether to continue the request to `C`.
In short, `Chain Stop` avoids retry amplification caused by `B` sending a retry request to `C`. The service's own control can completely avoid requests from `B` to `C`.
