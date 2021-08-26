# Middleware Extensions

## Introduction

Middleware is the major method to extend the Kitex framework. Most of the Kitex-based extensions and secondary development features are based on middleware.

Before extending, it is important to remember two principles:

1. Middleware and Suit are only allowed to be set before initializing Server and Client, do not allow modified dynamically.
2. Behind override ahead.

Middleware is defined in `pkg/endpoint/endpoint.go`, the two major types are:

1. `Endpoint` is a function that accepts ctx, req, resp and returns err, see the example below.
2. `Middleware` (aka MW) is also a function that receives and returns an `Endpoint`. 3.

In fact, a middleware is a function whose input and output are both `Endpoint`, which ensures the transparency to the application, and the application itself does not need to know whether it is decorated by the middleware. Due to this feature, middleware can be nested.

Middleware should be used in series, by calling the next, you can get the response (if any) and err returned by the latter middleware, and then process accordingly and return the err to the former middleware (be sure to check the err of next function returned, do not swallow the err) or set the response.

## Client-side Middleware

There are two ways to add client-side middleware:

1. `client.WithMiddleware` adds a middleware to the current client, executes before all the rest of the middleware.
2. `client.WithInstanceMW` adds a middleware to the current client and executes after service discovery and load balancing (if `Proxy` is used, it will not be called).

Note that the above functions should all be passed as `Option`s when creating the client. 

The order of client middleware calls:
1. the middleware set by `client.WithMiddleware`
2. ACLMiddleware
3. (ResolveMW + client.WithInstanceMW + PoolMW / DialerMW) / ProxyMW
4. IOErrorHandleMW

The order in which the calls are returned is reversed.

The order of all middleware calls on the client side can be seen in `client/client.go`.

## Server-side Middleware

The server-side middleware is different from the client-side.

You can add server-side middleware via `server.WithMiddleware`, and passing `Option` when creating the server.

The order of server-side middleware calls can be found in `server/server.go`.

## Example

You can see how to use the middleware in the following example.

If you have a requirement to print out the request and the response, we can write the following MW:

```go
func PrintRequestResponseMW(next endpoint.Endpoint) endpoint.Endpoint {
    return func(ctx context.Context, request, response interface{}) error {
        fmt.Printf("request: %v\n", request)
        err := next(ctx, request, response)
        fmt.Printf("response: %v", response)
        return err
    }
}
```

Assuming we are at Server side, we can use `server.WithMiddleware(PrintRequestResponseMW)` to use this MW.

**The above scenario is only for example, not for production, there will be performance issues. **

## Attention

If RPCInfo is used in a custom middleware, please pay attention to that RPCInfo will be recycled after the rpc is finished. If you start a goroutine in the middleware to modify RPCInfo, there will have some problems.
