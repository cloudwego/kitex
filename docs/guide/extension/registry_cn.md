# 服务注册扩展

Kitex 支持自定义注册模块，使用者可自行扩展集成其他注册中心，该扩展定义在 pkg/registry 下。

## 扩展接口和 Info 定义
- 扩展接口

```go
// Registry is extension interface of service registry.
type Registry interface {
	Register(info *Info) error
	Deregister(info *Info) error
}
```

- Info 定义
Kitex 定义了部分注册信息，使用者也可以根据需要自行扩展注册信息到 Tags 中。
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

## 集成到 Kitex
通过 option 指定自己的注册模块和自定义的注册信息。注意注册需要服务信息，服务信息也是通过 option 指定。

- 指定服务信息

  option: `WithServerBasicInfo`

  ```go
  ebi := &rpcinfo.EndpointBasicInfo{
  		ServiceName: 'yourSerivceName',
  		Tags:        make(map[string]string),
  }
  ebi.Tags[idc] = "xxx"

  svr := xxxservice.NewServer(handler, server.WithServerBasicInfo(ebi))
  ```

- 指定自定义注册模块
  option: `WithRegistry`

  ```go
  svr := xxxservice.NewServer(handler, server.WithServerBasicInfo(ebi), server.WithRegistry(yourRegistry))
  ```
- 自定义 RegistyInfo 
  Kitex 默认赋值 ServiceName、Addr 和 PayloadCodec，若需要其他注册信息需要使用者自行注入。option:  `WithRegistryInfo`

  ```go
  svr := xxxservice.NewServer(handler, server.WithRegistry(yourRegistry), server.WithRegistryInfo(yourRegistryInfo))
  ```
