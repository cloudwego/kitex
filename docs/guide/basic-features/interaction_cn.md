# 交互模式

## 协议支持

目前 Kitex 支持的交互方式和协议列表

|模式\支持|编码协议|传输协议|
|--------|-------|--------|
|PingPong|Thrift / Protobuf| None / [TTHeader](../extension/codec_cn.md) / HTTP2(gRPC)|
|Oneway|Thrift| None / [TTHeader](../extension/codec_cn.md) |
|Streaming|Protobuf|HTTP2(gRPC)|

- PingPong  发起一个请求等待一个响应
- Oneway 发起一个请求不等待一个响应
- Streaming 发起一个或多个请求, 等待一个或多个响应

## Thrift

目前 Thrift 支持 PingPong 和 Oneway 交互方式。Kitex 计划支持 Thrift Streaming。

### Example

idl 定义:

```thrift
namespace go echo

struct Request {
    1: string Msg
}

struct Response {
    1: string Msg
}

service EchoService {
    Response Echo(1: Request req); // pingpong method
    oneway void Echo(1: Request req); // oneway method
}
```

生成的代码组织结构:
```
.
└── echo
    ├── echo.go
    ├── echoservice
    │   ├── client.go
    │   ├── echoservice.go
    │   ├── invoker.go
    │   └── server.go
    └── k-echo.go
```

Server 侧代码:

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
)

type handler struct {}

func (handler) Echo(ctx context.Context, req *echo.Request) (r *echo.Response, err error) {
    return &echo.Response{ Msg: "world" }
}

func (handler) Echo1(ctx context.Context, req *echo.Request) (err error) {
    return nil
}

func main() {
    svr, err := echoservice.NewServer(handler{})
    if err != nil {
        panic(err)
    }
    svr.Run()
}
```

#### PingPong

Client 侧代码:

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
)

func main() {
    cli, err := echoservice.NewClient("p.s.m")
    if err != nil {
        panic(err)
    }
    req := echo.NewRequest()
    req.Msg = "hello"
    resp, err := cli.Echo(req)
    if err != nil {
        panic(err)
    }
    // resp.Msg == "world"
}
```

#### Oneway

Client 侧代码:

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
)

func main() {
    cli, err := echoservice.NewClient("p.s.m")
    if err != nil {
        panic(err)
    }
    req := echo.NewRequest()
    req.Msg = "hello"
    err = cli.Echo1(req)
    if err != nil {
        panic(err)
    }
    // no response return
}
```

## Protobuf

  Kitex 对 Protobuf 支持的协议有两种：
  
  - Kitex Protobuf - 只支持 PingPong，若 IDL 定义了 stream 方法，将默认使用 gRPC 协议
        
  - gRPC协议 - 可以与 gRPC 互通，与 gRPC service 定义相同，支持 Unary(PingPong)、 Streaming 调用

### Example
以下给出 Streaming 的使用示例。

idl 定义:

```protobuf
syntax = "proto3";

option go_package = "echo";

package echo;

message Request {
  string msg = 1;
}

message Response {
  string msg = 1;
}

service EchoService {
  rpc ClientSideStreaming(stream Request) returns (Response) {} // 客户端侧 streaming
  rpc ServerSideStreaming(Request) returns (stream Response) {} // 服务端侧 streaming
  rpc BidiSideStreaming(stream Request) returns (stream Response) {} // 双向流
}
```

生成的代码组织结构:

```
.
└── echo
    ├── echo.pb.go
    └── echoservice
        ├── client.go
        ├── echoservice.go
        ├── invoker.go
        └── server.go
```

Server 侧代码:

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
}

type handler struct{}

func (handler) ClientSideStreaming(stream echo.EchoService_ClientSideStreamingServer) (err error) {
    for {
        req, err := stream.Recv()
        if err != nil {
            return err
        }
    }
}

func (handler) ServerSideStreaming(req *echo.Request, stream echo.EchoService_ServerSideStreamingServer) (err error) {
      _ = req
      for {
          resp := &echo.Response{Msg: "world"}
          if err := stream.Send(resp); err != nil {
              return err
          }
      }
}

func (handler) BidiSideStreaming(stream echo.EchoService_BidiSideStreamingServer) (err error) {
    go func() {
        for {
            req, err := stream.Recv()
            if err != nil {
                return err
            }
        }
    }()
    for {
        resp := &echo.Response{Msg: "world"}
        if err := stream.Send(resp); err != nil {
            return err
        }
    }
}

func main() {
    svr, err := echoservice.NewServer(handler{})
    if err != nil {
        panic(err)
    }
    svr.Run()
}
```

#### Streaming

ClientSideStreaming:

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
}

func main() {
    cli, err := echoseervice.NewClient("p.s.m")
    if err != nil {
        panic(err)
    }
    cliStream, err := cli.ClientSideStreaming(context.Background())
    if err != nil {
        panic(err)
    }
    for {
        req := &echo.Request{Msg: "hello"}
        if err := cliStream.Send(req); err != nil {
            panic(err)
        }
    }
    
}
```

ServerSideStreaming:

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
}

func main() {
    cli, err := echoseervice.NewClient("p.s.m")
    if err != nil {
        panic(err)
    }
    req := &echo.Request{Msg: "hello"}
    svrStream, err := cli.ServerSideStreaming(context.Background(), req)
    if err != nil {
        panic(err)
    }
    for {
        resp, err := svrStream.Recv()
        if err != nil {
            panic(err)
        }
        // resp.Msg == "world"
    }
    
}
```

BidiSideStreaming:

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
}

func main() {
    cli, err := echoseervice.NewClient("p.s.m")
    if err != nil {
        panic(err)
    }
    req := &echo.Request{Msg: "hello"}
    bidiStream, err := cli.BidiSideStreaming(context.Background())
    if err != nil {
        panic(err)
    }
    go func() {
        for {
            req := &echo.Request{Msg: "hello"}
            err := bidiStream.Send(req)
            if err != nil {
                panic(err)
            }
        }
    }()
    for {
        resp, err := bidiStream.Recv()
        if err != nil {
            panic(err)
        }
        // resp.Msg == "world"
    } 
}
```
