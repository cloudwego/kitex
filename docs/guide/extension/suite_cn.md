# Suite 扩展

## 封装自定义治理模块

Suite（套件）是一种对于扩展的高级抽象，可以理解为是对于 Option 和 Middleware 的组合和封装。

在 middleware 扩展一文中我们有说到，在扩展过程中，要记得两点原则：

1. 中间件和套件都只允许在初始化 Server、Client 的时候设置，不允许动态修改。
2. 后设置的会覆盖先设置的。

这个原则针对 Suite 也是一样有效的。

Suite 的定义如下：

```go
type Suite interface {
    Options() []Option
}
```

这也是为什么说，Suite 是对于 Option 和 Middleware（通过 Option 设置）的组合和封装。

// TODO: 增加示例。

Server 端和 Client 端都是通过 `WithSuite` 这个方法来启用新的套件。

在初始化 Server 和 Client 的时候，Suite 是采用 DFS(Deep First Search) 方式进行设置。

举个例子，假如我有以下代码：

```go
type s1 struct {
    timeout time.Duration
}

func (s s1) Options() []client.Option {
    return []client.Option{client.WithRPCTimeout(s.timeout)}
}

type s2 struct {
}

func (s2) Options() []client.Option {
    return []client.Option{client.WithSuite(s1{timeout:1*time.Second}), client.WithRPCTimeout(2*time.Second)}
}
```

那么如果我在创建 client 时传入 `client.WithSuite(s2{}), client.WithRPCTimeout(3*time.Second)`，在初始化的时候，会先执行到 `client.WithSuite(s1{})`，然后是 `client.WithRPCTimeout(1*time.Second)`，接着是 `client.WithRPCTimeout(2*time.Second)`，最后是 `client.WithRPCTimeout(3*time.Second)`。这样初始化之后，RPCTimeout 的值会被设定为 3s（参见开头所说的原则）。

## 总结

Suite 是一种更高层次的组合和封装，更加推荐第三方开发者能够基于 Suite 对外提供 Kitex 的扩展，Suite 可以允许在创建的时候，动态地去注入一些值，或者在运行时动态地根据自身的某些值去指定自己的 middleware 中的值，这使得用户的使用以及第三方开发者的开发都更加地方便，无需再依赖全局变量，也使得每个 client 使用不同的配置成为可能。
