# HTTPResolver

## Specify RPC URL

You can use `callopt.WithURL` to specify a URL, which will be resolved by default DNS resolver to get host and port. It's functionally equal to `callopt.WithHostPort`。

```go
import "github.com/jackedelic/kitex/internal/client/callopt"
...
url := callopt.WithURL("http://myserverdomain.com:8888")
resp, err := cli.Echo(context.Background(), req, url)
if err != nil {
	log.Fatal(err)
}
```

## Customized DNS resolver

You can also use your own DNS resolver

resolver interface(pkg/http)：

```go
type Resolver interface {
	Resolve(string) (string, error)
}
```

The only parameter is URL，return value should be "host:port".

You can use `client.WithHTTPResolver` to specify DNS resolver.

```go
import "github.com/jackedelic/kitex/internal/client/callopt"
...
dr := client.WithHTTPResolver(myResolver)
cli, err := echo.NewClient("echo", dr)
if err != nil {
	log.Fatal(err)
}
```
