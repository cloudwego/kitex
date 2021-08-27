# Getting Started

## 准备 Golang 开发环境

1. 如果您之前未搭建 Golang 开发环境， 可以参考 [Golang 安装](https://golang.org/doc/install)
2. 推荐使用最新版本的 Golang，或保证现有 Golang 版本 >= 1.15。小于 1.15 版本，可以自行尝试使用但不保障兼容性和稳定性
3. 确保打开 go mod 支持 (Golang >= 1.15时，默认开启)

## 快速上手

在完成环境准备后，本章节将帮助你快速上手 Kitex

### 安装代码生成工具

首先，我们需要安装使用本示例所需要的命令行代码生成工具：

1. 确保 `GOPATH` 环境变量已经被正确地定义（例如 `export GOPATH=~/go`）并且将`$GOPATH/bin`添加到 `PATH` 环境变量之中（例如 `export PATH=$GOPATH/bin:$PATH`）；请勿将 `GOPATH` 设置为当前用户没有读写权限的目录
2. 安装 kitex：`go get github.com/cloudwego/kitex/tool/cmd/kitex@latest`
3. 安装 thriftgo：`go get github.com/cloudwego/thriftgo@latest`

安装成功后，执行 `kitex --version` 和 `thriftgo --version` 应该能够看到具体版本号的输出（版本号有差异，以 x.x.x 示例）：

 ```shell
$ kitex --version
vx.x.x

$ thriftgo --version
thriftgo x.x.x
```
4. 如果在安装阶段发生问题，可能主要是由于对 Golang 的不当使用造成，请依照报错信息进行检索

### 获取示例代码

1. 你可以直接点击 [此处](https://github.com/cloudwego/kitex-examples/archive/refs/heads/main.zip) 下载示例仓库
2. 也可以克隆该示例仓库到本地 `git clone https://github.com/cloudwego/kitex-examples.git`

### 运行示例代码

#### 方式一：直接启动

1. 进入示例仓库的 `hello` 目录

   `cd examples/hello`

2. 运行 server

   `go run .`

3. 运行 client

   另起一个终端后，`go run ./client`

#### 方式二：使用 Docker 快速启动

1. 进入示例仓库目录

   `cd examples`

2. 编译项目

   `docker build -t kitex-examples .`
3. 运行 server

   `docker run --network host kitex-examples ./hello-server`

4. 运行 client

   另起一个终端后，`docker run --network host kitex-examples ./hello-client`

恭喜你，你现在成功通过 Kitex 发起了 RPC 调用。

### 增加一个新的方法

打开 `hello.thrift`，你会看到如下内容：

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

现在让我们为新方法分别定义一个新的请求和响应，`AddRequest` 和 `AddResponse`，并在 `service Hello` 中增加 `add` 方法：

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

完成之后 `hello.thrift` 的内容应该和上面一样。

### 重新生成代码

运行如下命令后，`kitex` 工具将根据 `hello.thrift` 更新代码文件。

`kitex -service a.b.c hello.thrift`

执行完上述命令后，`kitex` 工具将更新下述文件

	1. 更新 `./handler.go`，在里面增加一个 `Add` 方法的基本实现
	2. 更新 `./kitex_gen`，里面有框架运行所必须的代码文件

### 更新服务端处理逻辑

上述步骤完成后，`./handler.go` 中会自动补全一个 `Add` 方法的基本实现，类似如下代码：

```go
// Add implements the HelloImpl interface.
func (s *HelloImpl) Add(ctx context.Context, req *api.AddRequest) (resp *api.AddResponse, err error) {
        // TODO: Your code here...
        return
}
```

让我们在里面增加我们所需要的逻辑，类似如下代码：

```go
// Add implements the HelloImpl interface.
func (s *HelloImpl) Add(ctx context.Context, req *api.AddRequest) (resp *api.AddResponse, err error) {
        // TODO: Your code here...
        resp = &api.AddResponse{Sum: req.First + req.Second}
        return
}
```

### 增加客户端调用

服务端已经有了 `Add` 方法的处理，现在让我们在客户端增加对 `Add` 方法的调用。

在 `./client/main.go` 中你会看到类似如下的 `for` 循环：

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

现在让我们在里面增加 `Add` 方法的调用：

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

### 重新运行示例代码

关闭之前运行的客户端和服务端之后

1. 运行 server

`go run .`

2. 运行 client

另起一个终端后，`go run ./client`

现在，你应该能看到客户端在调用 `Add` 方法了。

## 基础教程

### 关于 Kitex

Kitex 是一个 RPC 框架，既然是 RPC，底层就需要两大功能：
1. Serialization 序列化
2. Transport 传输

Kitex 框架及命令行工具，默认支持 `thrift` 和 `proto3` 两种 IDL，对应的 Kitex 支持 `thrift` 和 `protobuf` 两种序列化协议。传输上 Kitex 使用扩展的 `thrift` 作为底层的传输协议（注：thrift 既是 IDL 格式，同时也是序列化协议和传输协议）。IDL 全称是 Interface Definition Language，接口定义语言。

### 为什么要使用 IDL

如果我们要进行 RPC，就需要知道对方的接口是什么，需要传什么参数，同时也需要知道返回值是什么样的，就好比两个人之间交流，需要保证在说的是同一个语言、同一件事。这时候，就需要通过 IDL 来约定双方的协议，就像在写代码的时候需要调用某个函数，我们需要知道函数签名一样。  

Thrift IDL 语法可参考：[Thrift interface description language](http://thrift.apache.org/docs/idl)。   

proto3 语法可参考：[Language Guide(proto3)](https://developers.google.com/protocol-buffers/docs/proto3)。

### 创建项目目录

在开始后续的步骤之前，想让我们创建一个项目目录用于后续的教程。   

`$ mkdir example`   

然后让我们进入项目目录   

`$ cd example`

### Kitex 命令行工具

Kitex 自带了一个同名的命令行工具 `kitex`，用来帮助大家很方便地生成代码，新项目的生成以及之后我们会学到的 server、client 代码的生成都是通过 kitex 工具进行。

#### 安装

可以使用以下命令来安装或者更新 kitex：   

`$ go get github.com/cloudwego/kitex/tool/cmd/kitex`

完成后，可以通过执行 kitex 来检测是否安装成功。   

`$ kitex`   

如果出现如下输出，则安装成功。   

`$ kitex`   

`No IDL file found.`   

如果出现 `command not found` 错误，可能是因为没有把 `$GOPATH/bin` 加入到 `$PATH` 中，详见环境准备一章。   

#### 使用

kitex 的具体使用请参考[代码生成工具](basic-features/code_generation_cn.md)

### 编写 IDL

首先我们需要编写一个 IDL，这里以 thrift IDL 为例。   

首先创建一个名为 `echo.thrift` 的 thrift IDL 文件。   

然后在里面定义我们的服务
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

### 生成 echo 服务代码

有了 IDL 以后我们便可以通过 kitex 工具生成项目代码了，执行如下命令：   

`$ kitex -module example -service example echo.thrift`   

上述命令中，`-module` 表示生成的该项目的 go module 名，`-service` 表明我们要生成一个服务端项目，后面紧跟的 `example` 为该服务的名字。最后一个参数则为该服务的 IDL 文件。   

生成后的项目结构如下：
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

### 获取最新的 Kitex 框架

由于 kitex 要求使用 go mod 进行依赖管理，所以我们要升级 kitex 框架会很容易，只需要执行以下命令即可：
```
$ go get github.com/cloudwego/kitex
$ go mod tidy
```

如果遇到类似如下报错：   

`github.com/apache/thrift/lib/go/thrift: ambiguous import: found package github.com/apache/thrift/lib/go/thrift in multiple modules`   

先执行一遍下述命令，再继续操作：
```
go mod edit -droprequire=github.com/apache/thrift/lib/go/thrift
go mod edit -replace=github.com/apache/thrift=github.com/apache/thrift@v0.13.0
```

### 编写 echo 服务逻辑

我们需要编写的服务端逻辑都在 `handler.go` 这个文件中，现在这个文件应该如下所示：

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

这里的 `Echo` 函数就对应了我们之前在 IDL 中定义的 `echo` 方法。   

现在让我们修改一下服务端逻辑，让 `Echo` 服务名副其实。   

修改 `Echo` 函数为下述代码：

```go
func (s *EchoImpl) Echo(ctx context.Context, req *api.Request) (resp *api.Response, err error) {
	return &api.Response{Message: req.Message}, nil
}
```

### 编译运行

kitex 工具已经帮我们生成好了编译和运行所需的脚本：   

编译：   

`$ sh build.sh`   

执行上述命令后，会生成一个 `output` 目录，里面含有我们的编译产物。   

运行：   

`$ sh output/bootstrap.sh`

执行上述命令后，`Echo` 服务就开始运行啦！

### 编写客户端

有了服务端后，接下来就让我们编写一个客户端用于调用刚刚运行起来的服务端。

首先，同样的，先创建一个目录用于存放我们的客户端代码：   

`$ mkdir client`

进入目录：   

`$ cd client`

创建一个 `main.go` 文件，然后就开始编写客户端代码了。   

#### 创建 client

首先让我们创建一个调用所需的 `client`：

```go
import "example/kitex_gen/api/echo"
import "github.com/cloudwego/kitex/client"
...
c, err := echo.NewClient("example", client.WithHostPorts("0.0.0.0:8888"))
if err != nil {
	log.Fatal(err)
}
```
上述代码中，`echo.NewClient` 用于创建 `client`，其第一个参数为调用的 *服务名*，第二个参数为 *options*，用于传入参数，此处的 `client.WithHostPorts` 用于指定服务端的地址，更多参数可参考 *** 基本特性*** 一节。

#### 发起调用

接下来让我们编写用于发起调用的代码：

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
上述代码中，我们首先创建了一个请求 `req` , 然后通过 `c.Echo` 发起了调用。   

其第一个参数为 `context.Context`，通过通常用其传递信息或者控制本次调用的一些行为，你可以在后续章节中找到如何使用它。   

其第二个参数为本次调用的请求。   

其第三个参数为本次调用的 `options` ，Kitex 提供了一种 `callopt` 机制，顾名思义——调用参数 ，有别于创建 client 时传入的参数，这里传入的参数仅对此次生效。   
此处的 `callopt.WithRPCTimeout` 用于指定此次调用的超时（通常不需要指定，此处仅作演示之用）同样的，你可以在 *** 基础特性*** 一节中找到更多的参数。

### 发起调用

在编写完一个简单的客户端后，我们终于可以发起调用了。

你可以通过下述命令来完成这一步骤：   

`$ go run main.go`

如果不出意外，你可以看到类似如下输出：

`2021/05/20 16:51:35 Response({Message:my request})`

恭喜你！至此你成功编写了一个 Kitex 的服务端和客户端，并完成了一次调用！
