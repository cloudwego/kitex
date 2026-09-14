/*
 * Copyright 2026 CloudWeGo Authors
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
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/remote"
	netpolltrans "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
)

func pipeProxy(t *testing.T, cfg HTTPConnectConfig, serve func(net.Conn)) *HTTPConnectDialer {
	t.Helper()
	c, s := net.Pipe()
	done := make(chan struct{})
	t.Cleanup(func() { c.Close(); s.Close(); <-done })
	cfg.Address = "proxy.example.com:443"
	cfg.UnderlyingDialer = remote.SynthesizedDialer{DialFunc: func(string, string, time.Duration) (net.Conn, error) { return c, nil }}
	go func() { defer close(done); defer s.Close(); serve(s) }()
	return NewHTTPConnectDialer(cfg)
}

func TestHTTPConnectHandshakeTimeout(t *testing.T) {
	for _, stage := range []string{"tls", "write", "response", "rejection_body"} {
		for _, budget := range []struct {
			name           string
			caller, config time.Duration
		}{
			{"zero_caller", 0, 30 * time.Millisecond},
			{"caller_minimum", 30 * time.Millisecond, time.Second},
			{"config_minimum", time.Second, 30 * time.Millisecond},
		} {
			t.Run(stage+"/"+budget.name, func(t *testing.T) {
				cfg := HTTPConnectConfig{ConnectTimeout: budget.config}
				if stage == "tls" {
					cfg.ProxyTLS = &tls.Config{ServerName: "proxy.example.com"}
				}
				release := make(chan struct{})
				d := pipeProxy(t, cfg, func(c net.Conn) {
					if stage == "write" || stage == "tls" {
						<-release
						return
					}
					if _, err := http.ReadRequest(bufio.NewReader(c)); err != nil {
						return
					}
					if stage == "rejection_body" {
						io.WriteString(c, "HTTP/1.1 403 Forbidden\r\nContent-Length: 100\r\n\r\n")
					}
					<-release
				})
				defer close(release)
				done := make(chan error, 1)
				go func() {
					c, err := d.DialTimeout("tcp", "target:80", budget.caller)
					if c != nil {
						c.Close()
					}
					done <- err
				}()
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("expected handshake error")
					}
					if stage != "rejection_body" {
						var timeout net.Error
						if !errors.As(err, &timeout) || !timeout.Timeout() {
							t.Fatalf("expected timeout classification: %v", err)
						}
					}
					if stage == "rejection_body" && !strings.Contains(err.Error(), "403") {
						t.Fatalf("lost rejection status: %v", err)
					}
				case <-time.After(500 * time.Millisecond):
					t.Fatal("CONNECT exceeded its 30ms budget")
				}
			})
		}
	}
}

func TestHTTPConnectPreservesTunnelBytes(t *testing.T) {
	const settings = "\x00\x00\x00\x04\x00\x00\x00\x00\x00"
	d := pipeProxy(t, HTTPConnectConfig{}, func(c net.Conn) {
		if _, err := http.ReadRequest(bufio.NewReader(c)); err != nil {
			return
		}
		io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n"+settings)
		io.Copy(c, c)
	})
	c, err := d.DialTimeout("tcp", "target:80", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(200 * time.Millisecond))
	b := make([]byte, len(settings))
	if _, err = io.ReadFull(c, b); err != nil {
		t.Fatalf("lost coalesced SETTINGS: %v", err)
	}
	if string(b) != settings {
		t.Fatalf("SETTINGS = %q", b)
	}
	if _, err = io.WriteString(c, "hello"); err != nil {
		t.Fatal(err)
	}
	b = make([]byte, 5)
	if _, err = io.ReadFull(c, b); err != nil || string(b) != "hello" {
		t.Fatalf("tunnel echo %q: %v", b, err)
	}
}

func TestHTTPConnectProxyTLSVerification(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer c.Close()
		io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
		io.Copy(c, c)
	}))
	defer s.Close()
	roots := x509.NewCertPool()
	roots.AddCert(s.Certificate())
	for _, tc := range []struct {
		name, address, serverName string
		trusted, success          bool
	}{
		{"infer_ip", "127.0.0.1:443", "", true, true},
		{"infer_dns", "example.com:443", "", true, true},
		{"infer_ipv6", "[::1]:443", "", true, true},
		{"override", "127.0.0.1:443", "example.com", true, true},
		{"wrong_override", "127.0.0.1:443", "wrong.example", true, false},
		{"untrusted", "127.0.0.1:443", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &tls.Config{ServerName: tc.serverName}
			if tc.trusted {
				cfg.RootCAs = roots
			}
			d := NewHTTPConnectDialer(HTTPConnectConfig{Address: tc.address, ProxyTLS: cfg, UnderlyingDialer: remote.SynthesizedDialer{DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
				if address != tc.address {
					t.Errorf("dialed unexpected proxy %q", address)
				}
				return net.DialTimeout(network, s.Listener.Addr().String(), timeout)
			}}})
			c, err := d.DialTimeout("tcp", "target:80", time.Second)
			if (err == nil) != tc.success {
				t.Fatalf("success=%v, err=%v", tc.success, err)
			}
			if c != nil {
				c.Close()
			}
			if cfg.ServerName != tc.serverName || cfg.InsecureSkipVerify {
				t.Fatal("mutated caller TLS config")
			}
		})
	}
}

func TestHTTPConnectTunnelOutlivesHandshakeTimeout(t *testing.T) {
	d := pipeProxy(t, HTTPConnectConfig{ConnectTimeout: 50 * time.Millisecond}, func(c net.Conn) {
		if _, err := http.ReadRequest(bufio.NewReader(c)); err != nil {
			return
		}
		io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
		io.Copy(c, c)
	})
	c, err := d.DialTimeout("tcp", "target:80", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// Deliberately cross the former deadline before using the established tunnel.
	<-time.After(100 * time.Millisecond)
	c.SetDeadline(time.Now().Add(time.Second))
	if _, err = io.WriteString(c, "ok"); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 2)
	if _, err = io.ReadFull(c, b); err != nil || string(b) != "ok" {
		t.Fatalf("expired tunnel: %q %v", b, err)
	}
}

func TestHTTPConnectTimeoutIncludesDial(t *testing.T) {
	c, s := net.Pipe()
	defer c.Close()
	defer s.Close()
	started := time.Now()
	d := NewHTTPConnectDialer(HTTPConnectConfig{Address: "proxy:3128", ConnectTimeout: 300 * time.Millisecond, UnderlyingDialer: remote.SynthesizedDialer{DialFunc: func(string, string, time.Duration) (net.Conn, error) {
		<-time.After(200 * time.Millisecond)
		return c, nil
	}}})
	_, err := d.DialTimeout("tcp", "target:80", time.Second)
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("expected timeout, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 450*time.Millisecond {
		t.Fatalf("dial time was excluded: %v", elapsed)
	}
	b := make([]byte, 1)
	if _, err = s.Read(b); err != io.EOF {
		t.Fatalf("failed tunnel still open: %v", err)
	}
}

func TestHTTPConnectDefaultBudget(t *testing.T) {
	sentinel := errors.New("dial stopped")
	d := NewHTTPConnectDialer(HTTPConnectConfig{Address: "proxy:3128", UnderlyingDialer: remote.SynthesizedDialer{DialFunc: func(_, _ string, timeout time.Duration) (net.Conn, error) {
		if timeout != 10*time.Second {
			t.Errorf("default dial budget = %v", timeout)
		}
		return nil, sentinel
	}}})
	if _, err := d.DialTimeout("tcp", "target:80", 0); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}

func TestHTTPConnectNetpollUnderlyingDialer(t *testing.T) {
	for _, stall := range []bool{false, true} {
		t.Run(fmt.Sprintf("stall_%v", stall), func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				c, err := ln.Accept()
				if err != nil {
					return
				}
				defer c.Close()
				if _, err = http.ReadRequest(bufio.NewReader(c)); err != nil {
					return
				}
				if !stall {
					io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
				}
				io.Copy(c, c)
			}()
			d := NewHTTPConnectDialer(HTTPConnectConfig{Address: ln.Addr().String(), ConnectTimeout: 100 * time.Millisecond, UnderlyingDialer: netpolltrans.NewDialer()})
			c, err := d.DialTimeout("tcp", "target:80", time.Second)
			if stall {
				var timeout net.Error
				if !errors.As(err, &timeout) || !timeout.Timeout() {
					t.Fatalf("expected timeout, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if _, err = io.WriteString(c, "ok"); err != nil {
					t.Fatal(err)
				}
				b := make([]byte, 2)
				if _, err = io.ReadFull(c, b); err != nil || string(b) != "ok" {
					t.Fatalf("tunnel echo %q: %v", b, err)
				}
				c.Close()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("proxy connection was not closed")
			}
		})
	}
}
