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
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/transport"
)

func TestParseHTTPProxyAddr_PlainHTTP(t *testing.T) {
	cfg, err := parseHTTPProxyAddr("http://proxy.example.com:3128")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cfg.Address != "proxy.example.com:3128" {
		t.Errorf("Address = %q, want %q", cfg.Address, "proxy.example.com:3128")
	}
	if cfg.Type != proxy.ProxyHTTPConnect {
		t.Errorf("Type = %v, want ProxyHTTPConnect", cfg.Type)
	}
	if cfg.ProxyTLS != nil {
		t.Error("ProxyTLS should be nil for http scheme")
	}
	if cfg.Username != "" {
		t.Errorf("Username = %q, want empty", cfg.Username)
	}
}

func TestParseHTTPProxyAddr_HTTPS(t *testing.T) {
	cfg, err := parseHTTPProxyAddr("https://proxy.example.com:443")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cfg.Address != "proxy.example.com:443" {
		t.Errorf("Address = %q, want %q", cfg.Address, "proxy.example.com:443")
	}
	if cfg.ProxyTLS == nil {
		t.Error("ProxyTLS should not be nil for https scheme")
	}
}

func TestParseHTTPProxyAddr_WithAuth(t *testing.T) {
	cfg, err := parseHTTPProxyAddr("http://user:pass@proxy.example.com:3128")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cfg.Username != "user" {
		t.Errorf("Username = %q, want %q", cfg.Username, "user")
	}
	if cfg.Password != "pass" {
		t.Errorf("Password = %q, want %q", cfg.Password, "pass")
	}
}

func TestParseHTTPProxyAddr_NoScheme(t *testing.T) {
	cfg, err := parseHTTPProxyAddr("proxy.example.com:3128")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cfg.Address != "proxy.example.com:3128" {
		t.Errorf("Address = %q, want %q", cfg.Address, "proxy.example.com:3128")
	}
	if cfg.ProxyTLS != nil {
		t.Error("ProxyTLS should be nil when scheme omitted")
	}
}

func TestParseHTTPProxyAddr_Invalid(t *testing.T) {
	_, err := parseHTTPProxyAddr("://invalid")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestParseHTTPProxyAddr_MissingHost(t *testing.T) {
	_, err := parseHTTPProxyAddr("http://")
	if err == nil {
		t.Error("expected error for missing host, got nil")
	}
}

func TestContainsScheme(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"http://example.com", true},
		{"https://example.com", true},
		{"socks5://example.com", true},
		{"example.com:3128", false},
		{"127.0.0.1:3128", false},
		{"", false},
		{":3128", false}, // colon at position 0 is not a valid scheme
	}
	for _, tt := range tests {
		if got := containsScheme(tt.addr); got != tt.want {
			t.Errorf("containsScheme(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestMaskProxyAddrForLog(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{"http://user:pass@proxy.example.com:3128", "http://user:******@proxy.example.com:3128"},
		{"http://proxy.example.com:3128", "http://proxy.example.com:3128"},
		{"proxy.example.com:3128", "http://proxy.example.com:3128"},
		{"https://user:secret@proxy.example.com:443", "https://user:******@proxy.example.com:443"},
	}
	for _, tt := range tests {
		if got := maskProxyAddrForLog(tt.addr); got != tt.want {
			t.Errorf("maskProxyAddrForLog(%q) = %q, want %q", tt.addr, got, tt.want)
		}
	}
}

func TestWithHTTPProxy_SetsDialer(t *testing.T) {
	opts := newTestOptions()
	di := &utils.Slice{}

	opt := WithHTTPProxy("http://proxy.example.com:3128")
	opt.F(opts, di)

	if opts.RemoteOpt.Dialer == nil {
		t.Fatal("Dialer should be set")
	}
	if _, ok := opts.RemoteOpt.Dialer.(*proxy.HTTPConnectDialer); !ok {
		t.Errorf("Dialer type = %T, want *proxy.HTTPConnectDialer", opts.RemoteOpt.Dialer)
	}
}

func TestWithHTTPProxy_InvalidAddrPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid proxy address")
		}
	}()
	opts := newTestOptions()
	di := &utils.Slice{}
	opt := WithHTTPProxy("://invalid")
	opt.F(opts, di)
}

func TestWithHTTPProxyConfig_SetsDialer(t *testing.T) {
	opts := newTestOptions()
	di := &utils.Slice{}

	cfg := proxy.HTTPConnectConfig{
		Address:  "proxy.example.com:3128",
		Username: "user",
		Password: "pass",
	}
	opt := WithHTTPProxyConfig(cfg)
	opt.F(opts, di)

	if opts.RemoteOpt.Dialer == nil {
		t.Fatal("Dialer should be set")
	}
	d, ok := opts.RemoteOpt.Dialer.(*proxy.HTTPConnectDialer)
	if !ok {
		t.Fatalf("Dialer type = %T, want *proxy.HTTPConnectDialer", opts.RemoteOpt.Dialer)
	}
	_ = d
}

func TestWithHTTPProxyConfig_EmptyAddressPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty address")
		}
	}()
	opts := newTestOptions()
	di := &utils.Slice{}
	opt := WithHTTPProxyConfig(proxy.HTTPConnectConfig{})
	opt.F(opts, di)
}

func TestWithHTTPProxyConfig_ProxyDirectTypeDefaultsToHTTPConnect(t *testing.T) {
	opts := newTestOptions()
	di := &utils.Slice{}

	cfg := proxy.HTTPConnectConfig{
		Type:    proxy.ProxyDirect,
		Address: "proxy.example.com:3128",
	}
	opt := WithHTTPProxyConfig(cfg)
	opt.F(opts, di)

	if opts.RemoteOpt.Dialer == nil {
		t.Fatal("Dialer should be set")
	}
}

func TestCheckProxyConflict_WithMeshProxyPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic when Mesh Proxy is already set")
			return
		}
		msg, ok := r.(string)
		if !ok {
			t.Errorf("panic type = %T, want string", r)
			return
		}
		if !strings.Contains(msg, "conflicts with WithProxy") {
			t.Errorf("panic message = %q, should contain 'conflicts with WithProxy'", msg)
		}
	}()

	opts := newTestOptions()
	// Set a mock Mesh ForwardProxy
	opts.Proxy = &mockForwardProxy{}
	checkProxyConflict(opts)
}

func TestCheckProxyConflict_NoConflict(t *testing.T) {
	opts := newTestOptions()
	// Should not panic
	checkProxyConflict(opts)
}

// --- Test helpers ---

func newTestOptions() *client.Options {
	return &client.Options{
		RemoteOpt: &remote.ClientOption{
			Dialer: remote.NewDefaultDialer(),
		},
	}
}

// mockForwardProxy implements proxy.ForwardProxy for testing
type mockForwardProxy struct{}

func (m *mockForwardProxy) Configure(cfg *proxy.Config) error { return nil }
func (m *mockForwardProxy) ResolveProxyInstance(ctx context.Context) error {
	return nil
}

func TestHTTPProxyOptionConflicts(t *testing.T) {
	options := []struct {
		name   string
		option Option
	}{
		{"url", WithHTTPProxy("proxy:3128")},
		{"config", WithHTTPProxyConfig(proxy.HTTPConnectConfig{Address: "proxy:3128"})},
		{"dialer", WithDialer(remote.NewDefaultDialer())},
		{"mesh", WithProxy(&mockForwardProxy{})},
	}
	for i, a := range options {
		for j, b := range options {
			if i > 1 && j > 1 {
				continue
			}
			t.Run(a.name+"_then_"+b.name, func(t *testing.T) {
				defer func() {
					if recover() == nil {
						t.Fatal("accepted conflicting options")
					}
				}()
				client.NewOptions([]Option{a.option, b.option})
			})
		}
	}
}

func TestHTTPProxyRejectsIncompatibleTransports(t *testing.T) {
	for _, other := range []Option{WithHTTPConnection(), WithMuxConnection(1), WithTransHandlerFactory(nil), WithTransportProtocol(transport.TTHeaderStreaming), WithTransportProtocol(transport.HTTP)} {
		for _, opts := range [][]Option{{WithHTTPProxy("proxy:3128"), other}, {other, WithHTTPProxy("proxy:3128")}} {
			func() {
				defer func() {
					if recover() == nil {
						t.Error("accepted incompatible proxy transport")
					}
				}()
				client.NewOptions(opts)
			}()
		}
	}
}

func TestHTTPProxyOptionReuse(t *testing.T) {
	for _, opt := range []Option{WithHTTPProxy("proxy:3128"), WithHTTPProxyConfig(proxy.HTTPConnectConfig{Address: "proxy:3128"})} {
		client.NewOptions([]Option{opt})
		client.NewOptions([]Option{opt})
	}
	// Existing mesh and custom dialer combinations remain allowed.
	client.NewOptions([]Option{WithProxy(&mockForwardProxy{}), WithDialer(remote.NewDefaultDialer())})
	client.NewOptions([]Option{WithDialer(remote.NewDefaultDialer()), WithProxy(&mockForwardProxy{})})
}
