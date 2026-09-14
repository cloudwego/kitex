/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package client

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

	"github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/utils"
)

// WithHTTPProxy sets an HTTP CONNECT proxy for the client.
//
// The proxy address should be in one of the following formats:
//   - "http://proxy.example.com:3128" (plain HTTP proxy)
//   - "https://proxy.example.com:443" (HTTPS proxy, TLS to proxy itself)
//   - "http://user:pass@proxy.example.com:3128" (with Basic Auth)
//   - "proxy.example.com:3128" (scheme omitted, defaults to http)
//
// HTTP CONNECT proxy establishes a transparent TCP tunnel between the client
// and the target server. Default Thrift unary transports use gonet with buffered
// connections. gRPC supports both h2c and TLS to the target; configure target TLS
// separately with WithGRPCTLSConfig.
//
// This is useful when the client needs to access external services through
// an enterprise HTTP proxy, or when the network environment requires all
// outbound traffic to go through a proxy.
//
// This option conflicts with WithHTTPProxyConfig, WithDialer, and WithProxy
// (Mesh ForwardProxy). HTTP, mux, custom transport handlers, and TTHeaderStreaming
// are unsupported. Conflicting combinations panic during client construction,
// independently of option order.
//
// Example:
//
//	cli, err := myservice.NewClient(
//	    "my.service",
//	    client.WithTransportProtocol(transport.GRPC),
//	    client.WithHTTPProxy("http://proxy.company.com:3128"),
//	)
func WithHTTPProxy(addr string) Option {
	return Option{F: func(o *client.Options, di *utils.Slice) {
		o.Once.OnceOrPanic()
		di.Push(fmt.Sprintf("WithHTTPProxy(%s)", maskProxyAddrForLog(addr)))

		cfg, err := parseHTTPProxyAddr(addr)
		if err != nil {
			panic(fmt.Errorf("invalid HTTP proxy address: %w", err))
		}

		checkProxyConflict(o)
		o.HTTPProxyEnabled = true
		o.RemoteOpt.Dialer = proxy.NewHTTPConnectDialer(cfg)
	}}
}

// WithHTTPProxyConfig sets an HTTP CONNECT proxy with detailed configuration.
//
// This provides more control than WithHTTPProxy, including:
//   - Proxy TLS configuration (for HTTPS proxies)
//   - Connect timeout
//   - NoProxy rules (targets that should bypass the proxy)
//   - Custom underlying dialer (for connecting to the proxy itself)
//
// Supported transports and restrictions are the same as WithHTTPProxy. This
// option conflicts with WithHTTPProxy, WithDialer, and WithProxy in either order.
// ConnectTimeout caps the complete proxy handshake; a smaller positive caller
// timeout takes precedence. A nonpositive ConnectTimeout defaults to 10 seconds.
//
// Example:
//
//	cli, err := myservice.NewClient(
//	    "my.service",
//	    client.WithTransportProtocol(transport.GRPC),
//	    client.WithHTTPProxyConfig(proxy.HTTPConnectConfig{
//	        Address:        "proxy.company.com:3128",
//	        Username:       "user",
//	        Password:       "pass",
//	        ConnectTimeout: 15 * time.Second,
//	        NoProxy:        []string{"10.0.0.0/8", ".internal"},
//	    }),
//	)
func WithHTTPProxyConfig(cfg proxy.HTTPConnectConfig) Option {
	if cfg.Type == proxy.ProxyDirect {
		cfg.Type = proxy.ProxyHTTPConnect
	}
	return Option{F: func(o *client.Options, di *utils.Slice) {
		o.Once.OnceOrPanic()
		di.Push(fmt.Sprintf("WithHTTPProxyConfig(address=%s)", cfg.Address))

		if cfg.Address == "" {
			panic("invalid HTTP proxy config: address is empty")
		}
		checkProxyConflict(o)
		o.HTTPProxyEnabled = true
		o.RemoteOpt.Dialer = proxy.NewHTTPConnectDialer(cfg)
	}}
}

// checkProxyConflict checks for conflicts between HTTP proxy, custom Dialer,
// and Mesh ForwardProxy. These options are mutually exclusive.
func checkProxyConflict(o *client.Options) {
	if o.HTTPProxyEnabled || o.DialerExplicitlySet {
		panic("WithHTTPProxy and WithHTTPProxyConfig conflict with each other and WithDialer")
	}
	// Conflict with WithProxy (Mesh ForwardProxy)
	if o.Proxy != nil || o.ProxyExplicitlySet {
		panic(fmt.Sprintf(
			"WithHTTPProxy conflicts with WithProxy (Mesh ForwardProxy): "+
				"only one of them can be set. WithProxy is for Mesh Sidecar Egress, "+
				"while WithHTTPProxy is for HTTP CONNECT tunnel proxy. "+
				"Existing proxy type: %T", o.Proxy))
	}
}

// parseHTTPProxyAddr parses a proxy URL string into HTTPConnectConfig.
func parseHTTPProxyAddr(addr string) (proxy.HTTPConnectConfig, error) {
	// Add scheme if missing. A valid URL scheme is followed by "://",
	// so we check for that pattern to distinguish "host:port" from "scheme://host".
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}

	u, err := url.Parse(addr)
	if err != nil {
		return proxy.HTTPConnectConfig{}, err
	}
	if u.Host == "" {
		return proxy.HTTPConnectConfig{}, fmt.Errorf("missing host in proxy address: %s", addr)
	}

	cfg := proxy.HTTPConnectConfig{
		Type:    proxy.ProxyHTTPConnect,
		Address: u.Host,
	}
	if u.User != nil {
		cfg.Username = u.User.Username()
		cfg.Password, _ = u.User.Password()
	}
	if u.Scheme == "https" {
		cfg.ProxyTLS = &tls.Config{}
	}
	return cfg, nil
}

// containsScheme checks if the address contains a URL scheme (e.g., "http://", "https://").
// A valid scheme is followed by "://", which distinguishes it from "host:port".
func containsScheme(addr string) bool {
	return strings.Contains(addr, "://")
}

// maskProxyAddrForLog masks the password in a proxy address for logging.
func maskProxyAddrForLog(addr string) string {
	if !containsScheme(addr) {
		addr = "http://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil || u.User == nil {
		return addr
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		// Manually construct the masked URL to avoid URL-encoding the asterisks
		return u.Scheme + "://" + u.User.Username() + ":******@" + u.Host
	}
	return u.Scheme + "://" + u.Host
}
