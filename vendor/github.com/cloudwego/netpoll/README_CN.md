# CloudWeGo-Netpoll

[English](README.md)

[![Release](https://img.shields.io/github/v/release/cloudwego/netpoll)](https://github.com/cloudwego/netpoll/releases)
[![WebSite](https://img.shields.io/website?up_message=cloudwego&url=https%3A%2F%2Fwww.cloudwego.io%2F)](https://www.cloudwego.io/)
[![License](https://img.shields.io/github/license/cloudwego/netpoll)](https://github.com/cloudwego/netpoll/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudwego/netpoll)](https://goreportcard.com/report/github.com/cloudwego/netpoll)
[![OpenIssue](https://img.shields.io/github/issues/cloudwego/netpoll)](https://github.com/cloudwego/netpoll/issues)
[![ClosedIssue](https://img.shields.io/github/issues-closed/cloudwego/netpoll)](https://github.com/cloudwego/netpoll/issues?q=is%3Aissue+is%3Aclosed)
![Stars](https://img.shields.io/github/stars/cloudwego/netpoll)
![Forks](https://img.shields.io/github/forks/cloudwego/netpoll)

## 简介

[Netpoll][Netpoll] 是由 [字节跳动][ByteDance] 开发的高性能 NIO(Non-blocking I/O)
网络库，专注于 RPC 场景。

RPC 通常有较重的处理逻辑，因此无法串行处理 I/O。而 Go 的标准库 [net][net] 设计了 BIO(Blocking I/O) 模式的
API，使得 RPC 框架设计上只能为每个连接都分配一个 goroutine。 这在高并发下，会产生大量的
goroutine，大幅增加调度开销。此外，[net.Conn][net.Conn] 没有提供检查连接活性的 API，因此 RPC
框架很难设计出高效的连接池，池中的失效连接无法及时清理。

另一方面，开源社区目前缺少专注于 RPC 方案的 Go 网络库。类似的项目如：[evio][evio]
, [gnet][gnet] 等，均面向 [Redis][Redis], [HAProxy][HAProxy] 这样的场景。

因此 [Netpoll][Netpoll] 应运而生，它借鉴了 [evio][evio]
和 [netty][netty] 的优秀设计，具有出色的 [性能](#性能)，更适用于微服务架构。
同时，[Netpoll][Netpoll] 还提供了一些 [特性](#特性)，推荐在 RPC 设计中替代
[net][net] 。

基于 [Netpoll][Netpoll] 开发的 RPC 框架 [Kitex][Kitex] 和 HTTP
框架 [Hertz][Hertz] (即将开源)，性能均业界领先。

[范例][netpoll-examples] 展示了如何使用 [Netpoll][Netpoll]
构建 RPC Client 和 Server。

更多信息请参阅 [文档](#文档)。

## 特性

* **已经支持**
    - [LinkBuffer][LinkBuffer] 提供可以流式读写的 nocopy API
    - [gopool][gopool] 提供高性能的 goroutine 池
    - [mcache][mcache] 提供高效的内存复用
    - `IsActive` 支持检查连接是否存活
    - `Dialer` 支持构建 client
    - `EventLoop` 支持构建 server
    - 支持 TCP，Unix Domain Socket
    - 支持 Linux，macOS（操作系统）

* **即将开源**
    - [multisyscall][multisyscall] 支持批量系统调用
    - [io_uring][io_uring]
    - Shared Memory IPC
    - 串行调度 I/O，适用于纯计算
    - 支持 TLS
    - 支持 UDP

* **不被支持**
    - Windows（操作系统）

## 性能

性能测试应满足工业级使用要求，在 RPC 场景下，并发请求、等待超时是必要的支持项。

我们提供了 [netpoll-benchmark][netpoll-benchmark] 项目用来长期追踪和比较 [Netpoll][Netpoll] 与其他框架在不同情况下的性能数据以供参考。

更多测试参考 [kitex-benchmark][kitex-benchmark] 和 [hertz-benchmark][hertz-benchmark] (即将开源)

## 参考

* [官方网站](https://www.cloudwego.io)
* [使用文档](docs/guide/guide_cn.md)
* [设计文档](docs/reference/design_cn.md)
* [DATA RACE 说明](docs/reference/explain.md)

[Netpoll]: https://github.com/cloudwego/netpoll
[net]: https://github.com/golang/go/tree/master/src/net
[net.Conn]: https://github.com/golang/go/blob/master/src/net/net.go
[evio]: https://github.com/tidwall/evio
[gnet]: https://github.com/panjf2000/gnet
[netty]: https://github.com/netty/netty
[Kitex]: https://github.com/cloudwego/kitex
[Hertz]: https://github.com/cloudwego/hertz

[netpoll-benchmark]: https://github.com/cloudwego/netpoll-benchmark
[kitex-benchmark]: https://github.com/cloudwego/kitex-benchmark
[hertz-benchmark]: https://github.com/cloudwego/hertz-benchmark
[netpoll-examples]:https://github.com/cloudwego/netpoll-examples

[ByteDance]: https://www.bytedance.com
[Redis]: https://redis.io
[HAProxy]: http://www.haproxy.org

[LinkBuffer]: nocopy_linkbuffer.go
[gopool]: https://github.com/bytedance/gopkg/tree/develop/util/gopool
[mcache]: https://github.com/bytedance/gopkg/tree/develop/lang/mcache
[multisyscall]: https://github.com/cloudwego/multisyscall
[io_uring]: https://github.com/axboe/liburing
