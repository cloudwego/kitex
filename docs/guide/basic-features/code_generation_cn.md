
# kitex -- Kitex 框架的代码生成工具

**kitex** 是 Kitex 框架提供的用于生成代码的一个命令行工具。目前，kitex 支持 thrift 和 protobuf 的 IDL，并支持生成一个服务端项目的骨架。

## 安装

```
go get github.com/cloudwego/kitex/tool/cmd/kitex
```

用 go 命令来安装是最简单的，你也可以选择自己从源码构建和安装。要查看 kitex 的安装位置，可以用：

```
go list -f {{.Target}} github.com/cloudwego/kitex/tool/cmd/kitex
```

**注意**，由于 kitex 会为自身的二进制文件创建软链接，因此请确保 kitex 的安装路径具有可写权限。

## 依赖与运行模式

### 底层编译器

要使用 thrift 或 protobuf 的 IDL 生成代码，需要安装相应的编译器：[thriftgo](https://github.com/cloudwego/thriftgo) 或 [protoc](https://github.com/protocolbuffers/protobuf/releases)。

kitex 生成的代码里，一部分是底层的编译器生成的（通常是关于 IDL 里定义的结构体的编解码逻辑），另一部分是用于连接 Kitex 框架与生成代码并提供给终端用户良好界面的胶水代码。

从执行流上来说，当 kitex 被要求来给一个 thrift IDL 生成代码时，kitex 会调用 thriftgo 来生成结构体编解码代码，并将自身作为 thriftgo 的一个插件（名为 thrift-gen-kitex）来执行来生成胶水代码。当用于 protobuf IDL 时亦是如此。

而以哪个名字运行，决定了 kitex 的执行模式是命令行入口还是一个插件（thriftgo 的或 protoc 的），所以 kitex 的安装目录需要具备可写权限，用于 kitex 建立必要的软链接以区分不同的执行模式。

```
$> kitex IDL
    |
    | thrift-IDL
    |---------> thriftgo --gen go:... -plugin=... -out ... IDL
    |        |
    |        -----> thrift-gen-kitex (symbol link to kitex)
    |
    | protobuf-IDL
    ----------> protoc --kitex_out=... --kitex_opt=... IDL
             |
             -----> protoc-gen-kitex (symbol link to kitex)
```

### 库依赖

kitex 生成的代码会依赖相应的 Go 语言代码库：

- 对于 thrift IDL，是 `github.com/apache/thrift v0.13.0`
- 对于 protobuf IDL，是 ` google.golang.org/protobuf v1.26.0`

要注意的一个地方是，`github.com/apache/thrift/lib/go/thrift` 的 `v0.14.0` 版本开始提供的 API 和之前的版本是**不兼容的**，如果在更新依赖的时候给 `go get` 命令增加了 `-u` 选项，会导致该库更新到不兼容的版本造成编译失败。你可以通过额外执行一个命令来指定正确的版本：

```
go get github.com/apache/thrift@v0.13.0
```

或用 `replace` 指令强制固定该版本： 

```
go mod edit -replace github.com/apache/thrift=github.com/apache/thrift@v0.13.0
```

### 使用 protobuf IDL 的注意事项

kitex 仅支持 protocol buffers 的 [proto3](https://developers.google.com/protocol-buffers/docs/proto3) 版本的语法。

IDL 里的 `go_package` 是必需的，其值可以是一个用点（`.`）或斜线（`/`）分隔的包名序列，决定了生成的 import path 的后缀。例如

```
option go_package = "hello.world"; // or hello/world
```

生成的 import path 会是 `${当前目录的 import path}/kitex_gen/hello/world`。


## 使用

### 基础使用

语法：`kitex [options] IDL`

下面以 thrift IDL 作为例子。 Protobuf IDL 的使用也是类似的。

#### 生成客户端代码

```
kitex path_to_your_idl.thrift
```

执行后在当前目录下会生成一个名为 kitex_gen 目录，其中包含了 IDL 定义的数据结构，以及与 IDL 里定义的 service 相对应的 `*service` 包，提供了创建这些 service 的 client API。

#### 生成服务端代码

```
kitex -service service_name path_to_your_idl.thrift
```

执行后在当前目录下会生成一个名为 kitex_gen 目录，同时包含一些用于创建和运行一个服务的脚手架代码。具体见[生成代码的结构](#生成代码的结构)一节的描述。

### 生成代码的结构

假设我们有两个 thrift IDL 文件，demo.thrift 和 base.thrift，其中 demo.thrift 依赖了 base.thrift，并且 demo.thrift 里定义了一个名为 `demo` 的 service 而 base 里没有 service 定义。

那么在一个空目录下，`kitex -service demo demo.thrift` 生成的结果如下：

```
.
├── build.sh                     // 服务的构建脚本，会创建一个名为 output 的目录并生成启动服务所需的文件到里面
├── conf                         // 配置文件的目录，该目录下的文件会被 build.sh 拷贝到 output/conf 下
│   └── kitex.yml                // 默认的配置文件
├── handler.go                   // 用户在该文件里实现 IDL service 定义的方法
├── kitex_gen                    // IDL 内容相关的生成代码
│   ├── base                     // base.thrift 的生成代码
│   │   ├── base.go              // thriftgo 的产物，包含 base.thrift 定义的内容的 go 代码
│   │   └── k-base.go            // kitex 在 thriftgo 的产物之外生成的代码
│   └── demo                     // demo.thrift 的生成代码
│       ├── demo.go              // thriftgo 的产物，包含 demo.thrift 定义的内容的 go 代码
│       ├── k-demo.go            // kitex 在 thriftgo 的产物之外生成的代码
│       └── demoservice          // kitex 为 demo.thrift 里定义的 demo service 生成的代码
│           ├── demoservice.go   // 提供了 client.go 和 server.go 共用的一些定义
│           ├── client.go        // 提供了 NewClient API
│           └── server.go        // 提供了 NewServer API
├── main.go                      // 程序入口
└── script                       // 构建脚本
    └── bootstrap.sh             // 服务的启动脚本，会被 build.sh 拷贝至 output 下
```

如果不指定 `-service` 参数，那么生成的只有 kitex_gen 目录。

### 选项

本文描述的选项可能会过时，可以用 `kitex -h` 或 `kitex --help` 来查看 kitex 的所有可用的选项。

#### `-service service_name`

使用该选项时，kitex 会生成构建一个服务的脚手架代码，参数 `service_name` 给出启动时服务自身的名字，通常其值取决于使用 Kitex 框架时搭配的服务注册和服务发现功能。

#### `-module module_name`

该参数用于指定生成代码所属的 Go 模块，会影响生成代码里的 import path。

1. 如果当前目录是在 `$GOPATH/src` 下的一个目录，那么可以不指定该参数；kitex 会使用 `$GOPATH/src` 开始的相对路径作为 import path 前缀。例如，在 `$GOPATH/src/example.com/hello/world` 下执行 kitex，那么 `kitex_gen/example_package/example_package.go` 在其他代码代码里的 import path 会是 `example.com/hello/world/kitex_gen/example_package`。
2. 如果当前目录不在 `$GOPATH/src` 下，那么必须指定该参数。
3. 如果指定了 `-module` 参数，那么 kitex 会从当前目录开始往上层搜索 go.mod 文件

    - 如果不存在 go.mod 文件，那么 kitex 会调用 `go mod init` 生成 go.mod；
    - 如果存在 go.mod 文件，那么 kitex 会检查 `-module` 的参数和 go.mod 里的模块名字是否一致，如果不一致则会报错退出；
    - 最后，go.mod 的位置及其模块名字会决定生成代码里的 import path。

#### `-I path`

添加一个 IDL 的搜索路径。

#### `-type type`

指明当前使用的 IDL 类型，当前可选的值有 `thrift`（默认）和 `protobuf`。

#### `-v` 或 `-verbose`

输出更多日志。

### 高级选项

#### `-use path`

在生成服务端代码（使用了 `-service`）时，可以用 `-use` 选项来让 kitex 不生成 kitex_gen 目录，而使用该选项给出的 import path。

#### `-combine-service`

对于 thrift IDL，kitex 在生成服务端代码脚手架时，只会针对最后一个 service 生成相关的定义。如果你的 IDL 里定义了多个 service 定义并且希望在一个服务里同时提供这些 service 定义的能力时，可以使用 `-combine-service` 选项。

该选项会生成一个合并了目标 IDL 文件中所有 service 方法的 `CombineService`，并将其用作 main 包里使用的 service 定义。注意这个模式下，被合并的 service 之间不能有冲突的方法名。

#### `-protobuf value`

传递给 protoc 的参数。会拼接在 `-go_out` 的参数后面。可用的值参考 `protoc` 的帮助文档。

#### `-thrift value`

传递给 thriftgo 的参数。会拼接在 `-g go:` 的参数后面。可用的值参考 `thriftgo` 的帮助文档。kitex 默认传递了 `naming_style=golint,ignore_initialisms,gen_setter,gen_deep_equal`，可以被覆盖。



