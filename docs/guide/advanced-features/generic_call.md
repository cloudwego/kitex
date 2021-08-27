# Kitex Generic Call Guide

Generic call is typically used for mid-platform services that do not need generated code, and only Thrift generic call is supported currently.

## Supported Scenarios

1. Binary Generic Call: for traffic transit scenario
2. HTTP Mapping Generic Call: for API Gateway scenario
3. Map Mapping Generic Call

## Example of Usage

### 1. Binary Generic

#### Client Usage

Application scenario: mid-platform services can forward the received original Thrift protocol packets to the target miscoservice through Binary Forwarding.

- Client Initialization

  ```go
  import (
     "github.com/cloudwego/kitex/client/genericclient"
     "github.com/cloudwego/kitex/pkg/generic"
  )
  func NewGenericClient(psm string) genericclient.Client {
      genericCli := genericclient.NewClient(psm, generic.BinaryThriftGeneric())
      return genericCli
  }
  ```

- Generic Call

  If you encode by yourself, you have to use Thrift serialization protocol [thrift/thrift-binary-protocol.md](https://github.com/apache/thrift/blob/master/doc/specs/thrift-binary-protocol.md#message). Note that you shouldn't encode original function parameter, but the **XXXArgs** which wraps function parameters. You can refer to github.com/cloudwego/kitex/generic/generic_test.go.
  
  Kitex provides a thrift codec package `github.com/cloudwego/kitex/pkg/utils.NewThriftMessageCodec`.
  
  ```go
  rc := utils.NewThriftMessageCodec()
  buf, err := rc.Encode("Test", thrift.CALL, 100, args)
  // generic call
  resp, err := genericCli.GenericCall(ctx, "actualMethod", buf)
  ```

#### Server Usage

It is not necessary to use Client and Server of Binary Generic Call together. Binary Generic Client can access normal Thrift Server if the correct Thrift encoded binary is passed.

The server just supports request with length header like Framed and TTheader, Bufferd Binary is not ok. So the client has to specify the transport protocol with option, eg: client.WithTransportProtocol(transport.Framed).

```go
package main

import (
    "github.com/cloudwego/kitex/pkg/generic"
    "github.com/cloudwego/kitex/server/genericserver"
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

### 2. HTTP Mapping Generic Call

The HTTP Mapping Generic Call is only for the client, and requires Thrift IDL to comply with the interface mapping specification. See the specific specification[ByteAPI IDL specification](https://bytedance.feishu.cn/docs/doccn4wmKylFJl0OpTCEkf)(To be sorted out)

#### IDL Definition Example

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



#### Generic Call Example

- **Request**

Type: *generic.HTTPReqeust

- **Response**

Type: *generic.HTTPResponse

```go
package main

import (
    "github.com/cloudwego/kitex/client/genericclient"
    "github.com/cloudwego/kitex/pkg/generic"
)

func main() {
    // Parse IDL with Local Files
	// YOUR_IDL_PATH thrift file path, eg: ./idl/example.thrift
    // includeDirs: specify include path
    p, err := generic.NewThriftFileProvider("./YOUR_IDL_PATH")
    if err != nil {
        panic(err)
    }
    g, err := generic.HTTPThriftGeneric(p)
    if err != nil {
        panic(err)
    }
    cli, err := genericclient.NewClient("psm", g, opts...)
    if err != nil {
        panic(err)
    }
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
    url := "http://example.com/1/1?v_int64=1&req_items=item1,item2,itme3&cids=1,2,3&vids=1,2,3"
    req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
    if err != nil {
        panic(err)
    }
    req.Header.Set("token", "1")
    customReq, err := generic.FromHTTPRequest(req) 
    // customReq *generic.HttpRequest
    resp, err := cli.GenericCall(ctx, "", customReq)
    realResp := resp.(*generic.HttpResponse)
    realResp.Write(w)
}
```

#### Annotation Extension

For example, add a `xxx.source = 'not_body_struct'` annotation to indicate that a certain field itself does not have a mapping to the HTTP request fields, and you need to traverse its subfields to obtain the corresponding value from the HTTP request. The usage is as follows:

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

Extension way：

```go
func init() {
        descriptor.RegisterAnnotation(new(notBodyStruct))
}

// Implement descriptor.Annotation
type notBodyStruct struct {
}

func (a * notBodyStruct) Equal(key, value string) bool {
        return key == "xxx.source" && value == "not_body_struct"
}

// Support 4 types Handle: HttpMapping, FieldMapping, ValueMapping, Router
func (a * notBodyStruct) Handle() interface{} {
        return newNotBodyStruct
}

type notBodyStruct struct{}

var newNotBodyStruct descriptor.NewHttpMapping = func(value string) descriptor.HttpMapping {
        return &notBodyStruct{}
}

// get value from request
func (m *notBodyStruct) Request(req *descriptor.HttpRequest, field *descriptor.FieldDescriptor) (interface{}, bool) {
        return req, true
}

// set value to response
func (m *notBodyStruct) Response(resp *descriptor.HttpResponse, field *descriptor.FieldDescriptor, val interface{}) {
}
```

### 3. Map Mapping Generic Call

Map Mapping Generic Call means that the user can directly construct Map request or response according to the specification, and Kitex will do Thrift codec accordingly.

#### Build Map 

Kitex will strictly verify the field name and type constructed according to the given IDL. The field name only supports string type corresponding to the Map Key. The type mapping of the field Value is shown in the Type Mapping Table below.

Returns the Field ID and type that will verify the Response and generate the corresponding Map Key based on the Field Name of the IDL.

For response, the Field ID and Type will be verified, and return Map to user  corresponding to the IDL.

##### Type Mapping Table

The Mapping between Golang and Thrift:

| **Golang Type**             | **Thrift IDL Type** |
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

#### Example

Take the following IDL as an example:

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

The request construction is as follows:

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

#### Generic Call Example

Example IDL:

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

##### Client Usage

- **Request**

Type: map[string]interface{}

- **Response**

Type: map[string]interface{}

```go
package main

import (
    "github.com/cloudwego/kitex/pkg/generic"
    "github.com/cloudwego/kitex/client/genericclient"
)

func main() {
    // Parse IDL with Local Files
    // YOUR_IDL_PATH thrift file path, eg:./idl/example.thrift
    p, err := generic.NewThriftFileProvider("./YOUR_IDL_PATH")
    if err != nil {
        panic(err)
    }
    g, err := generic.MapThriftGeneric(p)
    if err != nil {
        panic(err)
    }
    cli, err := genericclient.NewClient("psm", g, opts...)
    if err != nil {
        panic(err)
    }
    // 'ExampleMethod' method name must be passed as param
    resp, err := cli.GenericCall(ctx, "ExampleMethod", map[string]interface{}{
        "Msg": "hello",
    })
    // resp is a map[string]interface{}
}
```

##### Server Usage

- **Request**

Type: map[string]interface{}

- **Response**

Type: map[string]interface{}

```go
package main

import (
    "github.com/cloudwego/kitex/pkg/generic"
    "github.com/cloudwego/kitex/server/genericserver"
)

func main() {
    // Parse IDL with Local Files
  	// YOUR_IDL_PATH thrift file path,eg: ./idl/example.thrift
    p, err := generic.NewThriftFileProvider("./YOUR_IDL_PATH")
    if err != nil {
        panic(err)
    }
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

Generic Call of HTTP/Map mapping does not require generated code, but require IDL which need users provide.

At present, Kitex has two IDLProvider implementations. Users can choose to specify the IDL path or pass in IDL content. Of course, you can also expand the `generci.DescriptorProvider` according to your needs.

### Parse IDL with Local Files

```go
p, err := generic.NewThriftFileProvider("./YOUR_IDL_PATH")
 if err != nil {
     panic(err)
 }
```

### Parse IDL with Memory 

All IDLs need to be constructed into a Map, Key is Path, Value is IDL definition, and the usage is as follows:

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

Simple example (not real IDL, just for minimizing display Path constructs):

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



#### Absolute Path including path Addressing

If you construct an IDL Map for convenience, you can also use an absolute path as a Key through  `NewThriftContentWithAbsIncludePathProvider` .

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

Simple example (not real IDL, just for minimizing display Path constructs):

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
