# Kitex 泛化调用使用指南

目前仅支持 Thrift 泛化调用，通常用于不需要生成代码的中台服务。

## 支持场景

1. 二进制泛化调用：用于流量中转场景
2. HTTP映射泛化调用：用于 API 网关场景
3. Map映射泛化调用

## 使用方式示例

### 1. 二进制泛化调用

#### 调用端使用

应用场景：比如中台服务，可以通过二进制流转发将收到的原始 Thrift 协议包发给目标服务。

- 初始化Client

  ```go
  import (
     "github.com/jackedelic/kitex/internal/client/genericclient"
     "github.com/jackedelic/kitex/internal/pkg/generic"
  )
  func NewGenericClient(psm string) genericclient.Client {
      genericCli := genericclient.NewClient(psm, generic.BinaryThriftGeneric())
      return genericCli
  }
  ```

- 泛化调用

  若自行编码，需要使用 Thrift 编码格式 [thrift/thrift-binary-protocol.md](https://github.com/apache/thrift/blob/master/doc/specs/thrift-binary-protocol.md#message)。注意，二进制编码不是对原始的 Thrift 请求参数编码，是 method 参数封装的 **XXXArgs**。可以参考 github.com/jackedelic/kitex/internal/generic/generic_test.go。

  Kitex 提供了 thrift 编解码包`github.com/jackedelic/kitex/internal/pkg/utils.NewThriftMessageCodec`。
  
  ```go
  rc := utils.NewThriftMessageCodec()
  buf, err := rc.Encode("Test", thrift.CALL, 100, args)
  // generic call
  resp, err := genericCli.GenericCall(ctx, "actualMethod", buf)
  ```

#### 服务端使用

​	二进制泛化 Client 和 Server **并不是配套**使用的，Client 传入**正确的 Thrift 编码二进制**，可以访问普通的 Thrift Server。

​    二进制泛化 Server 只支持 Framed 或 TTHeader 请求，不支持 Bufferd Binary，需要 Client 通过 Option 指定，如：`client.WithTransportProtocol(transport.Framed)`。

```go
package main

import (
    "github.com/jackedelic/kitex/internal/pkg/generic"
    "github.com/jackedelic/kitex/internal/server/genericserver"
)

func main() {
    g := generic.BinaryThriftGeneric()
    svr := genericserver.NewServer(&GenericServiceImpl{}, g)
    err := svr.Run()
    if err != nil {
            panic(err)
    }
}

type GenericServiceImpl struct {}

// GenericCall ...
func (g *GenericServiceImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
    // request is thrift binary
    reqBuf := request.([]byte)
    // e.g. 
    fmt.Printf("Method: %s\n", method))
    result := xxx.NewMockTestResult()
    result.Success = &resp
    respBuf, err = rc.Encode(mth, thrift.REPLY, seqID, result)
    
    return respBuf, nil
}
```

### 2. HTTP 映射泛化调用

HTTP 映射泛化调用只针对客户端，要求 Thrift IDL 遵从接口映射规范，具体规范见 [ByteAPI IDL规范](TODO)（待整理）。

#### IDL 定义示例

```thrift
namespace go http

struct ReqItem {
    1: optional i64 id(go.tag = "json:\"id\"")
    2: optional string text
}

struct BizRequest {
    1: optional i64 v_int64(api.query = 'v_int64', api.vd = "$>0&&$<200")
    2: optional string text(api.body = 'text')
    3: optional i32 token(api.header = 'token')
    4: optional map<i64, ReqItem> req_items_map (api.body='req_items_map')
    5: optional ReqItem some(api.body = 'some')
    6: optional list<string> req_items(api.query = 'req_items')
    7: optional i32 api_version(api.path = 'action')
    8: optional i64 uid(api.path = 'biz')
    9: optional list<i64> cids(api.query = 'cids')
    10: optional list<string> vids(api.query = 'vids')
}

struct RspItem {
    1: optional i64 item_id
    2: optional string text
}

struct BizResponse {
    1: optional string T                             (api.header= 'T') 
    2: optional map<i64, RspItem> rsp_items           (api.body='rsp_items')
    3: optional i32 v_enum                       (api.none = '')
    4: optional list<RspItem> rsp_item_list            (api.body = 'rsp_item_list')
    5: optional i32 http_code                         (api.http_code = '') 
    6: optional list<i64> item_count (api.header = 'item_count')
}

service BizService {
    BizResponse BizMethod1(1: BizRequest req)(api.get = '/life/client/:action/:biz', api.baseurl = 'ib.snssdk.com', api.param = 'true')
    BizResponse BizMethod2(1: BizRequest req)(api.post = '/life/client/:action/:biz', api.baseurl = 'ib.snssdk.com', api.param = 'true', api.serializer = 'form')
    BizResponse BizMethod3(1: BizRequest req)(api.post = '/life/client/:action/:biz/other', api.baseurl = 'ib.snssdk.com', api.param = 'true', api.serializer = 'json')
}
```



#### 泛化调用示例

- **Request**

类型：*generic.HTTPReqeust

- **Response**

类型：*generic.HTTPResponse

```go
package main

import (
    "github.com/jackedelic/kitex/internal/client/genericclient"
    "github.com/jackedelic/kitex/internal/pkg/generic"
)

func main() {
    // 本地文件idl解析
    // YOUR_IDL_PATH thrift文件路径: 举例 ./idl/example.thrift
    // includeDirs: 指定include路径，默认用当前文件的相对路径寻找include
    p, err := generic.NewThriftFileProvider("./YOUR_IDL_PATH")
    if err != nil {
        panic(err)
    }
    // 构造http类型的泛化调用
    g, err := generic.HTTPThriftGeneric(p)
    if err != nil {
        panic(err)
    }
    cli, err := genericclient.NewClient("psm", g, opts...)
    if err != nil {
        panic(err)
    }
    // 构造request，或者从ginex获取
    body := map[string]interface{}{
        "text": "text",
        "some": map[string]interface{}{
            "id":   1,
            "text": "text",
        },
        "req_items_map": map[string]interface{}{
            "1": map[string]interface{}{
                "id":   1,
                "text": "text",
            },
        },
    }
    data, err := json.Marshal(body)
    if err != nil {
        panic(err)
    }
    url := "http://example.com/life/client/1/1?v_int64=1&req_items=item1,item2,itme3&cids=1,2,3&vids=1,2,3"
    req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
    if err != nil {
        panic(err)
    }
    req.Header.Set("token", "1")
    customReq, err := generic.FromHTTPRequest(req) // 考虑到业务有可能使用第三方http request，可以自行构造转换函数
    // customReq *generic.HttpRequest
    // 由于http泛化的method是通过bam规则从http request中获取的，所以填空就行
    resp, err := cli.GenericCall(ctx, "", customReq)
    realResp := resp.(*generic.HttpResponse)
    realResp.Write(w) // 写回ResponseWriter，用于http网关
}
```

#### 注解扩展

比如增加一个 `xxx.source='not_body_struct'` 注解，表示某个字段本身没有对 HTTP 请求字段的映射，需要遍历其子字段从 HTTP 请求中获取对应的值。使用方式如下：

```thrift
struct Request {
    1: optional i64 v_int64(api.query = 'v_int64')
    2: optional CommonParam common_param (xxx.source='not_body_struct')
}

struct CommonParam {
    1: optional i64 api_version (api.query = 'api_version')
    2: optional i32 token(api.header = 'token')
}
```

扩展方式如下：

```go
func init() {
        descriptor.RegisterAnnotation(new(notBodyStruct))
}

// 实现descriptor.Annotation
type notBodyStruct struct {
}

func (a * notBodyStruct) Equal(key, value string) bool {
        return key == "xxx.source" && value == "not_body_struct"
}

// Handle 目前支持四种类型：HttpMapping, FieldMapping, ValueMapping, Router
func (a * notBodyStruct) Handle() interface{} {
        return newNotBodyStruct
}

type notBodyStruct struct{}

var newNotBodyStruct descriptor.NewHttpMapping = func(value string) descriptor.HttpMapping {
        return &notBodyStruct{}
}

// get value from request
func (m *notBodyStruct) Request(req *descriptor.HttpRequest, field *descriptor.FieldDescriptor) (interface{}, bool) {
        // not_body_struct 注解的作用相当于 step into，所以直接返回req本身，让当前filed继续从Request中查询所需要的值
        return req, true
}

// set value to response
func (m *notBodyStruct) Response(resp *descriptor.HttpResponse, field *descriptor.FieldDescriptor, val interface{}) {
}
```

### 3. Map 映射泛化调用

Map 映射泛化调用是指用户可以直接按照规范构造 Map 请求参数或返回，Kitex 会对应完成 Thrift 编解码。

#### Map 构造

Kitex 会根据给出的 IDL 严格校验用户构造的字段名和类型，字段名只支持字符串类型对应 Map Key，字段 Value 的类型映射见类型映射表。

对于Response会校验 Field ID 和类型，并根据 IDL 的 Field Name 生成相应的 Map Key。

##### 类型映射

Golang 与 Thrift IDL 类型映射如下：

| **Golang 类型**             | **Thrift IDL 类型** |
| --------------------------- | ------------------- |
| bool                        | bool                |
| int8                        | i8                  |
| int16                       | i16                 |
| int32                       | i32                 |
| int64                       | i64                 |
| float64                     | double              |
| string                      | string              |
| []byte                      | binary              |
| []interface{}               | list/set            |
| map[interface{}]interface{} | map                 |
| map[string]interface{}      | struct              |
| int32                       | enum                |

##### 示例

以下面的 IDL 为例：

```thrift
enum ErrorCode {
    SUCCESS = 0
    FAILURE = 1
}

struct Info {
    1: map<string,string> Map
    2: i64 ID
}

struct EchoRequest {
    1: string Msg
    2: i8 I8
    3: i16 I16
    4: i32 I32
    5: i64 I64
    6: binary Binary
    7: map<string,string> Map
    8: set<string> Set
    9: list<string> List
    10: ErrorCode ErrorCode
    11: Info Info

    255: optional Base Base
}
```

构造请求如下：

```go
req := map[string]interface{}{
                "Msg":    "hello",
                "I8":     int8(1),
                "I16":    int16(1),
                "I32":    int32(1),
                "I64":    int64(1),
                "Binary": []byte("hello"),
                "Map": map[interface{}]interface{}{
                        "hello": "world",
                },
                "Set":       []interface{}{"hello", "world"},
                "List":      []interface{}{"hello", "world"},
                "ErrorCode": int32(1),
                "Info": map[string]interface{}{
                        "Map": map[interface{}]interface{}{
                                "hello": "world",
                        },
                        "ID": int64(232324),
                },
        }
```

#### 泛化调用示例

示例 IDL ：

`base.thrift`

```thrift
namespace py base
namespace go base
namespace java com.xxx.thrift.base

struct TrafficEnv {
    1: bool Open = false,
    2: string Env = "",
}

struct Base {
    1: string LogID = "",
    2: string Caller = "",
    3: string Addr = "",
    4: string Client = "",
    5: optional TrafficEnv TrafficEnv,
    6: optional map<string, string> Extra,
}

struct BaseResp {
    1: string StatusMessage = "",
    2: i32 StatusCode = 0,
    3: optional map<string, string> Extra,
}
```

`example_service.thrift`

```go
include "base.thrift"
namespace go kitex.test.server

struct ExampleReq {
    1: required string Msg,
    255: base.Base Base,
}
struct ExampleResp {
    1: required string Msg,
    255: base.BaseResp BaseResp,
}
service ExampleService {
    ExampleResp ExampleMethod(1: ExampleReq req),
}
```

##### 客户端使用

- **Request**

类型：map[string]interface{}

- **Response**

类型：map[string]interface{}

```go
package main

import (
    "github.com/jackedelic/kitex/internal/pkg/generic"
    "github.com/jackedelic/kitex/internal/client/genericclient"
)

func main() {
    // 本地文件idl解析
    // YOUR_IDL_PATH thrift文件路径: 举例 ./idl/example.thrift
    // includeDirs: 指定include路径，默认用当前文件的相对路径寻找include
    p, err := generic.NewThriftFileProvider("./YOUR_IDL_PATH")
    if err != nil {
        panic(err)
    }
    // 构造map 请求和返回类型的泛化调用
    g, err := generic.MapThriftGeneric(p)
    if err != nil {
        panic(err)
    }
    cli, err := genericclient.NewClient("psm", g, opts...)
    if err != nil {
        panic(err)
    }
    // 'ExampleMethod' 方法名必须包含在idl定义中
    resp, err := cli.GenericCall(ctx, "ExampleMethod", map[string]interface{}{
        "Msg": "hello",
    })
    // resp is a map[string]interface{}
}
```

##### 服务端使用

- **Request**

类型：map[string]interface{}

- **Response**

类型：map[string]interface{}

```go
package main

import (
    "github.com/jackedelic/kitex/internal/pkg/generic"
    "github.com/jackedelic/kitex/internal/server/genericserver"
)

func main() {
    // 本地文件idl解析
    // YOUR_IDL_PATH thrift文件路径: e.g. ./idl/example.thrift
    p, err := generic.NewThriftFileProvider("./YOUR_IDL_PATH")
    if err != nil {
        panic(err)
    }
    // 构造map请求和返回类型的泛化调用
    g, err := generic.MapThriftGeneric(p)
    if err != nil {
        panic(err)
    }
    svc := genericserver.NewServer(new(GenericServiceImpl), g, opts...)
    if err != nil {
        panic(err)
    }
    err := svr.Run()
    if err != nil {
        panic(err)
    }
    // resp is a map[string]interface{}
}

type GenericServiceImpl struct {
}

func (g *GenericServiceImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
        m := request.(map[string]interface{})
        fmt.Printf("Recv: %v\n", m)
        return  map[string]interface{}{
            "Msg": "world",
        }, nil
}

```

## IDLProvider

HTTP/Map 映射的泛化调用虽然不需要生成代码，但需要使用者提供 IDL。

目前 Kitex 有两种 IDLProvider 实现，使用者可以选择指定 IDL 路径，也可以选择传入 IDL 内容。当然也可以根据需求自行扩展 `generci.DescriptorProvider`。

### 基于本地文件解析 IDL

```go
p, err := generic.NewThriftFileProvider("./YOUR_IDL_PATH")
 if err != nil {
     panic(err)
 }
```

### 基于内存解析 IDL

所有 IDL 需要构造成 Map ，Key 是 Path，Value 是 IDL 定义，使用方式如下：

```go
p, err := generic.NewThriftContentProvider("YOUR_MAIN_IDL_CONTENT", map[string]string{/*YOUR_INCLUDES_IDL_CONTENT*/})
    if err != nil {
        panic(err)
    }

// dynamic update
err = p.UpdateIDL("YOUR_MAIN_IDL_CONTENT", map[string]string{/*YOUR_INCLUDES_IDL_CONTENT*/})
if err != nil {
    // handle err
}
```

简单实例（为最小化展示 Path 构造，并非真实的 IDL）：

```go
path := "a/b/main.thrift"
content := `
namespace go kitex.test.server
include "x.thrift"
include "../y.thrift" 

service InboxService {}
`
includes := map[string]string{
   path:           content,
   "x.thrift": "namespace go kitex.test.server",
   "../y.thrift": `
   namespace go kitex.test.server
   include "z.thrift"
   `,
}

p, err := NewThriftContentProvider(path, includes)
```



#### 支持绝对路径的 include path 寻址

若为方便构造 IDL Map，也可以通过 `NewThriftContentWithAbsIncludePathProvider` 使用绝对路径作为 Key。

```go
p, err := generic.NewThriftContentWithAbsIncludePathProvider("YOUR_MAIN_IDL_PATH", "YOUR_MAIN_IDL_CONTENT", map[string]string{"ABS_INCLUDE_PATH": "CONTENT"})
    if err != nil {
        panic(err)
    }

// dynamic update
err = p.UpdateIDL("YOUR_MAIN_IDL_PATH", "YOUR_MAIN_IDL_CONTENT", map[string]string{/*YOUR_INCLUDES_IDL_CONTENT*/})
if err != nil {
    // handle err
}
```

简单实例（为最小化展示 Path 构造，并非真实的 IDL）：

```go
path := "a/b/main.thrift"
content := `
namespace go kitex.test.server
include "x.thrift"
include "../y.thrift" 

service InboxService {}
`
includes := map[string]string{
   path:           content,
   "a/b/x.thrift": "namespace go kitex.test.server",
   "a/y.thrift": `
   namespace go kitex.test.server
   include "z.thrift"
   `,
   "a/z.thrift": "namespace go kitex.test.server",
}
p, err := NewThriftContentWithAbsIncludePathProvider(path, includes)
```
