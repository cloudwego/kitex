# CloudWeGo-Kitex

[English](README.md) | 中文

Kitex[kaɪt'eks] 字节跳动内部的 Golang 微服务 RPC 框架，具有**高性能**、**强可扩展**的特点，在字节内部已广泛使用。如今越来越多的微服务选择使用 Golang，如果对微服务性能有要求，又希望定制扩展融入自己的治理体系，Kitex 会是一个不错的选择。

## 框架特点

- **高性能**

  使用自研的高性能网络库 [Netpoll](https://github.com/cloudwego/netpoll)，性能相较 go net 具有显著优势。

- **扩展性**

  提供了较多的扩展接口以及默认扩展实现，使用者也可以根据需要自行定制扩展，具体见下面的框架扩展。

- **多消息协议**

  RPC 消息协议默认支持 **Thrift**、**Kitex Protobuf**、**gRPC**。Thrift 支持 Buffered 和 Framed 二进制协议；Kitex Protobuf 是 Kitex 自定义的 Protobuf 消息协议，协议格式类似 Thrift；gRPC 是对 gRPC 消息协议的支持，可以与 gRPC 互通。除此之外，使用者也可以扩展自己的消息协议。

- **多传输协议**

  传输协议封装消息协议进行 RPC 互通，传输协议可以额外透传元信息，用于服务治理，Kitex 支持的传输协议有 **TTHeader**、**HTTP2**。TTHeader 可以和 Thrift、Kitex Protobuf 结合使用；HTTP2 目前主要是结合 gRPC 协议使用，后续也会支持 Thrift。

- **多种消息类型**

  支持 **PingPong**、**Oneway**、**双向 Streaming**。其中 Oneway 目前只对 Thrift 协议支持，双向 Streaming 只对 gRPC 支持，后续会考虑支持 Thrift 的双向 Streaming。

- **服务治理**

  支持服务注册/发现、负载均衡、熔断、限流、重试、监控、链路跟踪、日志、诊断等服务治理模块，大部分均已提供默认扩展，使用者可选择集成。

- **代码生成**

  Kitex 内置代码生成工具，可支持生成 **Thrift**、**Protobuf** 以及脚手架代码。

## 详细文档

  - [**快速开始**](https://www.cloudwego.io/zh/docs/getting-started/)

  - **用户指南**
  
    - **基本特性**
    
      包含消息类型、编解码协议、直连访问、连接池、超时控制、请求重试、负载均衡、熔断、埋点粒度控制、日志以及 HttpResolver，详见[文档](https://www.cloudwego.io/zh/docs/tutorials/basic-feature/) 
    
    - **治理特性**
      
      支持服务发现、监控、链路跟踪、自定义访问控制等治理特性，详见[文档](https://www.cloudwego.io/zh/docs/tutorials/service-governance/)
      
    - **高级特性**
    
      支持泛化调用、Server SDK 化等高级特性，详见[文档](https://www.cloudwego.io/zh/docs/tutorials/advanced-features/)
    
    - **代码生成**
    
      提供代码生成工具与 Combine Service说明，详见[文档](https://www.cloudwego.io/zh/docs/tutorials/code-gen/)
    
    - **框架扩展**
    
      提供基本扩展 - 自定义Middleware、Suite 扩展（封装自定义治理模块）、服务注册扩展、服务发现扩展、负载均衡扩展、监控扩展、日志扩展、编解码(协议)扩展、传输模块扩展、Transport Pipeline-Bound扩展、元信息传递扩展、诊断模块扩展等支持，详见[文档](https://www.cloudwego.io/zh/docs/tutorials/framework-exten/)
    
  - **参考**

    - 关于应用层传输协议 TTHeader、异常说明与版本管理，请参考[文档](https://www.cloudwego.io/zh/docs/reference/)

  - **FAQ**

## 框架性能

性能测试只能提供相对参考，工业场景下，有诸多因素可以影响实际的性能表现。

由于开源社区缺少支持 Thrift 的优秀 RPC 框架，当前对比项目为 [gRPC](https://github.com/grpc/grpc)、[RPCX](https://github.com/smallnest/rpcx)，均使用 Protobuf 。

我们通过 [测试代码](https://github.com/cloudwego/kitex-benchmark) 比较了它们的性能，测试表明 [Kitex](https://github.com/cloudwego/kitex) 具有明显优势。

### 测试环境

* CPU:    Intel(R) Xeon(R) Gold 5118 CPU @ 2.30GHz, 4 cores
* Memory: 8GB
* OS:     Debian 5.4.56.bsk.1-amd64 x86_64 GNU/Linux
* Go:     1.15.4

### 并发表现 (Echo 1KB, 改变并发量)

| QPS                                                  |                         TP99                          |                         TP999                          |
| :--------------------------------------------------- | :---------------------------------------------------: | :----------------------------------------------------: |
| ![image](docs/images/performance_concurrent_qps.png) | ![image](docs/images/performance_concurrent_tp99.png) | ![image](docs/images/performance_concurrent_tp999.png) |

### 吞吐表现 (并发 100, 改变包大小)

| QPS                                                |                        TP99                         |                        TP999                         |
| :------------------------------------------------- | :-------------------------------------------------: | :--------------------------------------------------: |
| ![image](docs/images/performance_bodysize_qps.png) | ![image](docs/images/performance_bodysize_tp99.png) | ![image](docs/images/performance_bodysize_tp999.png) |

## 相关项目

- [Netpoll](https://github.com/cloudwego/netpoll): 自研的高性能网络库，Kitex 默认集成的。
- [kitex-contrib](https://github.com/kitex-contrib)：Kitex 的部分扩展库，使用者可以根据需求通过 Option 集成进 Kitex 中。
- [Example](https://github.com/cloudwego/kitex-examples)：Kitex 的使用示例。

## 相关文章

- [字节跳动 Go RPC 框架 Kitex 性能优化实践](https://www.cloudwego.io/zh/blog/2021/09/23/%E5%AD%97%E8%8A%82%E8%B7%B3%E5%8A%A8-go-rpc-%E6%A1%86%E6%9E%B6-kitex-%E6%80%A7%E8%83%BD%E4%BC%98%E5%8C%96%E5%AE%9E%E8%B7%B5/)
- [字节跳动在 Go 网络库上的实践](https://mp.weixin.qq.com/s?__biz=MzI1MzYzMjE0MQ==&mid=2247485756&idx=1&sn=4d2712e4bfb9be27a790fa15159a7be1&chksm=e9d0c2dedea74bc8179af39888a5b2b99266587cad32744ad11092b91ec2e2babc74e69090e6&scene=21#wechat_redirect)

## 贡献代码

[Contributing](CONTRIBUTING.md)。

## 开源许可

Kitex 基于[Apache License 2.0](LICENSE) 许可证，其依赖的三方组件的开源许可见 [Licenses](licenses)。

## 联系我们
- Email: conduct@cloudwego.io
- Issues: [Issues](https://github.com/cloudwego/kitex/issues)
- 飞书用户群（[注册飞书](https://www.feishu.cn/)进群）

  ![LarkGroup](docs/images/lark_group_cn.png)

## Landscapes

<p align="center">
<img src="https://landscape.cncf.io/images/left-logo.svg" width="150"/>&nbsp;&nbsp;<img src="https://landscape.cncf.io/images/right-logo.svg" width="200"/>
<br/><br/>
CloudWeGo 丰富了 <a href="https://landscape.cncf.io/">CNCF 云原生生态</a>。
</p>
