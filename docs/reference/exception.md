# Exception Instruction

## Exception Type

Defined in `github.com/cloudwego/kitex/pkg/kerrors`

### internal exception

`ErrInternalException`, framework internal error, it cloud be:

1. `ErrNotSupported`, `"operation not supported"`, have some operation not supported yet
2. `ErrNoResolver`, `"no resolver available"`, no resolver is available
3. `ErrNoDestService`, `"no dest service"`, target service is not specified
4. `ErrNoDestAddress`, `"no dest address"`, target address is not specified
5. `ErrNoConnection`, `"no connection available"`, not connection is available
6. `ErrNoIvkRequest`, `"invoker request not set"`, request is not set in invoker mode

### service discovery error

`ErrServiceDiscovery`, service discovery error, see error message for detail.

### get connection error

`ErrGetConnection`, get connection error, see error message for detail.

### loadbalance error

`ErrLoadbalance`, loadbalance error, see error message for detail.

### no more instances to retry

`ErrNoMoreInstance`, no more instance to retry, see last call error message for detail.

### rpc timeout

`ErrRPCTimeout`, RPC timeout, see error message for detail.

### request forbidden

`ErrACL`, RPC is rejected by ACL, see error message for detail.

### forbidden by circuitbreaker

`ErrCircuitBreak`, request is circuitbreaked, it could be two type of circuitbreak:

1. `ErrServiceCircuitBreak`, `"service circuitbreak"`, service level circuitbreak encountered, request is rejected.
2. `ErrInstanceCircuitBreak`, `"instance circuitbreak"`, instance level circuitbreak encountered, request is rejected.

### remote or network error

`ErrRemoteOrNetwork`, remote server error, or network error, see error message for detail.

`[remote]` indicates the error is returned by server

### request over limit

`ErrOverlimit`, overload protection error, it cloud be:

1. `ErrConnOverLimit`, `"too many connections"`, connection overload, connection number is over limit
2. `ErrQPSOverLimit`, `"request too frequent"`, concurrent request overload, concurrent request number is over limit

### panic

`ErrPanic`, panic detected.

`[happened in biz handler]` indicated panic happened in server handler, usually call stack will attached to error message.

### biz error

`ErrBiz`, server handler error.

### retry error

`ErrRetry`, retry error, see error message for detail.

### THRIFT Error Code

These error is thrift Application Exception, usually these error will be wrapped to `remote or network error`.

| Code | Name                        | Meaning              |
| ---- | --------------------------- | -------------------- |
| 0    | UnknownApplicationException | Unknown Error        |
| 1    | UnknownMethod               | Unknown Function     |
| 2    | InValidMessageTypeException | Invalid Message Type |
| 3    | WrongMethodName             | Wrong Method Name    |
| 4    | BadSequenceID               | Bad Sequence ID      |
| 5    | MissingResult               | Result is missing    |
| 6    | InternalError               | Internal Error       |
| 7    | ProtocolError               | Protocol Error       |

## Exception Check

### Check whether a Kitex error

Use  `IsKitexError` in `kerrors` package

```go
import "github.com/cloudwego/kitex/pkg/kerrors"
...
isKitexErr := kerrors.IsKitexError(kerrors.ErrInternalException) // return true
```

### Check Specified Error type

Use `errors.Is`, detailed error cloud use to check detailed error:

```go
import "errors"
import "github.com/cloudwego/kitex/client"
import "github.com/cloudwego/kitex/pkg/kerrors"
...
_, err := echo.NewClient("echo", client.WithResolver(nil)) // return kerrors.ErrNoResolver
...
isKitexErr := errors.Is(err, kerrors.ErrNoResolver) // return true
```

detailed error is also a basic error:

```go
import "errors"
import "github.com/cloudwego/kitex/client"
import "github.com/cloudwego/kitex/pkg/kerrors"
...
_, err := echo.NewClient("echo", client.WithResolver(nil)) // return kerrors.ErrNoResolver
...
isKitexErr := errors.Is(err, kerrors.ErrInternalException) // return true
```

Specially, you can use `IsTimeoutError` in `kerrors` to check whether it is a `timeout` error

### Get Detailed Error Message

All detailed errors is defined by `DetailedError` in `kerrors`, so you can use `errors.As` to get specified `DetailedError`, like:

```go
import "errors"
import "github.com/cloudwego/kitex/client"
import "github.com/cloudwego/kitex/pkg/kerrors"
...
_, err := echo.NewClient("echo", client.WithResolver(nil)) // return kerrors.ErrNoResolver
...
var de *kerrors.DetailedError
ok := errors.As(err, &ke) // return true
if de.ErrorType() == kerrors.ErrInternalException {} // return true
```

`DetailedError` provide following functions to get detail message.
1. `ErrorType() error`, used to get basic error type
2. `Stack() string`, used to get stack (for now only works for `ErrPanic`)
