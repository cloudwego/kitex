# CloudWeGo-Kitex
English | [中文](README_cn.md)

Kitex [kaɪt'eks] is a **high-performance** and **strong-extensibility** Golang RPC framework that helps developers build microservices. If the performance and extensibility are the main concerns when you develop microservices, Kitex can be a good choice.

## Basic Features

- **High Performance**

Kitex integrates [Netpoll](https://github.com/cloudwego/netpoll), a high-performance network library, which offers significant performance advantage over [go net](https://pkg.go.dev/net).

- **Extensibility**

Kitex provides many interfaces with default implementation for users to customize. You can extend or inject them into Kitex to fulfill your needs (please refer to the framework extension section below).

- **Multi-message Protocol**

Kitex is designed to be extensible to support multiple RPC messaging protocols. The initial release contains support for **Thrift**, **Kitex Protobuf** and **gRPC**, in which Kitex Protobuf is a Kitex custom Protobuf messaging protocol with a protocol format similar to Thrift. Kitex also supports developers extending their own messaging protocols.

- **Multi-transport Protocol**

For service governance, Kitex supports **TTHeader** and **HTTP2**. TTHeader can be used in conjunction with Thrift and Kitex Protobuf; HTTP2 is currently mainly used with the gRPC protocol, and it will support Thrift in the future.

- **Multi-message Type**

Kitex supports **PingPong**, **One-way**, and **Bidirectional Streaming**. Among them, One-way currently only supports Thrift protocol, two-way Streaming only supports gRPC, and Kitex will support Thrift's two-way Streaming in the future.

- **Service Governance**

Kitex integrates service governance modules such as service registry, service discovery, load balancing, circuit breaker, rate limiting, retry, monitoring, tracing, logging, diagnosis, etc. Most of these have been provided with default extensions, and users can choose to integrate.

- **Code Generation**

Kitex has built-in code generation tools that support generating **Thrift**, **Protobuf**, and scaffold code.

## Documentation

- [**Getting Started**](https://github.com/cloudwego/kitex/blob/develop/docs/guide/getting_started.md)
- **User Guide**

  - Basic Features: including **Message Type**, **Supported Protocols**, **Directly Invoke**, **Connection Pool**, **Timeout Control**, **Request Retry**, **LoadBalancer**, **Circuit Breaker**, **Rate Limiting**, **Instrumentation Control**, **Logging** and **HttpResolver**, [learn more](https://www.cloudwego.io/docs/tutorials/basic-feature/)
    
  - Governance Features: supporting **Service Discovery**, **Monitoring**, **Tracing** and **Customized Access Control**, [learn more](https://www.cloudwego.io/docs/tutorials/service-governance/)
    
  - Advanced Features: supporting **Generic Call** and **Server SDK Mode**, [learn more](https://www.cloudwego.io/docs/tutorials/advanced-feature/)
    
  - Code Generation: including **Code Generation Tool** and **Combined Service**, [learn more](https://www.cloudwego.io/docs/tutorials/code-gen/)
    
  - Framework Extension: providing **Middleware Extensions**, **Suite Extensions**, **Service Registry**, **Service Discovery**, **Customize LoadBalancer**, **Monitoring**, **Logging**, **Codec**, **Transport Module**, **Transport Pipeline**, **Metadata Transparent Transmission**, **Diagnosis Module**, [learn more](https://www.cloudwego.io/docs/tutorials/framework-exten/)
    
- **Reference**

  - For **Transport Protocol**, **Exception Instruction** and **Version Specification**, please refer to [doc](https://www.cloudwego.io/docs/reference/)
    
- **FAQ**

## Performance

We compared the performance of Kitex with some popular RPC frameworks ([benchmark](https://github.com/cloudwego/kitex-benchmark)), such as [gRPC](https://github.com/grpc/grpc) and [RPCX](https://github.com/smallnest/rpcx), both using Protobuf. The test results show that [Kitex](https://github.com/cloudwego/kitex) performs better.

*Note: The performance benchmarks obtained from the experiment are for reference only, because there are many factors that can affect the actual performance in application scenarios.*

### Test Environment

- CPU: Intel(R) Xeon(R) Gold 5118 CPU @ 2.30GHz, 4 cores
- Memory: 8GB
- OS: Debian 5.4.56.bsk.1-amd64 x86_64 GNU/Linux
- Go: 1.15.4

### Concurrency Performance 

Change the concurrency with a fixed packet size 1KB.

QPS|TP99|TP999
----|----|----
| ![image](docs/images/performance_concurrent_qps.png) | ![image](docs/images/performance_concurrent_tp99.png) | ![image](docs/images/performance_concurrent_tp999.png) |

### Throughput Performance

Change packet size with a fixed concurrency of 100.

QPS|TP99|TP999
----|----|----
|![image](docs/images/performance_bodysize_qps.png) | ![image](docs/images/performance_bodysize_tp99.png) | ![image](docs/images/performance_bodysize_tp999.png) |

## Related Projects

- [Netpoll](https://github.com/cloudwego/netpoll): A high-performance network library.
- [kitex-contrib](https://github.com/kitex-contrib): A partial extension library of Kitex, which users can integrate into Kitex through options according to their needs.
- [Example](https://github.com/cloudwego/kitex-examples): Use examples of Kitex.

## Blogs

- [Performance Optimization Practice of Go RPC framework Kitex](https://www.cloudwego.io/zh/blog/2021/09/23/%E5%AD%97%E8%8A%82%E8%B7%B3%E5%8A%A8-go-rpc-%E6%A1%86%E6%9E%B6-kitex-%E6%80%A7%E8%83%BD%E4%BC%98%E5%8C%96%E5%AE%9E%E8%B7%B5/)
- [Practice of ByteDance on Go Network Library](https://mp.weixin.qq.com/s?__biz=MzI1MzYzMjE0MQ==&mid=2247485756&idx=1&sn=4d2712e4bfb9be27a790fa15159a7be1&chksm=e9d0c2dedea74bc8179af39888a5b2b99266587cad32744ad11092b91ec2e2babc74e69090e6&scene=21#wechat_redirect)

## Contributing

[Contributing](https://github.com/cloudwego/kitex/blob/develop/CONTRIBUTING.md).

## License

Kitex is distributed under the [Apache License, version 2.0](https://github.com/cloudwego/kitex/blob/develop/LICENSE). The licenses of third party dependencies of Kitex are explained [here](https://github.com/cloudwego/kitex/blob/develop/licenses).

## Community
- Email: [conduct@cloudwego.io](conduct@cloudwego.io)
- Issues: [Issues](https://github.com/cloudwego/kitex/issues)
- Lark: Scan the QR code below with [Lark](https://www.larksuite.com/zh_cn/download) to join our CloudWeGo/kitex user group.

  ![LarkGroup](docs/images/lark_group.png)

## Landscapes

<p align="center">
<img src="https://landscape.cncf.io/images/left-logo.svg" width="150"/>&nbsp;&nbsp;<img src="https://landscape.cncf.io/images/right-logo.svg" width="200"/>
<br/><br/>
CloudWeGo enriches the <a href="https://landscape.cncf.io/">CNCF CLOUD NATIVE Landscape</a>.
</p>
