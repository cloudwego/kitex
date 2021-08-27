# HTTPResolver

## 指定 URL 进行调用

在进行调用时，可以通过`callopt.WithURL`指定，通过该 option 指定的URL，会经过默认的 DNS resolver 解析后拿到 host 和 port，此时其等效于`callopt.WithHostPort`。

```go
import "github.com/cloudwego/kitex/client/callopt"
...
url := callopt.WithURL("http://myserverdomain.com:8888")
resp, err := cli.Echo(context.Background(), req, url)
if err != nil {
	log.Fatal(err)
}
```

## 自定义 DNS resolver

此外也可以自定义 DNS resolver

resolver定义如下（pkg/http）：

```go
type Resolver interface {
	Resolve(string) (string, error)
}
```

参数为 URL，返回值为访问的 server 的 "host:port"。

通过`client.WithHTTPResolver`指定用于 DNS 解析的 resolver。

```go
import "github.com/cloudwego/kitex/client/callopt"
...
dr := client.WithHTTPResolver(myResolver)
cli, err := echo.NewClient("echo", dr)
if err != nil {
	log.Fatal(err)
}
```
