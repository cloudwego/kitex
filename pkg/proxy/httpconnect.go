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

package proxy

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/kitex/pkg/remote"
)

// ProxyType represents the type of proxy.
type ProxyType int

const (
	// ProxyDirect means no proxy, direct connection.
	ProxyDirect ProxyType = iota
	// ProxyHTTPConnect means HTTP CONNECT tunnel proxy.
	ProxyHTTPConnect
	// ProxySOCKS5 means SOCKS5 proxy (reserved for future use).
	ProxySOCKS5
)

// HTTPConnectConfig contains the configuration for HTTP CONNECT proxy.
//
// HTTP CONNECT proxy establishes a transparent TCP tunnel between the client
// and the target server through an HTTP proxy. After the tunnel is established,
// the proxy blindly forwards bytes in both directions, making it suitable for
// TLS-encrypted protocols like gRPC.
type HTTPConnectConfig struct {
	// Type is the proxy type, default is ProxyHTTPConnect.
	Type ProxyType

	// Address is the proxy server address in host:port format.
	// This field is required.
	Address string

	// Username is the proxy authentication username (Basic Auth).
	Username string

	// Password is the proxy authentication password (Basic Auth).
	Password string

	// ProxyTLS is the TLS config for the connection to the proxy server itself.
	// If nil, the connection to proxy is plain TCP. Set this when the proxy
	// is an HTTPS proxy (e.g., https://proxy.example.com:443).
	ProxyTLS *tls.Config

	// ConnectTimeout caps TCP dialing, proxy TLS, and the CONNECT exchange as
	// one budget. A nonpositive value defaults to 10 seconds. A smaller positive
	// DialTimeout argument takes precedence. Target TLS and RPCs are separate.
	ConnectTimeout time.Duration

	// NoProxy is a list of targets that should bypass the proxy and connect
	// directly. Supports CIDR notation (e.g., "10.0.0.0/8", "172.16.0.0/12")
	// and domain suffixes (e.g., ".internal", "example.com").
	NoProxy []string

	// UnderlyingDialer is the dialer used to connect to the proxy server itself.
	// If nil, remote.NewDefaultDialer() is used. It must honor DialTimeout and
	// return a connection whose Close interrupts pending reads and writes.
	// For unary RPC timeouts, connections must support SetReadDeadline or expose
	// SetReadTimeout(time.Duration) error. Custom wrappers must preserve that API.
	UnderlyingDialer remote.Dialer
}

// HTTPConnectDialer implements remote.Dialer by establishing a TCP tunnel
// through an HTTP CONNECT proxy.
//
// The dialer returns a net.Conn that preserves buffered tunnel bytes. Kitex
// clients should configure it through WithHTTPProxy or WithHTTPProxyConfig,
// which also select the appropriate transport and connection adapter.
type HTTPConnectDialer struct {
	cfg    HTTPConnectConfig
	direct remote.Dialer
}

// NewHTTPConnectDialer creates a new HTTPConnectDialer with the given config.
func NewHTTPConnectDialer(cfg HTTPConnectConfig) *HTTPConnectDialer {
	d := &HTTPConnectDialer{
		cfg:    cfg,
		direct: cfg.UnderlyingDialer,
	}
	if d.direct == nil {
		d.direct = remote.NewDefaultDialer()
	}
	if d.cfg.ConnectTimeout <= 0 {
		d.cfg.ConnectTimeout = 10 * time.Second
	}
	return d
}

// DialTimeout implements remote.Dialer interface.
// It establishes a TCP tunnel to the target address through the HTTP CONNECT proxy.
//
// The flow is:
//  1. Check if the target should bypass the proxy (NoProxy rules).
//  2. Connect to the proxy server (with optional TLS).
//  3. Send an HTTP CONNECT request with the target host:port.
//  4. Read the proxy response and verify it returns 200.
//  5. Return the tunnel connection for RPC traffic and optional target TLS.
//
// NoProxy uses the underlying dialer directly with the caller's timeout. For a
// proxied connection, a zero caller timeout uses the configured handshake budget.
func (d *HTTPConnectDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	// Check if the target should bypass the proxy
	if d.shouldBypass(address) {
		return d.direct.DialTimeout(network, address, timeout)
	}

	// One budget covers dialing, proxy TLS, and the CONNECT exchange.
	effectiveTimeout := d.cfg.ConnectTimeout
	if timeout > 0 && timeout < effectiveTimeout {
		effectiveTimeout = timeout
	}
	deadline := time.Now().Add(effectiveTimeout)
	proxyConn, err := d.direct.DialTimeout("tcp", d.cfg.Address, effectiveTimeout)
	if err != nil {
		return nil, fmt.Errorf("connect proxy %s failed: %w", d.cfg.Address, err)
	}
	// Closing the connection also interrupts custom dialers' connections that
	// do not implement deadlines (for example netpoll.Connection).
	expired := make(chan struct{})
	timer := time.AfterFunc(time.Until(deadline), func() {
		proxyConn.Close()
		close(expired)
	})
	success := false
	defer func() {
		timer.Stop()
		if !success {
			proxyConn.Close()
		}
	}()
	// Expose only net.Conn: netpoll also has a buffered WriteString method,
	// which net/http would use without flushing the CONNECT request.
	var handshakeConn net.Conn = struct{ net.Conn }{proxyConn}
	if d.cfg.ProxyTLS != nil {
		cfg := d.cfg.ProxyTLS.Clone()
		if cfg.ServerName == "" {
			host, _, err := net.SplitHostPort(d.cfg.Address)
			if err != nil {
				return nil, fmt.Errorf("invalid proxy address: %w", err)
			}
			cfg.ServerName = host
		}
		tlsConn := tls.Client(proxyConn, cfg)
		if err := tlsConn.Handshake(); err != nil {
			return nil, handshakeError("proxy TLS handshake", err, deadline)
		}
		handshakeConn = tlsConn
	}

	// Step 2: Send CONNECT request
	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Host: address},
		Host:   address,
		Header: make(http.Header),
	}
	if d.cfg.Username != "" {
		cred := base64.StdEncoding.EncodeToString(
			[]byte(d.cfg.Username + ":" + d.cfg.Password),
		)
		req.Header.Set("Proxy-Authorization", "Basic "+cred)
	}
	if err := req.Write(handshakeConn); err != nil {
		return nil, handshakeError("write CONNECT request", err, deadline)
	}

	// Step 3: Read proxy response
	reader := bufio.NewReader(handshakeConn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		return nil, handshakeError("read proxy response", err, deadline)
	}
	// A successful CONNECT has no HTTP body; subsequent bytes belong to the tunnel.
	// On rejection close the connection without draining an untrusted body.

	// Step 4: Verify tunnel established
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy CONNECT rejected: %s", resp.Status)
	}

	// Stop and synchronize with the callback before handing ownership to the
	// caller. A timer that already fired must never close a returned tunnel.
	if !timer.Stop() {
		<-expired
		return nil, fmt.Errorf("proxy CONNECT handshake: %w", os.ErrDeadlineExceeded)
	}
	if !time.Now().Before(deadline) {
		return nil, fmt.Errorf("proxy CONNECT handshake: %w", os.ErrDeadlineExceeded)
	}
	success = true
	conn := &httpConnectConn{Conn: handshakeConn, reader: reader}
	// Capture this capability before TLS and net.Conn-only wrappers hide it.
	if c, ok := proxyConn.(interface{ SetReadTimeout(time.Duration) error }); ok {
		conn.setReadTimeout = c.SetReadTimeout
	}
	return conn, nil
}

func handshakeError(stage string, err error, deadline time.Time) error {
	if !time.Now().Before(deadline) {
		err = os.ErrDeadlineExceeded
	}
	return fmt.Errorf("%s failed: %w", stage, err)
}

// httpConnectConn preserves bytes read ahead while parsing the CONNECT response.
type httpConnectConn struct {
	net.Conn
	reader         *bufio.Reader
	setReadTimeout func(time.Duration) error
}

func (c *httpConnectConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// SetReadTimeout configures RPC reads using the underlying connection's native
// timeout API. SetReadDeadline continues to have the underlying net.Conn's
// semantics; duration timeouts are not an emulation of absolute deadlines.
func (c *httpConnectConn) SetReadTimeout(timeout time.Duration) error {
	if c.setReadTimeout != nil {
		return c.setReadTimeout(timeout)
	}
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	return c.Conn.SetReadDeadline(deadline)
}

// shouldBypass checks if the target address should bypass the proxy.
// It supports both CIDR notation and domain suffix matching.
func (d *HTTPConnectDialer) shouldBypass(address string) bool {
	if len(d.cfg.NoProxy) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	for _, rule := range d.cfg.NoProxy {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		// CIDR match
		if _, cidr, err := net.ParseCIDR(rule); err == nil {
			if ip := net.ParseIP(host); ip != nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		// Domain suffix match:
		// ".example.com" matches "foo.example.com" but NOT "example.com"
		// "example.com" matches "example.com" AND "foo.example.com"
		if strings.HasPrefix(rule, ".") {
			if strings.HasSuffix(host, rule) {
				return true
			}
		} else {
			if host == rule || strings.HasSuffix(host, "."+rule) {
				return true
			}
		}
	}
	return false
}
