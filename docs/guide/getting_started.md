# Getting Started

## Prerequisites

1. If you don't setup golang development environment, please follow [Install Go](https://golang.org/doc/install) to install go.
2. We strongly recommend you use latest golang version. And compatibility is guaranteed within two latest minor version (for now **v1.15**).
3. Ensure `GO111MODULE` is set to `on`.

## Quick Start

This chapter gonna get you started with Kitex with a simple executable example.

### Install Compiler

First of all, let's install compilers we gonna work with. 

1. Ensure `GOPATH` environment variable is defined properly (for example `export GOPATH=~/go`), then add `$GOPATH/bin` to `PATH` environment variable (for example `export PATH=$GOPATH/bin:$PATH`). Make sure `GOPATH` is accessible.
2. Install kitex: `go get github.com/jackedelic/kitex/internal/tool/cmd/kitex@latest`
3. Install thriftgo: `go get github.com/cloudwego/thriftgo@latest`

Now you can run `kitex --version` and `thriftgo --version`, and you can see some outputs just like below if you setup compilers successfully.

 ```shell
$ kitex --version
vx.x.x

$ thriftgo --version
thriftgo x.x.x
```
Tips: If you encounter any problem during installation, it's probably you don't setup golang develop environment properly. In most cases you can search error message to find solution.

### Get the example

1. You can just click [HERE](https://github.com/jackedelic/kitex/internal/-examples/archive/refs/heads/main.zip) to download the example
2. Or you can clone the example repository `git clone https://github.com/jackedelic/kitex/internal/-examples.git`

### Run the example

#### Run by go 

1. enter `hello` directory

   `cd examples/hello`

2. run server

   `go run .`

3. run client

   open a another terminal, and `go run ./client`

#### Run by docker

1. enter the example directory
   
   `cd examples`
   
2. build the example project
   
   `docker build -t kitex-examples .`
3. run server
   
   `docker run --network host kitex-examples ./hello-server`
   
4. run client

   open another terminal, and `docker run --network host kitex-examples ./hello-client`

congratulation! You successfully use Kitex to complete a RPC.

### Add a new method

open `hello.thrift`, you will see code below:

```thrift
namespace go api

struct Request {
        1: string message
}

struct Response {
        1: string message
}

service Hello {
    Response echo(1: Request req)
}
```

Now let's define a new request and response`AddRequest` 和 `AddResponse`, after that add `add` method to `service Hello`:

```thrift
namespace go api

struct Request {
        1: string message
}

struct Response {
        1: string message
}

struct AddRequest {
	1: i64 first
	2: i64 second
}

struct AddResponse {
	1: i64 sum
}

service Hello {
    Response echo(1: Request req)
    AddResponse add(1: AddRequest req)
}
```

When you complete it, `hello.thrift` should be just like above.

### Regenerate code

Run below command, then `kitex` compiler will recompile `hello.thrift` and update generated code.

`kitex -service a.b.c hello.thrift`

After you run the above command, `kitex` compiler will update these files:

	1. update `./handler.go`, add a basic implementation of `add` method.
	2. update `./kitex_gen`, updates client and server implementation.

### Update handler

When you complete **Regenerate Code** chapter, `kitex` will add a basic implementation of `Add` to `./handler.go`, just like:

```go
// Add implements the HelloImpl interface.
func (s *HelloImpl) Add(ctx context.Context, req *api.AddRequest) (resp *api.AddResponse, err error) {
        // TODO: Your code here...
        return
}
```

Let's complete process logic, like:

```go
// Add implements the HelloImpl interface.
func (s *HelloImpl) Add(ctx context.Context, req *api.AddRequest) (resp *api.AddResponse, err error) {
        // TODO: Your code here...
        resp = &api.AddResponse{Sum: req.First + req.Second}
        return
}
```

### Call "add" method

Let's add `add` RPC to client example.

You can see something like below in `./client/main.go`:

```go
for {
        req := &api.Request{Message: "my request"}
        resp, err := client.Echo(context.Background(), req)
        if err != nil {
                log.Fatal(err)
        }
        log.Println(resp)
        time.Sleep(time.Second)
}
```

Let's add `add` RPC:

```go
for {
        req := &api.Request{Message: "my request"}
        resp, err := client.Echo(context.Background(), req)
        if err != nil {
                log.Fatal(err)
        }
        log.Println(resp)
        time.Sleep(time.Second)
  			addReq := &api.AddRequest{First: 512, Second: 512}
  			addResp, err := client.Add(context.Background(), addReq)
        if err != nil {
                log.Fatal(err)
        }
        log.Println(addResp)
        time.Sleep(time.Second)
}
```

### Run application again

Shutdown server and client we have run, then:

1.run server

`go run .`

1. run client

open another terminal, and `go run ./client`

Now, you can see outputs of `add` RPC.

## Tutorial

### About Kitex

Kitex is a RPC framework which supports multiple serialization protocols and transport protocols.

Kitex compiler supports both `thrift` and `proto3` IDL, and fairly Kitex supports `thrift` and `protobuf` serialization protocol. Kitex extends `thrift` as transport protocol, and also supports `gRPC` protocol.

### WHY IDL

We use IDL to define interface.

Thrift IDL grammar: [Thrift interface description language](http://thrift.apache.org/docs/idl)。   

proto3 grammar: [Language Guide(proto3)](https://developers.google.com/protocol-buffers/docs/proto3)。

### Create project directory

Let's create a directory to setup project.

`$ mkdir example`   

enter directory

`$ cd example`

### Kitex compiler

`kitex` is a compiler which has the same name as `Kitex` framework, it can generate a project including client and server conveniently.

#### Install

You can use following command to install and upgrade `kitex`:

`$ go get github.com/jackedelic/kitex/internal/tool/cmd/kitex`

After that, you can just run it to check whether it's installed successfully.
完成后，可以通过执行kitex来检测是否安装成功。   

`$ kitex`   

If you see some outputs like below, congratulation!

`$ kitex`   

`No IDL file found.`   


If you see something like `command not found`, you should add `$GOPATH/bin` to `$PATH`. For detail, see chapter **Prerequisites** .  

#### Usage

You can visit [Compiler](basic-features/code_generation.md) for detailed usage.

### Write IDL

For example, a thrift IDL. 

create a `echo.thrift` file, and define a service like below:

```thrift
namespace go api

struct Request {
	1: string message
}

struct Response {
	1: string message
}

service Echo {
    Response echo(1: Request req)
}
```

### Generate echo service code

We can use `kitex` compiler to compile the IDL file to generate whole project. 

`$ kitex -module example -service example echo.thrift`   

`-module` indicates go module name of project，`-service` indicates expected to generate a executable service named `example`, the last parameter is path to IDL file.

Generated project layout:
```
.
|-- build.sh
|-- conf
|   `-- kitex.yml
|-- echo.thrift
|-- handler.go
|-- kitex_gen
|   `-- api
|       |-- echo
|       |   |-- client.go
|       |   |-- echo.go
|       |   |-- invoker.go
|       |   `-- server.go
|       |-- echo.go
|       `-- k-echo.go
|-- main.go
`-- script
    |-- bootstrap.sh
    `-- settings.py
```

### Get latest Kitex

Kitex expect project to use go module as dependency manager. It cloud be easy to upgrade Kitex:
```
$ go get github.com/jackedelic/kitex/internal/
$ go mod tidy
```

If you encounter something like below :

`github.com/apache/thrift/lib/go/thrift: ambiguous import: found package github.com/apache/thrift/lib/go/thrift in multiple modules`   

Run following command, and try again:

```
go mod edit -droprequire=github.com/apache/thrift/lib/go/thrift
go mod edit -replace=github.com/apache/thrift=github.com/apache/thrift@v0.13.0
```

### Write echo service process

All method process entry should be in `handler.go`, you should see something like below in this file:

```go
package main

import (
	"context"
	"example/kitex_gen/api"
)

// EchoImpl implements the last service interface defined in the IDL.
type EchoImpl struct{}

// Echo implements the EchoImpl interface.
func (s *EchoImpl) Echo(ctx context.Context, req *api.Request) (resp *api.Response, err error) {
	// TODO: Your code here...
	return
}

```

`Echo` method represents the `echo` we defined in thrift IDL.   

Now let's make `Echo` a real echo.   

modify `Echo` method:

```go
func (s *EchoImpl) Echo(ctx context.Context, req *api.Request) (resp *api.Response, err error) {
	return &api.Response{Message: req.Message}, nil
}
```

### Compile and Run

kitex compiler has generated scripts to compile and run the project:

Compile:   

`$ sh build.sh`   

There should be a `output` directory After you execute above command, which includes compilation productions   .

Run:   

`$ sh output/bootstrap.sh`

Now, `Echo` service is running!

### Write Client

Let's write a client to call `Echo` server.

create a directory as client package:   

`$ mkdir client`

enter directory:

`$ cd client`

create a `main.go` file.

#### Create Client

Let's new a `client` to do RPC：

```go
import "example/kitex_gen/api/echo"
import "github.com/jackedelic/kitex/internal/client"
...
c, err := echo.NewClient("example", client.WithHostPorts("0.0.0.0:8888"))
if err != nil {
	log.Fatal(err)
}
```
`echo.NewClient` is used to new a `client`, th first parameter is *service name*, the second parameter is *options* which is used to pass options. `client.WithHostPorts` is used to specify server address, see chapter **Basic Feature** for detail.

#### Do RPC

Let's write call code:

```go
import "example/kitex_gen/api"
...
req := &api.Request{Message: "my request"}
resp, err := c.Echo(context.Background(), req, callopt.WithRPCTimeout(3*time.Second))
if err != nil {
	log.Fatal(err)
}
log.Println(resp)
```
We new a request `req`, then we use `c.Echo` to do a RPC call.

The first parameter `context.Context`, is used to transfer information or to control some call behaviors. You will see detailed usage in behind chapters.\

The seconde parameter is request. 

The third parameter is call `options`, which is called `callopt`, these options only works for this RPC call.  
`callopt.WithRPCTimeout` is used to specify timeout for this RPC call. See chapter **Basic Feature** for detail.

### Run Client

You can run following command to run a client:  

`$ go run main.go`

You should see some outputs like below:

`2021/05/20 16:51:35 Response({Message:my request})`

Congratulation! You have written a Kitex server and client, and have done a RPC call.
