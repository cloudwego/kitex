# HTTP CONNECT proxy support

`client.WithHTTPProxy` and `client.WithHTTPProxyConfig` route client connections through an explicit HTTP CONNECT proxy. Service discovery and load balancing resolve the target address normally. Each new connection sends `CONNECT target:port`; pooled connections reuse the established tunnel.

## Supported client transports

| Transport | Behavior |
| --- | --- |
| Default Thrift unary, Framed, TTHeader, TTHeaderFramed | Selects the gonet client transport and adapts each tunnel to its buffered reader/writer interface. |
| gRPC | Uses the existing HTTP/2 transport over the tunnel, with h2c or separately configured target TLS. |
| gRPC streaming alongside Thrift unary | Retains the gRPC streaming transport and uses gonet for unary calls. |
| TTHeaderStreaming | Rejected during client construction because its provider bypasses the configured dialer. |
| HTTP, mux, explicit custom transport handlers | Rejected during client construction to avoid incompatible connection interfaces or silently replaced handlers. |

The gonet adapter also covers direct connections selected by `NoProxy`. These options do not change transport selection for clients without an HTTP proxy. A standalone `HTTPConnectDialer` returns a standard `net.Conn`; code using it directly must adapt the connection for its chosen transport.

## Configuration

```go
// Plain HTTP proxy; the default Thrift transport works without extra options.
client.WithHTTPProxy("http://proxy.example.com:3128")

// TLS to the proxy, using system roots and the proxy hostname for verification.
client.WithHTTPProxy("https://proxy.example.com:443")

// Basic authentication; a missing scheme defaults to http.
client.WithHTTPProxy("http://user:password@proxy.example.com:3128")

// Custom trust roots, timeout, and direct-route exceptions.
client.WithHTTPProxyConfig(proxy.HTTPConnectConfig{
    Address:        "proxy.example.com:443",
    ProxyTLS:       &tls.Config{RootCAs: proxyRoots},
    ConnectTimeout: 5 * time.Second,
    NoProxy:        []string{"10.0.0.0/8", ".internal"},
})
```

Proxy addresses require a port. `NoProxy` supports CIDR ranges and domain suffixes: `example.com` includes the domain itself and subdomains; `.example.com` includes subdomains only. Environment proxy variables are not read automatically. All targets use the proxy unless a `NoProxy` rule matches.

`WithHTTPProxy`, `WithHTTPProxyConfig`, `WithDialer`, and mesh `WithProxy` cannot be combined with an HTTP proxy option. Explicit per-client option state enforces conflicts in either order. Existing combinations of mesh `WithProxy` and `WithDialer` remain allowed. Options can be reused across clients; configuration normalization does not mutate the option closure during application.

To customize TCP dialing, use `HTTPConnectConfig.UnderlyingDialer`. It must honor the supplied timeout and return connections whose `Close` interrupts pending I/O. Unary RPC read timeouts require either standard `SetReadDeadline` support or the native `SetReadTimeout(time.Duration) error` capability. Custom connection wrappers must preserve one of these APIs. The CONNECT implementation exposes only `net.Conn` methods to `net/http`, preventing use of netpoll's buffered `WriteString` without a flush.

## TLS verification

Proxy TLS and target TLS use independent configurations. The proxy config is cloned for each handshake. When `ServerName` is empty, the host from `Address` is used, including an unbracketed IPv6 address. Explicit `ServerName`, trust roots, and certificate verification settings are preserved. The caller's config is unchanged.

For gRPC target TLS, configure `WithGRPCTLSConfig` with the target's trust roots and `ServerName`. The sequence is TCP to proxy, optional proxy TLS, CONNECT, target TLS, then HTTP/2 and RPCs.

Basic authentication appears only in the CONNECT request. HTTPS encrypts that request between client and proxy. URL passwords are masked in option debug information.

## Timeout and tunnel ownership

The handshake budget is the smaller positive value of the caller's connect timeout and `ConnectTimeout`. A nonpositive configured timeout defaults to 10 seconds; a nonpositive caller timeout uses that configured budget. One absolute deadline covers underlying TCP dialing, proxy TLS, writing CONNECT, and reading its response. Elapsed dial time is included. Target TLS and subsequent RPCs use their existing timeout mechanisms.

After TCP dialing, a timer closes the connection on expiry to interrupt handshake I/O, including connections that do not support `SetDeadline`. Handshake timeouts retain `net.Error` timeout classification. Successful establishment stops and synchronizes with the timer before returning the tunnel; the proxy adds no lasting I/O deadline. Failure closes the connection.

After CONNECT, gonet forwards RPC read timeouts to the underlying connection's native duration API when present, including through proxy TLS and buffered tunnel wrappers. Standard TCP connections retain absolute read deadlines. `NoProxy` connections receive the same gonet adaptation. If neither timeout API works, the transport closes the connection before decoding; an outer RPC timeout cannot leave a transport read indefinitely blocked. Duration support does not change the underlying connection's `SetReadDeadline` semantics.

A successful CONNECT response has no HTTP response body. Bytes already read after the headers belong to the tunnel and are returned before further socket reads. Rejected responses are closed immediately without draining an untrusted response body.

## Validation

Regression tests cover:

- All proxy-option conflicts in both orders using initialized client options, option reuse, and rejection of incompatible transports.
- Verified proxy TLS with inferred IP identity, explicit hostname override, incorrect identity, and untrusted certificates.
- Stalled proxy TLS, CONNECT writes, response headers, and rejection bodies; caller/configured timeout minima; zero caller timeout; total elapsed dialing time; and tunnel use after the former handshake deadline.
- Post-CONNECT blackholes with a netpoll underlying dialer through plain HTTP, proxy TLS, and direct `NoProxy`: transport reads time out and underlying connections close. Successful RPCs reuse connections on the same paths.
- Coalesced CONNECT headers and an HTTP/2 SETTINGS frame, normal echo traffic, and a real netpoll underlying dialer.
- Real serialized Thrift RPCs through HTTP and HTTPS proxies, direct `NoProxy` calls, connection reuse, and gRPC RPCs using h2c, target TLS, and TLS at both proxy and target. Target TLS tests use a local TLS terminator in front of an actual Kitex server.

No external proxy deployment is required by these tests. SOCKS5, proxy chaining, PAC, and TTHeaderStreaming support are outside this change.
