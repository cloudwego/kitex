
metainfo
========

该库提供了一种在 go 语言的 `context.Context` 中保存用于跨服务传递的元信息的统一接口。

**元信息**被设计为字符串的键值对，并且键是**大小写敏感的**。

根据数据传递的场景，元信息被分为两种类别，*transient* 和 *persistent* —— 前者只会传递一跳，从客户端传递到下游服务端，然后消失；后者需要在整个服务调用链上一直传递，直到被丢弃。

由于传递过程使用了 go 语言的 `context.Context`，因此，为了避免服务端的 `Context` 实例直接传递给用于调用下游服务的客户端时造成从更上游服务传递来的 transient 数据被继续透传，需要引入一个中间态：*transient-upstream*，以和当前服务自己设置的 transient 数据作区分。该类别仅用于实现 transient 的语义，在接口上，transient 和 transient-upstream 并没有区别。

该库被设计成针对 `context.Context` 进行操作的接口集合，而具体的元数据在网络上传输的形式和方式，应由支持该库的框架来实现。通常，终端用户不应该关注其具体传输形式，而应该仅依赖该库提供的抽象接口。

框架支持指南
------------

如果要在某个框架里引入 metainfo 并对框架的用户提供支持，需要满足如下的条件：

1. 框架使用的传输协议应该支持元信息的传递（例如 HTTP header、thrift 的 header transport 等）。
2. 当框架作为服务端接收到元信息后，需要将元信息添加到 `context.Context` 对象里。随后，在进入用户的代码逻辑（和其他可能要使用元信息的代码）之前，需要用 `metainfo.TransferForward` 转化客户端传递过来的 transient 信息为 transient-upstream。
3. 在框架作为客户端发起对服务端的请求，需要将元信息传递出去之前，需要调用一次 `metainfo.TransferForward` 将 `context.Context` 里的 transient-upstream 信息丢弃掉，使得 transient 信息符合“只传递一跳”的语义。

API 参考
-------

**注意**

1. 出于兼容性和普适性，元信息的形式为字符串的 key value 对。
2. 空串作为 key 或者 value 都是无效的。
3. 由于 context 的特性，程序对 metainfo 的增删改只会对拥有相同的 contetxt 或者其子 context 的代码可见。

**常量**

metainfo 包提供了几个常量字符串前缀，用于无 context（例如网络传输）的场景下标记元信息的类型。

典型的业务代码通常不需要用到这些前缀。支持 metainfo 的框架也可以自行选择在传输时用于区分元信息类别的方式。

- `PrefixPersistent`
- `PrefixTransient`
- `PrefixTransientUpstream`

**方法**

- `TransferForward(ctx context.Context) context.Context`
    - 向前传递，用于将上游传来的 transient 数据转化为 transient-upstream 数据，并过滤掉原有的 transient-upstream 数据。
- `GetValue(ctx context.Context, k string) (string, bool)`
    - 从 context 里获取指定 key 的 transient 数据（包括 transient-upstream 数据）。
- `GetAllValues(ctx context.Context) map[string]string`
    - 从 context 里获取所有 transient 数据（包括 transient-upstream 数据）。
- `WithValue(ctx context.Context, k string, v string) context.Context`
    - 向 context 里添加一个 transient 数据。
- `DelValue(ctx context.Context, k string) context.Context`
    - 从 context 里删除指定的 transient 数据。
- `GetPersistentValue(ctx context.Context, k string) (string, bool)`
    - 从 context 里获取指定 key 的 persistent 数据。
- `GetAllPersistentValues(ctx context.Context) map[string]string`
    - 从 context 里获取所有 persistent 数据。
- `WithPersistentValue(ctx context.Context, k string, v string) context.Context`
    - 向 context 里添加一个 persistent 数据。
- `DelPersistentValue(ctx context.Context, k string) context.Context`
    - 从 context 里删除指定的 persistent 数据。

