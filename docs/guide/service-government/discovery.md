# Service Discovery

Currently in the Kitex open source release, only one service discovery extension is supported: DNS Resolver.

DNS Resolver is suitable for the clusters where DNS is used as a service discovery, commonly used for [Kubernetes](https://kubernetes.io/) clusters.

Extended repository: [Extended Repository](https://github.com/kitex-contrib)

## Usage

```go
import (
    "github.com/kitex-contrib/dns-resolver"
    kClient "github.com/cloudwego/kitex/client"
)

...
    // init client with dns resolver
	client, _ := testClient.NewClient("DestServiceName", kClient.WithResolver(dns.NewDNSResolver()))
...
```
