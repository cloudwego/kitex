# Circuit Breaker

Kitex provides a default implementation of Circuit Breaker, but it's disabled by default.

The following document will introduce that how to enable circuit breaker and configure the policy.


## How to use

### Example

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

### Introduction

Kitex provides a set of CBSuite that encapsulates both service-level breaker and instance-level breaker, which are the implementions of Middleware.

- Service-Level Breaker

	Statistics by service granularity, enabled via WithMiddleware.

	The specific division of service granularity depends on the Circuit Breaker Key, which is the key for breaker statistics. When initializing the CBSuite, you need to pass it in **GenServiceCBKeyFunc**. The default key is `circuitbreaker.RPCInfo2Key`, and the format of RPCInfo2Key is ` fromServiceName/toServiceName/method`.

- Instance-Level Breaker

	Statistics by instance granularity, enabled via WithInstanceMW.

	Instance-Level Breaker is used to solve the single-instance exception problem. If it's triggered, the framework will automatically retry the request.

	Note that the premise of retry is that you need to enable breaker with **WithInstanceMW**, which will be executed after load balancing.


- Threshold and **Threshold Change**

The default breaker threshold is `ErrRate: 0.5, MinSample: 200`, which means it's triggered by an error rate of 50% and requires the count of requests > 200. 

If you want to change the threshold, you can modify the `UpdateServiceCBConfig` and `UpdateInstanceCBConfig` in CBSuite.

## The Role of Circuit Breaker

When making RPC calls, errors are inevitable for downstream services.

When a downstream has a problem, if the upstream continues to make calls to it, it both prevents the downstream from recovering and wastes the upstream's resources.

To solve this problem, you can set up some dynamic switches that manually shut down calls to the downstream when it goes wrong.

A better approach, however, is to use Circuit Breaker.

Here is a more detailed document [Circuit Breaker Pattern](https://docs.microsoft.com/en-us/previous-versions/msp-n-p/dn589784(v=pandp.10)?redirectedfrom=MSDN).

One of the famous circuit breakers is hystrix, and here is its [design](https://github.com/Netflix/Hystrix/wiki).


## Breaker Strategy

The idea of a Circuit Breaker is simple: restrict access to downstream based on successful failures of RPC Calls.

The Circuit Breaker is usually divided into three periods: CLOSED, OPEN, and HALFOPEN.

- CLOSED when the RPC is normal.
- OPEN when RPC errors increase.
- HALFOPEN after a certain cooling time after OPEN.


HALFOPEN will make some strategic access to the downstream, and then decide whether to become CLOSED or OPEN according to the result.

In general, the transition of the three states is roughly as follows:

```
 [CLOSED] ---> tripped ----> [OPEN]<-------+
    ^ | ^
    | v
    + | detect fail
    | | v
    | cooling timeout |
    The timeout for cooling
    | v
    +-- detect succeed --<-[HALFOPEN]-->--+
```

### Trigger Strategies

Kitex provides three basic fuse triggering strategies by default:

- Number of consecutive errors reaches threshold (ConsecutiveTripFunc)

- Error count reaches threshold (ThresholdTripFunc)

- Error rate reaches the threshold (RateTripFunc)

Of course, you can write your own triggering strategy by implementing the TripFunc function.

Circuitbreaker will call TripFunc when Fail or Timeout happen to decide whether to trigger breaker.

### Cooling Strategy

After entering the OPEN state, the breaker will cool down for a period of time, the default is 10 seconds which is also configurable (with CoolingTimeout).

During this period, all IsAllowed() requests will be returned false.

Entering HALFOPEN when cooling is complete.

### Half-Open Strategy

During HALFOPEN, the breaker will let a request go every "interval", and after a "number" of consecutive successful requests, the breaker will become CLOSED; If any of them fail, it will become OPEN.

The process is a gradual-trial process.

Both the "interval" (DetectTimeout) and the "number" (DEFAULT_HALFOPEN_SUCCESSES) are configurable.

## Statistics

### Default Config

The breaker counts successes, failures and timeouts within a period of time window(default window size is 10 seconds).

The time window can be set by two parameters, but usually you can leave it alone.

### Statistics Implementation

The statistics will divide the time window into buckets, each bucket recording data for a fixed period of time.

For example, to count data within 10 seconds, you can spread the 10 second time period over 100 buckets, each bucket counting data within a 100ms time period.

The BucketTime and BucketNums in Options correspond to the time period of each bucket, and the number of buckets.

If BucketTime is set to 100ms, and BucketNums is set to 100, this corresponds to a 10 second time window.

### Jitter

As time moves, the oldest bucket in the window expires. The jitter occurs when the last bucket expires.

As an example:

- You divide 10 seconds into 10 buckets, bucket 0 corresponds to a time of [0S, 1S), bucket 1 corresponds to [1S, 2S), ... , and bucket 9 corresponds to [9S, 10S).

- At 10.1S, a Succ is executed, and the following operations occur within the circuitbreaker.
	
	- (1) detects that bucket 0 has expired and discards it; 
	- (2) creates a new bucket 10, corresponding to [10S, 11S); 
	- (3) puts that Succ into bucket 10.
	
- At 10.2S, you execute Successes() to query the number of successes in the window, then you get the actual statistics for [1S, 10.2S), not [0.2S, 10.2S).

Such jitter cannot be avoided if you use time-window-bucket statistics. A compromise approach is to increase the number of buckets, which can reduce the impact of jitter.

If 2000 buckets are divided, the impact of jitter on the overall data is at most 1/2000. In this package, the default number of buckets is 2000, the bucket time is 5ms, and the time window is 10S.

There were various technical solutions to avoid this problem, but they all introduce other problems, so if you have good ideas, please create a issue or PR.

