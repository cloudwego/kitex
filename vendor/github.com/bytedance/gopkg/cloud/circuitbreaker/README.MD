# circuitbreaker

## A brief introduction to circuit breaker
### What circuit breaker does
When making RPC calls, downstream services inevitably fail;

When a downstream service fails, if the upstream continues to make calls to it, it hinders the recovery of the downstream and wastes the resources of the upstream;

To solve this problem, you can set some dynamic switches to manually shut down the downstream calls when the downstream fails;

However, a better solution is to use circuit breakers to automate this problem.

Here is a more detailed [introduction to circuit breaker](https://msdn.microsoft.com/zh-cn/library/dn589784.aspx).

One of the better known circuit breakers is hystrix, and here is its [design document](https://github.com/Netflix/Hystrix/wiki).

### circuit breaker ideas
The idea of a circuit breaker is simple: restrict access to the downstream based on the success or failure of the RPC;

Usually there are three states: CLOSED, OPEN, HALFOPEN;

When the RPC is normal, it is CLOSED;

When the number of RPC failures increases, the circuit breaker is triggered and goes to OPEN;

After a certain cooling time after OPEN, the circuit breaker will become HALFOPEN;

HALFOPEN will do some strategic access to the downstream, and then decide whether to become CLOSED, or OPEN according to the result;

In summary, the three state transitions are roughly as follows:

<pre>
 [CLOSED] -->- tripped ----> [OPEN]&lt;-------+
    ^                          |           ^
    |                          v           |
    +                          |      detect fail
    |                          |           |
    |                    cooling timeout   |
    ^                          |           ^
    |                          v           |
    +--- detect succeed --&lt;-[HALFOPEN]-->--+
</pre>

## Use of this package

### Basic usage
This package divides the results of RPC calls into three categories: Succeed, Fail, Timeout, and maintains a count of all three within a certain time window;

Before each RPC, you should call IsAllowed() to decide whether to initiate the RPC;

and call Succeed(), Fail(), Timeout() for feedback after the call is completed, depending on the result;

The package also controls the number of concurrency, you must also call Done() after each RPC;

Here is an example:
<pre>
var p *Panel

func init() {
    var err error
    p, err = NewPanel(nil, Options{
    	CoolingTimeout: time.Minute,
    	DetectTimeout:  time.Minute,
    	ShouldTrip:     ThresholdTripFunc(100),
    })
    if err != nil {
    	panic(err)
    }
}

func DoRPC() error {
    key := "remote::rpc::method"
    if p.IsAllowed(key) == false {
        return Err("Not allowed by circuitbreaker")
    }

    err := doRPC()
    if err == nil {
        p.Succeed(key)
    } else if IsFailErr(err) {
        p.Fail(key)
    } else if IsTimeout(err) {
        p.Timeout(key)
    }
    return err
}

func main() {
    ...
    for ... {
        DoRPC()
    }
    p.Close()
}
</pre>

### circuit breaker Trigger strategies
This package provides three basic circuit breaker triggering strategies:
+ Number of consecutive failures reaches threshold (ExecutiveTripFunc)
+ Failure count reaches threshold (ThresholdTripFunc)
+ Failure rate reaches threshold (RateTripFunc)

Of course, you can write your own circuit breaker triggering strategy by implementing the TripFunc function;

Circuit breaker will call TripFunc each time Fail or Timeout to decide whether to trigger the circuit breaker;

### Circuit breaker cooling strategy
After entering the OPEN state, the circuit breaker will cool down for a period of time, the default is 10 seconds, but this parameter is configurable (CoolingTimeout);

During this period, all IsAllowed() requests will be returned false;

After cooling, HALFOPEN is entered;

### Half-open strategy
During HALFOPEN, the circuit breaker will let a request go every "while", and after a "number" of consecutive successful requests, the circuit breakerr will become CLOSED; if any of them fail, it will become OPEN;

This process is a gradual process of testing downstream, and opening up;

The above "timeout" (DetectTimeout) and "number" (DEFAULT_HALFOPEN_SUCCESSES) are both configurable;

### Concurrency control
The circuit breaker also performs concurrency control, with the parameter MaxConcurrency;

IsAllowed will return false when the maximum number of concurrency is reached;

### Statistics
##### Default parameter
The circuit breaker counts successes, failures and timeouts within a period of time window, the default window size is 10S;

The time window can be set with two parameters, but usually you don't need to care.

##### statistics method
The statistics method is to divide the time window into buckets, each bucket records data for a fixed period of time;

For example, if you want to count 10 seconds of data, you can divide the 10 second time period into 100 buckets, and each bucket will count 100ms of data;

The BucketTime and BucketNums in Options correspond to the time period maintained by each bucket and the number of buckets, respectively;

If BucketTime is set to 100ms and BucketNums is set to 100, it corresponds to a 10 second time window;

##### Jitter
As time moves, the oldest bucket in the window will expire, and when the last bucket expires, jitter will occur;

As an example:
+ you divide 10 seconds into 10 buckets, bucket 0 corresponds to the time [0S, 1S), bucket 1 corresponds to the time [1S, 2S), ... , barrel 9 corresponds to [9S, 10S);
+ At 10.1S, if Succ is executed once, the following operation occurs in the circuitbreaker;
+ (1) Bucket 0 is detected as expired and is discarded; (2) A new bucket 10 is created, corresponding to [10S, 11S); (3) The Succ is placed in bucket 10;
+ At 10.2S, you run Successes() to query the number of successes in the window, then you get the actual count of [1S, 10.2S) instead of [0.2S, 10.2S);

If you use the bucket counting method, such jitter is unavoidable, a compromise is to increase the number of buckets to reduce the impact of jitter;

If the number of buckets is divided into 2000, the impact of jitter on the overall data will be at most 1/2000;

In this package, the default number of buckets is also 100, the bucket time is 100ms, and the overall window is 10S;

There were several technical solutions to avoid this problem, but they all introduced more problems, if you have good ideas, please issue or PR.