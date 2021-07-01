# 异常说明

## 异常类型

Kitex框架定义在 `github.com/cloudwego/kitex/pkg/kerrors` 下

### internal exception

`ErrInternalException`, 框架内部发生的错误，具体包括以下几种：

1. `ErrNotSupported`, `"operation not supported"`，进行了尚不支持的操作
2. `ErrNoResolver`, `"no resolver available"`，没有可用的 resolver
3. `ErrNoDestService`, `"no dest service"`，没有指定目标 service
4. `ErrNoDestAddress`, `"no dest address"`，没有指定目标地址
5. `ErrNoConnection`, `"no connection available"`，当前没有可用连接
6. `ErrNoIvkRequest`, `"invoker request not set"`，invoker 模式下调用时为设置 request

### service discovery error

`ErrServiceDiscovery`, 服务发现错误，具体错误见报错信息

### get connection error

`ErrGetConnection`, 获取连接错误，具体错误见报错信息

### loadbalance error

`ErrLoadbalance`, 复杂均衡错误

### no more instances to retry

`ErrNoMoreInstance`, 没有可供重试的示例，上一次调用的错误见报错信息

### rpc timeout

`ErrRPCTimeout`, RPC 调用超时，具体错误见报错信息

### request forbidden

`ErrACL`, 调用被拒绝，具体错误见报错信息

### forbidden by circuitbreaker

`ErrCircuitBreak`, 发生熔断后请求被拒绝，通常包含两种错误：

1. `ErrServiceCircuitBreak`, `"service circuitbreak"`，发生服务级别熔断后请求被拒绝
2. `ErrInstanceCircuitBreak`, `"instance circuitbreak"`，发生实例级别熔断后请求被拒绝

### remote or network error

`ErrRemoteOrNetwork`, 远端服务发生错误，或者出现网络错误。具体错误见报错信息

当带有`[remote]`字样时，代表此错误为远端返回

### request over limit

`ErrOverlimit`, 过载保护错误。通常包括以下两种错误：

1. `ErrConnOverLimit`, `"too many connections"`，连接过载，建立的连接超过限制
2. `ErrQPSOverLimit`, `"request too frequent"`，请求过载，请求数超过限制

### panic

`ErrPanic`, 服务发生 panic 。

当带有`[happened in biz handler]`字样时，代表 panic 发生在服务端 handler 中，此时错误信息中会带上堆栈。

### biz error

`ErrBiz`, 服务端 handler 返回的错误。

### retry error

`ErrRetry`, 重试时发生错误，具体错误见报错信息。

### THRIFT 错误码 

该类别对应 thrift 框架原生的 Application Exception 错误，通常，这些错误会被 Kitex 框架包装成`remote or network error`。

| 错误码 | 名称                                 | 含义           |
| ------ | ------------------------------------ | -------------- |
| 0      | UnknwonApllicationException          | 未知错误       |
| 1      | UnknownMethod                        | 未知方法       |
| 2      | InValidMessageTypeException          | 无效的消息类型 |
| 3      | WrongMethodName                      | 错误的方法名字 |
| 4      | BadSequenceID                        | 错误的包序号   |
| 5      | MissingResult                        | 返回结果缺失   |
| 6      | InternalErrorupstream request failed | 内部错误       |
| 7      | ProtocolError                        | 协议错误       |

## 异常判断

### 判断是否是 Kitex 的错误

可以通过 `kerrors` 包提供的 `IsKitexError` 直接进行判断

```go
import "github.com/cloudwego/kitex/pkg/kerrors"
...
isKitexErr := kerrors.IsKitexError(kerrors.ErrInternalException) // 返回 true
```

### 判断具体的错误类型

可以通过`errors.Is`进行判断，其中详细错误可以通过详细错误判断，如：

```go
import "errors"
import "github.com/cloudwego/kitex/client"
import "github.com/cloudwego/kitex/pkg/kerrors"
...
_, err := echo.NewClient("echo", client.WithResolver(nil)) // 返回 kerrors.ErrNoResolver
...
isKitexErr := errors.Is(err, kerrors.ErrNoResolver) // 返回 true
```

也可以通过基本错误进行判断，如：

```go
import "errors"
import "github.com/cloudwego/kitex/client"
import "github.com/cloudwego/kitex/pkg/kerrors"
...
_, err := echo.NewClient("echo", client.WithResolver(nil)) // 返回 kerrors.ErrNoResolver
...
isKitexErr := errors.Is(err, kerrors.ErrInternalException) // 返回 true
```

特别的，`timeout` 错误可以通过 `kerrors` 包提供的 `IsTimeoutError` 进行判断

### 获取更详细的错误信息

`kerrors` 中所有的具体错误类型都是 `kerrors` 包下的 `DetailedError`，故而可以通过 `errors.As` 获取到实际的 `DetailedError`，如：

```go
import "errors"
import "github.com/cloudwego/kitex/client"
import "github.com/cloudwego/kitex/pkg/kerrors"
...
_, err := echo.NewClient("echo", client.WithResolver(nil)) // 返回 kerrors.ErrNoResolver
...
var de *kerrors.DetailedError
ok := errors.As(err, &ke) // 返回 true
if de.ErrorType() == kerrors.ErrInternalException {} // 返回 true
```

`DetailedError` 提供了下述方法用于获取更详细的信息：
1. `ErrorType() error` ，用于获取基本错误类型
2. `Stack() string` ，用于获取堆栈信息（目前仅 `ErrPanic` 会带上）