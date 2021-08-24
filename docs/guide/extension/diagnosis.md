# Extension of Diagosis

The `Diagosis` module is used to visualize the information in the service to facilitate troubleshooting and confirm the service status. Kitex defines an interface to register the `diagnostic func,` and developers can implement this interface to present diagnostic information. Presentation way like: output with log and query display with debug port. The open source version of Kitex does not provide a default extension temporarily, but some information that can be used for diagnosis is registered by default. The developers can also register more information for troubleshooting.

## Extension API

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

### Register diagnostic information

```go
// new diagnosisi service
var ds diagnosis.service = NewYourService()

// eg: register dump func to get discovery instances. 
ds.RegisterProbeFunc("instances", dr.Dump)

// eg: wrap the config data as probe func, register func to get config info. 
ds.RegisterProbeFunc("config_info", diagnosis.WrapAsProbeFunc(config))

```

## Default registered diagnostic information in Kitex

Kitex registers some diagnostic information for troubleshooting by default, as follows:

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



## Integrate into Kitex

Specify your own diagnostic service through option: `WithDiagnosisService`.

```go
// server side
svr := stservice.NewServer(handler, server.WithDiagnosisService(yourDiagnosisService))

// client side
cli, err := xxxservice.NewClient(targetService, client.WithDiagnosisService(yourDiagnosisService))
```

