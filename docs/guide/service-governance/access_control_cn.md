
# 自定义访问控制

Kitex 框架提供了一个简单的中间件构造器，可以支持用户自定义访问控制的逻辑，在特定条件下拒绝请求。下面是一个简单的例子，随机拒绝1%的请求：

```go
package myaccesscontrol

import (
    "math/rand"
    "github.com/cloudwego/kitex/pkg/acl"
)

var errRejected = errors.New("1% rejected")

// 实现一个判断函数
func reject1percent(ctx context.Context, request interface{}) (reason error) {
    if rand.Intn(100) == 0 {
        return errRejected // 拒绝请求时，需要返回一个错误
    }
    return nil
}

var MyMiddleware = acl.NewACLMiddleware(reject1percent) // 创建中间件
```

随后，你可以在创建 client 或者 server 的时候，通过 `WithMiddleware(myaccesscontrol.MyMiddleware)` 启用该中间件。

