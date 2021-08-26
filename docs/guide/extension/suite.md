# Suite Extensions - Encapsulating Custom Governance Modules

Suite is a high-level abstraction of extensions, a combination and encapsulation of Option and Middleware.

As mentioned in the middleware extensions document, there are two principles should be remembered in extensions:

1. Middleware and Suit are only allowed to be set before initializing Server and Client, do not allow modified dynamically.
2. Behind override ahead.

These tow principle is also valid for Suite.

Suite is defined as follows:

```go
type Suite interface {
    Options() []Option
}
```

// TODO: Add example.

Both Server side and Client side use the `WithSuite` method to enable new Suite.

When initializing Server and Client, Suite is setup in DFS(Deep First Search) way.

For example, if we have the following code:

```go
type s1 struct {
    timeout time.Duration 
}

func (s s1) Options() []client.Option {
    return []client.Option { client.WithRPCTimeout(s.timeout)}
}

type s2 struct {
}

func (s2) Options() []client.Option {
    return []client.Option{client.WithSuite(s1{timeout:1*time.Second}), client.WithRPCTimeout(2*time.Second)}
}
```

Then if we use `client.WithSuite(s2{}), client.WithRPCTimeout(3*time.Second)`, it will execute `client.WithSuite(s1{})` first, followed by `client. WithRPCTimeout(1*time.Second)`, followed by `client.WithRPCTimeout(2*time.Second)`, and finally `client.WithRPCTimeout(3*time.Second)`. After this initialization, the value of RPCTimeout will be set to 3s (see the principle described at the beginning).

# Summary

Suite is a higher-level combination and encapsulation, and it is recommended that third-party developers provide Kitex extensions based on Suite. Suite allows dynamically injecting values at creation time, or dynamically specifying values in its own middleware at runtime, making it easier for users and third-party developers to use and develop without relying on global variables, and making it possible to use different configurations for each client.
