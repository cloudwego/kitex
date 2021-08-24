# Rate Limiting

Rate limiting is an imperative technique to protect server, which prevents server from overloaded by sudden traffic increase from a client.

Kitex supports max connections limit and max QPS limit. You may add an `Option` during the `server` initialization, for example:

```go
import "github.com/cloudwego/kitex/pkg/limit"

func main() {
	svr := xxxservice.NewServer(handler, server.WithLimit(&limit.Option{MaxConnections: 10000, MaxQPS: 1000}))
  _ = svr.Run()
}
```

Parameter description：

- `MaxConnections`: max connections

- `MaxQPS`: max QPS (Queries Per Second)

- `UpdateControl`: provide the ability to modify the rate limit threshold dynamically, for example: 

  ```go
  import "github.com/cloudwego/kitex/pkg/limit"
  
  // define your limiter updater to update limit threshold
  type MyLimiterUpdater struct {
  	updater limit.Updater
  }
  
  func (lu *MyLimiterUpdater) YourChange() {
  	// your logic: set new option as needed
  	newOpt := &limit.Option{
  		MaxConnections: 20000,
  		MaxQPS:         2000,
  	}
  	// update limit config
  	isUpdated := lu.updater.UpdateLimit(newOpt)
  	// your logic
  }
  
  func (lu *MyLimiterUpdater) UpdateControl(u limit.Updater) {
  	lu.updater = u
  }
  
  //--- init server ---
  var lu  = MyLimiterUpdater{}
  svr := xxxservice.NewServer(handler, server.WithLimit(&limit.Option{MaxConnections: 10000, MaxQPS: 1000, UpdateControl: lu.UpdateControl}))
  ```

## Implementation

ConcurrencyLimiter and RateLimiter are used respectively to limit max connection and max QPS.

- ConcurrencyLimiter：a simple counter；
- RateLimiter：sliding window algorithm is used here. Time window is 1s, which is split into 10 small windows with each 100ms.

## Monitoring

Rate limiting defines the `LimitReporter` interface, which is used by rate limiting status monitoring, e.g. connection overloaded, QPS overloaded, etc.

Users may implement this interface and inject this implementation by `WithLimitReporter` if required.

```go
// LimitReporter is the interface define to report(metric or print log) when limit happen
type LimitReporter interface {
    ConnOverloadReport()
    QPSOverloadReport()
}
```
