# 编解码协议

目前，Kitex 支持了 thrift 和 protobuf 两种编解码协议。

## Thrift

Kitex 支持了 Thrift 的 [Binary](https://github.com/apache/thrift/blob/master/doc/specs/thrift-binary-protocol.md) 协议，暂时没有支持 [Compact](https://github.com/apache/thrift/blob/master/doc/specs/thrift-compact-protocol.md) 协议。

生成代码时指定 thrift 协议，也可以不指定，默认就是 thrift：

- 客户端

  ```shell
  kitex -type thrift ${service_name} ${idl_name}.thrift
  ```

- 服务端

  ```shell
  kitex -type thrift -service ${service_name} ${idl_name}.thrift
  ```

我们针对 thrift 的 binary 协议编解码进行了优化，具体优化细节参考 "Reference - 高性能 Thrift 编解码 " 篇章，假如想要关闭这些优化，生成代码时可以加上 `-no-fast-api` 参数。

## Protobuf

### 协议说明

Kitex 对 protobuf 支持的协议有两种：

1. 自定义的消息协议，可以理解为 Kitex Protobuf，使用方式与 thrift 一样
2. gRPC 协议，可以与 gRPC 互通，并且支持 streaming 调用

如果 IDL 文件中定义了 streaming 方法则走 gRPC 协议，否则走 Kitex Protobuf。没有 streaming 方法，又想指定 gRPC 协议，需要 client 初始化做如下配置（server 支持协议探测无需配置） ：

```go
// 使用 WithTransportProtocol 指定 transport
cli, err := service.NewClient(destService, client.WithTransportProtocol(transport.GRPC))
```

### 生成代码

只支持 proto3，语法参考 https://developers.google.com/protocol-buffers/docs/gotutorial

注意：

1. 相较其他语言，必须定义 go_package ，以后 pb 官方也会将此作为必须约束
2. go_package 和 thrift 的 namespace 定义一样，不用写完整的路径，只需指定包名，相当于 thrift 的 namespace，如：go_package = "pbdemo"
3. 提前下载好 protoc 二进制放在 $PATH 目录下

生成代码时需要指定 protobuf 协议：

- 客户端

  ```shell
  kitex -type protobuf -I idl/ idl/${proto_name}.proto
  ```

- 服务端

  ```shell
  kitex -type protobuf -service ${service_name} -I idl/ idl/${proto_name}.proto
  ```
