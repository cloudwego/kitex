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
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/remote"
)

// --- Test helpers ---

// testProxyServer is a minimal HTTP CONNECT proxy server for testing.
type testProxyServer struct {
	listener    net.Listener
	addr        string
	mu          sync.Mutex
	gotAuth     string
	gotTarget   string
	respStatus  int
	authUser    string
	authPass    string
	requireAuth bool
	closed      bool
	conns       map[net.Conn]struct{}
	wg          sync.WaitGroup
}

func newTestProxyServer(t *testing.T) *testProxyServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	s := &testProxyServer{
		listener:   ln,
		addr:       ln.Addr().String(),
		respStatus: http.StatusOK,
		conns:      make(map[net.Conn]struct{}),
	}
	s.wg.Add(1)
	go s.serve()
	return s
}

func (s *testProxyServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			conn.Close()
			return
		}
		s.conns[conn] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go s.handle(conn)
	}
}

func (s *testProxyServer) handle(conn net.Conn) {
	defer s.wg.Done()
	defer func() { conn.Close(); s.mu.Lock(); delete(s.conns, conn); s.mu.Unlock() }()
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}

	s.mu.Lock()
	s.gotTarget = req.Host
	s.gotAuth = req.Header.Get("Proxy-Authorization")
	requireAuth, authUser, authPass, respStatus := s.requireAuth, s.authUser, s.authPass, s.respStatus
	s.mu.Unlock()

	// Check auth if required
	if requireAuth {
		expected := "Basic " + base64.StdEncoding.EncodeToString(
			[]byte(authUser+":"+authPass),
		)
		if req.Header.Get("Proxy-Authorization") != expected {
			resp := &http.Response{
				StatusCode: http.StatusProxyAuthRequired,
				Status:     "407 Proxy Authentication Required",
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			resp.Write(conn)
			return
		}
	}

	// Send response
	resp := &http.Response{
		StatusCode: respStatus,
		Status:     fmt.Sprintf("%d %s", respStatus, http.StatusText(respStatus)),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}
	if err := resp.Write(conn); err != nil {
		return
	}

	// If 200, act as a tunnel: echo back data (for testing)
	if respStatus == http.StatusOK {
		// Echo server: read and write back
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				conn.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}
}

func (s *testProxyServer) close() {
	s.listener.Close()
	s.mu.Lock()
	s.closed = true
	for c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *testProxyServer) getGotAuth() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gotAuth
}

func (s *testProxyServer) getGotTarget() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gotTarget
}

// --- Tests ---

func TestNewHTTPConnectDialer_Defaults(t *testing.T) {
	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address: "proxy.example.com:3128",
	})
	if d.cfg.ConnectTimeout != 10*time.Second {
		t.Errorf("expected default ConnectTimeout 10s, got %v", d.cfg.ConnectTimeout)
	}
	if d.direct == nil {
		t.Error("expected default UnderlyingDialer to be set")
	}
}

func TestNewHTTPConnectDialer_CustomUnderlyingDialer(t *testing.T) {
	custom := remote.NewDefaultDialer()
	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address:          "proxy.example.com:3128",
		UnderlyingDialer: custom,
	})
	if d.direct != custom {
		t.Error("expected custom UnderlyingDialer to be used")
	}
}

func TestShouldBypass_CIDR(t *testing.T) {
	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address: "proxy.example.com:3128",
		NoProxy: []string{"10.0.0.0/8", "192.168.0.0/16"},
	})

	tests := []struct {
		address string
		bypass  bool
	}{
		{"10.1.2.3:8080", true},
		{"192.168.1.1:443", true},
		{"172.16.0.1:80", false},
		{"8.8.8.8:53", false},
	}
	for _, tt := range tests {
		if got := d.shouldBypass(tt.address); got != tt.bypass {
			t.Errorf("shouldBypass(%s) = %v, want %v", tt.address, got, tt.bypass)
		}
	}
}

func TestShouldBypass_DomainSuffix(t *testing.T) {
	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address: "proxy.example.com:3128",
		NoProxy: []string{".internal", "example.com", "localhost"},
	})

	tests := []struct {
		address string
		bypass  bool
	}{
		{"foo.internal:8080", true},
		{"bar.baz.internal:443", true},
		{"internal:80", false}, // ".internal" should not match "internal" without dot
		{"example.com:80", true},
		{"sub.example.com:443", true},
		{"notexample.com:80", false},
		{"localhost:8080", true},
		{"google.com:443", false},
	}
	for _, tt := range tests {
		if got := d.shouldBypass(tt.address); got != tt.bypass {
			t.Errorf("shouldBypass(%s) = %v, want %v", tt.address, got, tt.bypass)
		}
	}
}

func TestShouldBypass_EmptyNoProxy(t *testing.T) {
	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address: "proxy.example.com:3128",
	})
	if d.shouldBypass("anything:80") {
		t.Error("expected no bypass when NoProxy is empty")
	}
}

func TestShouldBypass_InvalidAddress(t *testing.T) {
	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address: "proxy.example.com:3128",
		NoProxy: []string{"10.0.0.0/8"},
	})
	// Address without port should not panic and should return false
	if d.shouldBypass("no-port-here") {
		t.Error("expected no bypass for invalid address")
	}
}

func TestDialTimeout_Success(t *testing.T) {
	proxy := newTestProxyServer(t)
	defer proxy.close()

	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address:        proxy.addr,
		ConnectTimeout: 5 * time.Second,
	})

	conn, err := d.DialTimeout("tcp", "target.example.com:443", 10*time.Second)
	if err != nil {
		t.Fatalf("DialTimeout failed: %v", err)
	}
	defer conn.Close()

	// Verify the proxy received the correct target
	if got := proxy.getGotTarget(); got != "target.example.com:443" {
		t.Errorf("proxy got target %q, want %q", got, "target.example.com:443")
	}

	// Verify tunnel works: write data and read echo
	testData := []byte("hello tunnel")
	if _, err := conn.Write(testData); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	buf := make([]byte, len(testData))
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(buf) != string(testData) {
		t.Errorf("tunnel echo = %q, want %q", buf, testData)
	}
}

func TestDialTimeout_BasicAuth(t *testing.T) {
	proxy := newTestProxyServer(t)
	defer proxy.close()
	proxy.mu.Lock()
	proxy.requireAuth = true
	proxy.authUser = "testuser"
	proxy.authPass = "testpass"
	proxy.mu.Unlock()

	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address:  proxy.addr,
		Username: "testuser",
		Password: "testpass",
	})

	conn, err := d.DialTimeout("tcp", "target.example.com:443", 10*time.Second)
	if err != nil {
		t.Fatalf("DialTimeout failed: %v", err)
	}
	defer conn.Close()

	// Verify auth header
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString(
		[]byte("testuser:testpass"),
	)
	if got := proxy.getGotAuth(); got != expectedAuth {
		t.Errorf("proxy got auth %q, want %q", got, expectedAuth)
	}
}

func TestDialTimeout_AuthFailed(t *testing.T) {
	proxy := newTestProxyServer(t)
	defer proxy.close()
	proxy.mu.Lock()
	proxy.requireAuth = true
	proxy.authUser = "correctuser"
	proxy.authPass = "correctpass"
	proxy.mu.Unlock()

	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address:  proxy.addr,
		Username: "wronguser",
		Password: "wrongpass",
	})

	_, err := d.DialTimeout("tcp", "target.example.com:443", 10*time.Second)
	if err == nil {
		t.Fatal("expected error for failed auth, got nil")
	}
	if !strings.Contains(err.Error(), "407") {
		t.Errorf("expected error to contain 407, got: %v", err)
	}
}

func TestDialTimeout_ProxyRejected(t *testing.T) {
	proxy := newTestProxyServer(t)
	defer proxy.close()
	proxy.mu.Lock()
	proxy.respStatus = http.StatusForbidden
	proxy.mu.Unlock()

	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address: proxy.addr,
	})

	_, err := d.DialTimeout("tcp", "target.example.com:443", 10*time.Second)
	if err == nil {
		t.Fatal("expected error for rejected CONNECT, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error to contain 403, got: %v", err)
	}
}

func TestDialTimeout_NoProxyBypass(t *testing.T) {
	// Start a direct echo server (no proxy)
	directLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer directLn.Close()
	go func() {
		for {
			conn, err := directLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	// Start a proxy that should NOT be used
	proxy := newTestProxyServer(t)
	defer proxy.close()

	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address: proxy.addr,
		NoProxy: []string{"127.0.0.0/8"},
	})

	// Dial the direct server, should bypass proxy
	conn, err := d.DialTimeout("tcp", directLn.Addr().String(), 10*time.Second)
	if err != nil {
		t.Fatalf("DialTimeout failed: %v", err)
	}
	defer conn.Close()

	// Verify direct connection works
	testData := []byte("direct connection")
	if _, err := conn.Write(testData); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	buf := make([]byte, len(testData))
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(buf) != string(testData) {
		t.Errorf("direct echo = %q, want %q", buf, testData)
	}
}

func TestDialTimeout_ProxyUnreachable(t *testing.T) {
	// Use a port that's definitely not listening
	d := NewHTTPConnectDialer(HTTPConnectConfig{
		Address:        "127.0.0.1:1", // port 1 is unlikely to be open
		ConnectTimeout: 2 * time.Second,
	})

	_, err := d.DialTimeout("tcp", "target.example.com:443", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for unreachable proxy, got nil")
	}
}

func TestProxyType_Constants(t *testing.T) {
	if ProxyDirect != 0 {
		t.Error("ProxyDirect should be 0")
	}
	if ProxyHTTPConnect != 1 {
		t.Error("ProxyHTTPConnect should be 1")
	}
	if ProxySOCKS5 != 2 {
		t.Error("ProxySOCKS5 should be 2")
	}
}

// Ensure HTTPConnectDialer implements remote.Dialer interface
var _ remote.Dialer = (*HTTPConnectDialer)(nil)
