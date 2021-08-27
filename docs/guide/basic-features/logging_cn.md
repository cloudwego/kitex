# 日志

## pkg/klog

Kitex 在 pkg/klog 里定义了 `Logger`、`CtxLogger`、`FormatLogger` 等几个接口，并提供了一个 `FormatLogger` 的默认实现，可以通过 `klog.DefaultLogger()` 获取到其实例。

pkg/klog 同时也提供了若干全局函数，例如 `klog.Info`、`klog.Errorf` 等，用于调用默认 logger 的相应方法。

注意，由于默认 logger 底层使用标准库的 `log.Logger` 实现，其在日志里输出的调用位置依赖于设置的调用深度（call depth），因此封装 klog 提供的实现可能会导致日志内容里文件名和行数不准确。

## 注入自己的 logger 实现

client 和 server 都提供了一个 `WithLogger` 的选项，用于注入自定义的 logger 实现，随后中间件以及框架的其他部分可以用该 logger 来输出日志。

默认使用的是 `klog.DefaultLogger`。

## 在中间件里打日志

如果你实现了一个中间件，并期望可以使用框架提供的 logger 来输出日志，并兼容 `WithLogger` 选项注入的自定义 logger 实现，那么你可以用 `WithMiddlewareBuilder` 而不是 `WithMiddleware` 来注入你实现的中间件。

`WithMiddlewareBuilder` 注入的 `endpoint.MiddlewareBuilder` 被调用时，其类型为 `context.Context` 的参数会携带当前注入的 logger 实例，可以用 `ctx.Value(endpoint.CtxLoggerKey)` 来取出。

```go
func MyMiddlewareBuilder(mwCtx context.Context) endpoint.Middleware { // middleware builder

    logger := mwCtx.Value(endpoint.CtxLoggerKey).(klog.FormatLogger)  // get the logger

    return func(next endpoint.Endpoint) endpoint.Endpoint {           // middleware
        return func(ctx context.Context, request, response interface{}) error {

            err := next(ctx, request, response)

            logger.Debugf("This is a log message from MyMiddleware! err: %v", err)

            return err
        }
    }
}
```

## 重定向默认 logger 的输出

可以使用 `klog.SetOutput` 来重定向 klog 提供的默认 logger 的输出。

注意，该方法无法影响到使用 client 或者 server 的 `WithLogger` 选项注入的自定义 logger 实现。
