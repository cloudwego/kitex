# Message Types

## Protocols

The table below is message types, codecs and transports supported by Kitex.

|Message Types|Codec|Transport|
|--------|-------|--------|
|PingPong|Thrift / Protobuf| [TTHeader](../extension/codec.md) / HTTP2(gRPC)|
|Oneway|Thrift| [TTHeader](../extension/codec.md) |
|Streaming|Protobuf|HTTP2(gRPC)|

- PingPong: the client always waits for a response after sending a request
- Oneway: the client does not expect any response after sending a request
- Streaming: the client can send one or more requests while receiving one or more responses.

## Thrift

When the codec is thrift, Kitex supports PingPong and Oneway. The streaming on thrift is under development.

### Example

Given an IDL:

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

The layout of generated code might be:

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

The handler code in server side might be:

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

The code in client side might be:

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

The code in client side might be:

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
    err = cli.Echo1(req)
    if err != nil {
        panic(err)
    }
    // no response return
}
```

## Protobuf

Kitex supports two kind of protocols that carries Protobuf payload:

- Kitex Protobuf
    - Only supports the PingPong type of messages. If any streaming method is defined in the IDL, the protocol will switch to gRPC.
- The gRPC Protocol
    - The protocol that shipped with gRPC.

### Example

The following is an example showing how to use the streaming types.

Given an IDL:

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
  rpc ClientSideStreaming(stream Request) returns (Response) {} // client streaming
  rpc ServerSideStreaming(Request) returns (stream Response) {} // server streaming
  rpc BidiSideStreaming(stream Request) returns (stream Response) {} // bidirectional streaming 
}
```

The generated code might be:

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

The handler code in server side:

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
