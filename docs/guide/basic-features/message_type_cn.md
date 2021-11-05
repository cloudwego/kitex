# 消息类型

## 协议支持

目前 Kitex 支持的消息类型、编解码协议和传输协议

| 消息类型 | 编码协议 | 传输协议 |
|--------|-------|--------|
|PingPong|Thrift / Protobuf| [TTHeader](../extension/codec_cn.md) / HTTP2(gRPC)|
|Oneway|Thrift| [TTHeader](../extension/codec_cn.md) |
|Streaming|Protobuf|HTTP2(gRPC)|

- PingPong：客户端发起一个请求后会等待一个响应才可以进行下一次请求
- Oneway：客户端发起一个请求后不等待一个响应
- Streaming：客户端发起一个或多个请求 , 等待一个或多个响应

## Thrift

目前 Thrift 支持 PingPong 和 Oneway。Kitex 计划支持 Thrift Streaming。

### Example

IDL 定义 :

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
    oneway void VisitOneway(1: Request req); // oneway method
}
```

生成的代码组织结构 :

```
.
└── kitex_gen
    └── echo
        ├── echo.go
        ├── echoservice
        │   ├── client.go
        │   ├── echoservice.go
        │   ├── invoker.go
        │   └── server.go
        └── k-echo.go
```

Server 的处理代码形如 :

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
)

type handler struct {}

func (handler) Echo(ctx context.Context, req *echo.Request) (r *echo.Response, err error) {
    // ...
    return &echo.Response{ Msg: "world" }
}

func (handler) VisitOneway(ctx context.Context, req *echo.Request) (err error) {
    // ...
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

Client 侧代码 :

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
)

func main() {
    cli, err := echoservice.NewClient("destServiceName")
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

Client 侧代码 :

```go
package main

import (
    "xx/echo"
    "xx/echo/echoservice"
)

func main() {
    cli, err := echoservice.NewClient("destServiceName")
    if err != nil {
        panic(err)
    }
    req := echo.NewRequest()
    req.Msg = "hello"
    err = cli.VisitOneway(req)
    if err != nil {
        panic(err)
    }
    // no response return
}
```

## Protobuf

Kitex 支持两种承载 Protobuf 负载的协议：

- Kitex Protobuf
    - 只支持 PingPong，若 IDL 定义了 stream 方法，将默认使用 gRPC 协议
- gRPC 协议
    - 可以与 gRPC 互通，与 gRPC service 定义相同，支持 Unary(PingPong)、 Streaming 调用

### Example

以下给出 Streaming 的使用示例。

IDL 定义 :

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

生成的代码组织结构 :

```
.
└── kitex_gen
    └── echo
        ├── echo.pb.go
        └── echoservice
            ├── client.go
            ├── echoservice.go
            ├── invoker.go
            └── server.go
```

Server 侧代码 :

```go
package main

import (
    "sync"

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
    var once sync.Once
	go func() {
		for {
			req, err2 := stream.Recv()
			log.Println("received:", req.GetMsg())
			if err2 != nil {
				once.Do(func() {
					err = err2
				})
				break
			}
		}
	}()
	for {
		resp := &echo.Response{Msg: "world"}
		if err2 := stream.Send(resp); err2 != nil {
			once.Do(func() {
				err = err2
			})
			return
		}
	}
	return
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
    cli, err := echoseervice.NewClient("destServiceName")
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
    cli, err := echoseervice.NewClient("destServiceName")
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
    cli, err := echoseervice.NewClient("destServiceName")
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
