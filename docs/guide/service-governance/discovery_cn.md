# 服务发现

目前在 Kitex 的开源版本中，暂时只提供了一种服务发现的扩展支持 : DNS Resolver, 适合使用 DNS 作为服务发现的场景， 常见的用于 [Kubernetes](https://kubernetes.io/) 集群。
扩展库：[扩展仓库](https://github.com/kitex-contrib)

## 使用方式

```go
import (
    "github.com/kitex-contrib/dns-resolver"
    kClient "github.com/jackedelic/kitex/internal/client"
)

...
    // init client with dns resolver
	client, _ := testClient.NewClient("DestServiceName", kClient.WithResolver(dns.NewDNSResolver()))
...
```
