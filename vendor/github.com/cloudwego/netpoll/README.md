# CloudWeGo-Netpoll

[中文](README_CN.md)

[![Release](https://img.shields.io/github/v/release/cloudwego/netpoll)](https://github.com/cloudwego/netpoll/releases)
[![WebSite](https://img.shields.io/website?up_message=cloudwego&url=https%3A%2F%2Fwww.cloudwego.io%2F)](https://www.cloudwego.io/)
[![License](https://img.shields.io/github/license/cloudwego/netpoll)](https://github.com/cloudwego/netpoll/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudwego/netpoll)](https://goreportcard.com/report/github.com/cloudwego/netpoll)
[![OpenIssue](https://img.shields.io/github/issues/cloudwego/netpoll)](https://github.com/cloudwego/netpoll/issues)
[![ClosedIssue](https://img.shields.io/github/issues-closed/cloudwego/netpoll)](https://github.com/cloudwego/netpoll/issues?q=is%3Aissue+is%3Aclosed)
![Stars](https://img.shields.io/github/stars/cloudwego/netpoll)
![Forks](https://img.shields.io/github/forks/cloudwego/netpoll)

## Introduction

[Netpoll][Netpoll] is a high-performance non-blocking I/O networking framework, which
focused on RPC scenarios, developed by [ByteDance][ByteDance].

RPC is usually heavy on processing logic and therefore cannot handle I/O serially. But Go's standard
library [net][net] is designed for blocking I/O APIs, so that the RPC framework can
only follow the One Conn One Goroutine design. It will waste a lot of cost for context switching, due to a large number
of goroutines under high concurrency. Besides, [net.Conn][net.Conn] has
no API to check Alive, so it is difficult to make an efficient connection pool for RPC framework, because there may be a
large number of failed connections in the pool.

On the other hand, the open source community currently lacks Go network libraries that focus on RPC scenarios. Similar
repositories such as: [evio][evio], [gnet][gnet], etc., are all
focus on scenarios like [Redis][Redis], [HAProxy][HAProxy].

But now, [Netpoll][Netpoll] was born and solved the above problems. It draws inspiration
from the design of [evio][evio] and [netty][netty], has
excellent [Performance](#performance), and is more suitable for microservice architecture.
Also [Netpoll][Netpoll] provides a number of [Features](#features), and it is recommended
to replace [net][net] in some RPC scenarios.

We developed the RPC framework [Kitex][Kitex] and HTTP
framework [Hertz][Hertz] (coming soon) based
on [Netpoll][Netpoll], both with industry-leading performance.

[Examples][netpoll-examples] show how to build RPC client and server
using [Netpoll][Netpoll].

For more information, please refer to [Document](#document).

## Features

* **Already**
    - [LinkBuffer][LinkBuffer] provides nocopy API for streaming reading and writing
    - [gopool][gopool] provides high-performance goroutine pool
    - [mcache][mcache] provides efficient memory reuse
    - `IsActive` supports checking whether the connection is alive
    - `Dialer` supports building clients
    - `EventLoop` supports building a server
    - TCP, Unix Domain Socket
    - Linux, macOS (operating system)

* **Future**
    - [multisyscall][multisyscall] supports batch system calls
    - [io_uring][io_uring]
    - Shared Memory IPC
    - Serial scheduling I/O, suitable for pure computing
    - TLS
    - UDP

* **Unsupported**
    - Windows (operating system)

## Performance

Benchmark should meet the requirements of industrial use. 
In the RPC scenario, concurrency and timeout are necessary support items.

We provide the [netpoll-benchmark][netpoll-benchmark] project to track and compare 
the performance of [Netpoll][Netpoll] and other frameworks under different conditions for reference.

More benchmarks reference [kitex-benchmark][kitex-benchmark] and [hertz-benchmark][hertz-benchmark] (open source soon).

## Reference

* [Official Website](https://www.cloudwego.io)
* [Getting Started](docs/guide/guide_en.md)
* [Design](docs/reference/design_en.md)
* [Why DATA RACE](docs/reference/explain.md)

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
