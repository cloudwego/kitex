# Extension of Service Registry

Kitex supports user-defined registration module. Users can extend and integrate other registration centers by themselves. This extension is defined under pkg/registry.

## Extension API and Definition of Info Struct
- Extension Api

```go
// Registry is extension interface of service registry.
type Registry interface {
	Register(info *Info) error
	Deregister(info *Info) error
}
```

- Definition of Info Struct
Kitex defines some registration information. Users can also expand the registration information into tags as needed.
```go
// Info is used for registry.
// The fields are just suggested, which is used depends on design.
type Info struct {
	// ServiceName will be set in kitex by default
	ServiceName string
	// Addr will be set in kitex by default
	Addr net.Addr
	// PayloadCodec will be set in kitex by default, like thrift, protobuf
	PayloadCodec string

	Weight        int
	StartTime     time.Time
	WarmUp        time.Duration

	// extend other infos with Tags.
	Tags map[string]string
}
```

## Integrate into Kitex
Specify your own registration module and customized registration information through `option`. Note that registration requires service information, which is also specified through option.

- Specify Server Info

  option: `WithServerBasicInfo`

  ```go
  ebi := &rpcinfo.EndpointBasicInfo{
  		ServiceName: 'yourSerivceName',
  		Tags:        make(map[string]string),
  }
  ebi.Tags[idc] = "xxx"
  
  svr := xxxservice.NewServer(handler, server.WithServerBasicInfo(ebi))
  ```

- Specify Custom Registion module
  
  option: `WithRegistry`
  
  ```go
  svr := xxxservice.NewServer(handler, server.WithServerBasicInfo(ebi), server.WithRegistry(yourRegistry))
  ```
  
- Custom RegistyInfo 
  
  Kitex sets ServiceName, Addr and PayloadCodec by default. If other registration information is required, you need to inject it by yourself. option: `WithRegistryInfo`.
  
  ```go
  svr := xxxservice.NewServer(handler, server.WithRegistry(yourRegistry), server.WithRegistryInfo(yourRegistryInfo))
  ```
  
  