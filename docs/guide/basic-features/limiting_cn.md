# 限流

限流是一种保护 server 的措施，防止上游某个 client 流量突增导致 server 端过载。

目前 Kitex 支持限制最大连接数和最大 QPS，在初始化 server 的时候，增加一个 Option，举例：

```go
import "github.com/cloudwego/kitex/pkg/limit"

func main() {
	svr := xxxservice.NewServer(handler, server.WithLimit(&limit.Option{MaxConnections: 10000, MaxQPS: 1000}))
  	svr.Run()
}
```

参数说明：

- `MaxConnections` 表示最大连接数

- `MaxQPS` 表示最大 QPS

- `UpdateControl` 提供动态修改限流阈值的能力，举例：

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

## 实现

分别使用 ConcurrencyLimiter 和 RateLimiter 对最大连接数和最大 QPS 进行限流。

- ConcurrencyLimiter：简单的计数器；
- RateLimiter：这里的限流算法采用了 " 滑动窗口法 "，时间窗口为 1s，再把时间窗口划分成 10 个小窗口，每个小窗口为 100ms。

## 监控

限流定义了 `LimitReporter` 接口，用于限流状态监控，例如当前连接数过多、QPS 过大等。

如有需求，用户需要自行实现该接口，并通过 `WithLimitReporter` 注入。

```go
// LimitReporter is the interface define to report(metric or print log) when limit happen
type LimitReporter interface {
    ConnOverloadReport()
    QPSOverloadReport()
}
```
