# 诊断模块扩展

诊断模块是用于将服务中的信息可视化出来，便于问题排查，确认服务状态。Kitex 定义了接口用来注册诊断 func，扩展者可实现该接口来呈现诊断信息。呈现的方式如：输出日志、debug端口查询展示。Kitex 开源版本暂未提供默认扩展，但默认注册了部分可用于诊断的信息，扩展者也可以注册更多的信息用于问题的排查。

## 扩展接口

```go
// ProbeName is the name of probe.
type ProbeName string

// ProbeFunc is used to get probe data, it is usually a data dump func.
type ProbeFunc func() interface{}

// Service is the interface for debug service.
type Service interface {
   // RegisterProbeFunc is used to register ProbeFunc with probe name
   // ProbeFunc is usually a dump func that to dump info to do problem diagnosis,
   // eg: CBSuite.Dump(), s.RegisterProbeFunc(CircuitInfoKey, cbs.Dump)
   RegisterProbeFunc(ProbeName, ProbeFunc)
}
```

### 注册诊断信息

```go
// new diagnosisi service
var ds diagnosis.service = NewYourService()

// eg: register dump func to get discovery instances. 
ds.RegisterProbeFunc("instances", dr.Dump)

// eg: wrap the config data as probe func, register func to get config info. 
ds.RegisterProbeFunc("config_info", diagnosis.WrapAsProbeFunc(config))

```

## Kitex 默认注册的诊断信息

Kitex 默认注册了部分诊断信息用于问题排查，具体如下：

```go
const (
	// Common
	ChangeEventsKey ProbeName = "events"
	ServiceInfoKey  ProbeName = "service_info"
	OptionsKey      ProbeName = "options"

	// Client
	DestServiceKey ProbeName = "dest_service"
	ConnPoolKey    ProbeName = "conn_pool"
	RetryPolicyKey ProbeName = "retry_policy"
)
```



## 集成到 Kitex

通过option指定自己的诊断服务，option: `WithDiagnosisService`。

```go
// server side
svr := stservice.NewServer(handler, server.WithDiagnosisService(yourDiagnosisService))

// client side
cli, err := xxxservice.NewClient(targetService, client.WithDiagnosisService(yourDiagnosisService))
```

