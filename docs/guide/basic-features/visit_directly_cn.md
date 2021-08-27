# 直连访问

在明确要访问某个下游地址时，可以选择直连访问的方式，不需要经过服务发现。

client 可以有两种形式指定下游地址：

- 使用 `WithHostPort` Option，支持两种参数：
    - 普通 IP 地址，形式为 "host:port"，支持 IPv6
    - sock 文件地址，通过 UDS (Unix Domain Socket) 通信
- 使用 `WithURL` Option，参数为合法的 HTTP URL 地址
