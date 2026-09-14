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

package client_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	mockthrift "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/detection"
	"github.com/cloudwego/kitex/pkg/remote/trans/gonet"
	netpolltrans "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/transport"
)

func runHTTPProxyTestServer(t *testing.T, svr server.Server) {
	t.Helper()

	started := make(chan struct{})
	var once sync.Once
	server.RegisterStartHook(func() {
		once.Do(func() {
			close(started)
		})
	})

	run := make(chan error, 1)
	go func() { run <- svr.Run() }()
	select {
	case <-started:
	case err := <-run:
		if err != nil {
			t.Fatalf("server startup: %v", err)
		}
		t.Fatal("server stopped before startup completed")
	case <-time.After(3 * time.Second):
		t.Fatal("server startup timeout")
	}
	t.Cleanup(func() {
		svr.Stop()
		if err := <-run; err != nil {
			t.Errorf("server exit: %v", err)
		}
	})
}

// terminateProxyTargetTLS supplies server-side TLS for Kitex's TCP listener.
func terminateProxyTargetTLS(t *testing.T, target string, certs []tls.Certificate) string {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: certs, NextProtos: []string{"h2"}})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var conns []net.Conn
	var closed bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			if closed {
				mu.Unlock()
				c.Close()
				return
			}
			conns = append(conns, c)
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer c.Close()
				upstream, err := net.Dial("tcp", target)
				if err != nil {
					return
				}
				defer upstream.Close()
				done := make(chan struct{})
				go func() { io.Copy(upstream, c); upstream.Close(); close(done) }()
				io.Copy(c, upstream)
				c.Close()
				<-done
			}()
		}
	}()
	t.Cleanup(func() {
		mu.Lock()
		closed = true
		ln.Close()
		for _, c := range conns {
			c.Close()
		}
		mu.Unlock()
		wg.Wait()
	})
	return ln.Addr().String()
}

// Exercise the public options with real serialized requests and responses.
func TestHTTPProxyRPC(t *testing.T) {
	certServer := httptest.NewTLSServer(http.NotFoundHandler())
	certs := certServer.TLS.Certificates
	roots := x509.NewCertPool()
	roots.AddCert(certServer.Certificate())
	certServer.Close()
	for _, tc := range []struct {
		name                                       string
		grpc, proxyTLS, targetTLS, bypass, netpoll bool
	}{
		{name: "default_thrift"},
		{name: "thrift_https_proxy", proxyTLS: true},
		{name: "thrift_no_proxy", bypass: true},
		{name: "grpc_h2c", grpc: true},
		{name: "grpc_tls", grpc: true, targetTLS: true},
		{name: "grpc_tls_https_proxy", grpc: true, proxyTLS: true, targetTLS: true},
		{name: "thrift_netpoll", netpoll: true},
		{name: "thrift_netpoll_https_proxy", netpoll: true, proxyTLS: true},
		{name: "thrift_netpoll_no_proxy", netpoll: true, bypass: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			target := ln.Addr().String()
			mode := serviceinfo.StreamingNone
			if tc.grpc {
				mode = serviceinfo.StreamingUnary
			}
			svc := &serviceinfo.ServiceInfo{ServiceName: "ProxyEcho", PayloadCodec: serviceinfo.Thrift, Methods: map[string]serviceinfo.MethodInfo{
				"echo": serviceinfo.NewMethodInfo(func(ctx context.Context, handler, args, result interface{}) error {
					value := "echo: " + args.(*mockthrift.MockTestArgs).Req.Msg
					result.(*mockthrift.MockTestResult).Success = &value
					return nil
				}, func() interface{} { return new(mockthrift.MockTestArgs) }, func() interface{} { return new(mockthrift.MockTestResult) }, false, serviceinfo.WithStreamingMode(mode)),
			}}
			var factory remote.ServerTransHandlerFactory = gonet.NewSvrTransHandlerFactory()
			var serverFactory remote.TransServerFactory = gonet.NewTransServerFactory()
			if tc.grpc {
				factory = detection.NewSvrTransHandlerFactory(netpolltrans.NewSvrTransHandlerFactory(), nphttp2.NewSvrTransHandlerFactory())
				serverFactory = netpolltrans.NewTransServerFactory()
			}
			svr := server.NewServer(server.WithCompatibleMiddlewareForUnary(), server.WithListener(ln), server.WithTransServerFactory(serverFactory), server.WithTransHandlerFactory(factory), server.WithExitWaitTime(time.Millisecond))
			if err = svr.RegisterService(svc, new(struct{})); err != nil {
				t.Fatal(err)
			}
			runHTTPProxyTestServer(t, svr)
			if tc.targetTLS {
				target = terminateProxyTargetTLS(t, target, certs)
			}
			var mu sync.Mutex
			var conns []net.Conn
			var wg sync.WaitGroup
			connects := 0
			p := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wg.Add(1)
				defer wg.Done()
				if r.Method != "CONNECT" || r.Host != target {
					t.Errorf("unexpected proxy request %s %s", r.Method, r.Host)
					http.Error(w, "bad target", 400)
					return
				}
				upstream, err := net.Dial("tcp", r.Host)
				if err != nil {
					http.Error(w, err.Error(), 502)
					return
				}
				downstream, reader, err := w.(http.Hijacker).Hijack()
				if err != nil {
					upstream.Close()
					return
				}
				mu.Lock()
				conns = append(conns, upstream, downstream)
				connects++
				mu.Unlock()
				defer upstream.Close()
				defer downstream.Close()
				if _, err = io.WriteString(downstream, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
					return
				}

				done := make(chan struct{})
				go func() { io.Copy(upstream, reader); upstream.Close(); close(done) }()
				io.Copy(downstream, upstream)
				downstream.Close()
				<-done
			}))
			if tc.proxyTLS {
				p.TLS = &tls.Config{Certificates: certs}
				p.StartTLS()
			} else {
				p.Start()
			}
			t.Cleanup(func() {
				p.Close()
				mu.Lock()
				for _, c := range conns {
					c.Close()
				}
				mu.Unlock()
				wg.Wait()
			})
			cfg := proxy.HTTPConnectConfig{Address: p.Listener.Addr().String(), ConnectTimeout: time.Second}
			opts := []client.Option{client.WithDestService("ProxyEcho"), client.WithHostPorts(target), client.WithRPCTimeout(2 * time.Second), client.WithConnectTimeout(time.Second)}
			if tc.proxyTLS {
				cfg.ProxyTLS = &tls.Config{RootCAs: roots}
			}
			if tc.bypass {
				cfg.NoProxy = []string{"127.0.0.0/8"}
			}
			if tc.netpoll {
				cfg.UnderlyingDialer = netpolltrans.NewDialer()
			}
			if tc.proxyTLS || tc.bypass || tc.netpoll {
				opts = append(opts, client.WithHTTPProxyConfig(cfg))
			} else {
				opts = append(opts, client.WithHTTPProxy(p.URL))
			}
			if tc.grpc {
				opts = append(opts, client.WithTransportProtocol(transport.GRPC), client.WithGRPCConnPoolSize(1))
			}
			if tc.targetTLS {
				opts = append(opts, client.WithGRPCTLSConfig(&tls.Config{RootCAs: roots, ServerName: "127.0.0.1"}))
			}
			cli, err := client.NewClient(svc, opts...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { cli.(io.Closer).Close() })
			for _, msg := range []string{"first", "reused connection"} {
				result := new(mockthrift.MockTestResult)
				err = cli.Call(context.Background(), "echo", &mockthrift.MockTestArgs{Req: &mockthrift.MockReq{Msg: msg}}, result)
				if err != nil {
					t.Fatal(err)
				}
				if result.GetSuccess() != "echo: "+msg {
					t.Fatalf("wrong RPC result %q", result.GetSuccess())
				}
			}
			mu.Lock()
			count := connects
			mu.Unlock()
			if tc.bypass && count != 0 || !tc.bypass && count != 1 {
				t.Fatalf("CONNECT count = %d", count)
			}
		})
	}
}

// readTrackedConn records the transport read finishing, independently of Call's
// outer timeout. The netpoll variant below retains its additional capability.
type proxyReadTrackedConn struct {
	net.Conn
	readDone  chan error
	closed    chan struct{}
	closeOnce sync.Once
}

func (c *proxyReadTrackedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if err != nil {
		select {
		case c.readDone <- err:
		default:
		}
	}
	return n, err
}

func (c *proxyReadTrackedConn) Close() error {
	err := c.Conn.Close()
	c.closeOnce.Do(func() { close(c.closed) })
	return err
}

type proxyNetpollReadTrackedConn struct{ *proxyReadTrackedConn }

func (c *proxyNetpollReadTrackedConn) SetReadTimeout(timeout time.Duration) error {
	return c.Conn.(interface{ SetReadTimeout(time.Duration) error }).SetReadTimeout(timeout)
}

func TestHTTPProxyRPCReadTimeoutClosesConnection(t *testing.T) {
	certServer := httptest.NewTLSServer(http.NotFoundHandler())
	certs := certServer.TLS.Certificates
	roots := x509.NewCertPool()
	roots.AddCert(certServer.Certificate())
	certServer.Close()
	for _, tc := range []struct {
		name                      string
		netpoll, proxyTLS, bypass bool
	}{
		{name: "tcp_control"},
		{name: "netpoll_http", netpoll: true},
		{name: "netpoll_https", netpoll: true, proxyTLS: true},
		{name: "netpoll_no_proxy", netpoll: true, bypass: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			if tc.proxyTLS {
				ln = tls.NewListener(ln, &tls.Config{Certificates: certs})
			}
			requestSeen := make(chan struct{})
			peerClosed := make(chan struct{})
			accepted := make(chan net.Conn, 1)
			go func() {
				defer close(peerClosed)
				c, err := ln.Accept()
				if err != nil {
					return
				}
				accepted <- c
				defer c.Close()
				reader := bufio.NewReader(c)
				if !tc.bypass {
					if _, err = http.ReadRequest(reader); err != nil {
						return
					}
					if _, err = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
						return
					}
				}
				if _, err = reader.ReadByte(); err != nil {
					return
				}
				close(requestSeen)
				io.Copy(io.Discard, reader) // Deliberately never answer the RPC.
			}()
			var tracked *proxyReadTrackedConn
			var trackingMu sync.Mutex
			dialer := remote.SynthesizedDialer{DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
				var d remote.Dialer = remote.NewDefaultDialer()
				if tc.netpoll {
					d = netpolltrans.NewDialer()
				}
				c, err := d.DialTimeout(network, address, timeout)
				if err != nil {
					return nil, err
				}
				trackingMu.Lock()
				tracked = &proxyReadTrackedConn{Conn: c, readDone: make(chan error, 1), closed: make(chan struct{})}
				trackingMu.Unlock()
				if tc.netpoll {
					return &proxyNetpollReadTrackedConn{tracked}, nil
				}
				return tracked, nil
			}}
			// Cleanup also bounds the deliberately failing pre-fix test.
			t.Cleanup(func() {
				ln.Close()
				trackingMu.Lock()
				if tracked != nil {
					tracked.Close()
				}
				trackingMu.Unlock()
				select {
				case c := <-accepted:
					c.Close()
				default:
				}
				<-peerClosed
			})
			cfg := proxy.HTTPConnectConfig{Address: ln.Addr().String(), UnderlyingDialer: dialer}
			if tc.proxyTLS {
				cfg.ProxyTLS = &tls.Config{RootCAs: roots}
			}
			if tc.bypass {
				cfg.NoProxy = []string{"127.0.0.0/8"}
			}
			svc := &serviceinfo.ServiceInfo{ServiceName: "ProxyReadTimeout", PayloadCodec: serviceinfo.Thrift, Methods: map[string]serviceinfo.MethodInfo{
				"echo": serviceinfo.NewMethodInfo(nil, func() interface{} { return new(mockthrift.MockTestArgs) }, func() interface{} { return new(mockthrift.MockTestResult) }, false),
			}}
			cli, err := client.NewClient(svc, client.WithDestService("ProxyReadTimeout"), client.WithHostPorts(ln.Addr().String()), client.WithConnectTimeout(time.Second), client.WithRPCTimeout(50*time.Millisecond), client.WithHTTPProxyConfig(cfg))
			if err != nil {
				t.Fatal(err)
			}
			defer cli.(io.Closer).Close()
			err = cli.Call(context.Background(), "echo", &mockthrift.MockTestArgs{Req: &mockthrift.MockReq{Msg: "blackhole"}}, new(mockthrift.MockTestResult))
			if !kerrors.IsTimeoutError(err) {
				t.Fatalf("expected RPC timeout, got %v", err)
			}
			select {
			case <-requestSeen:
			case <-time.After(time.Second):
				t.Fatal("RPC never reached target")
			}
			trackingMu.Lock()
			observed := tracked
			trackingMu.Unlock()
			select {
			case readErr := <-observed.readDone:
				var ne net.Error
				if !errors.As(readErr, &ne) || !ne.Timeout() {
					t.Fatalf("transport read did not time out: %v", readErr)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("outer Call returned but transport read is still blocked")
			}
			select {
			case <-observed.closed:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timed-out transport connection not closed")
			}
			select {
			case <-peerClosed:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("target connection remains open")
			}
		})
	}
}
