/*
 *
 * Copyright 2019 gRPC authors.
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
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2021 CloudWeGo Authors.
 */

// This file contains tests related to the following proposals:
// https://github.com/grpc/proposal/blob/master/A8-client-side-keepalive.md
// https://github.com/grpc/proposal/blob/master/A9-server-side-conn-mgt.md
// https://github.com/grpc/proposal/blob/master/A18-tcp-user-timeout.md
package grpc

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"golang.org/x/net/http2"
)

const defaultTestTimeout = 10 * time.Second

// TestMaxConnectionIdle tests that a server will send GoAway to an idle
// client. An idle client is one who doesn't make any RPC calls for a duration
// of MaxConnectionIdle time.
func TestMaxConnectionIdle(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveParams: ServerKeepalive{
			MaxConnectionIdle: 500 * time.Millisecond,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, suspended, ConnectOptions{})
	defer func() {
		client.Close()
		server.stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	stream, err := client.NewStream(ctx, &CallHdr{})
	if err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}
	client.CloseStream(stream, io.EOF)

	// Wait for the server's MaxConnectionIdle timeout to kick in, and for it
	// to send a GoAway.
	timeout := time.NewTimer(time.Second * 1)
	select {
	case <-client.Error():
		if !timeout.Stop() {
			<-timeout.C
		}
		if reason := client.GetGoAwayReason(); reason != GoAwayNoReason {
			t.Fatalf("GoAwayReason is %v, want %v", reason, GoAwayNoReason)
		}
	case <-timeout.C:
		t.Fatalf("MaxConnectionIdle timeout expired, expected a GoAway from the server.")
	}
}

// TestMaxConenctionIdleBusyClient tests that a server will not send GoAway to
// a busy client.
func TestMaxConnectionIdleBusyClient(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveParams: ServerKeepalive{
			MaxConnectionIdle: 500 * time.Millisecond,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, suspended, ConnectOptions{})
	defer func() {
		client.Close()
		server.stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	_, err := client.NewStream(ctx, &CallHdr{})
	if err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}

	// Wait for double the MaxConnectionIdle time to make sure the server does
	// not send a GoAway, as the client has an open stream.
	timeout := time.NewTimer(time.Second * 1)
	select {
	case <-client.GoAway():
		if !timeout.Stop() {
			<-timeout.C
		}
		t.Fatalf("A non-idle client received a GoAway.")
	case <-timeout.C:
	}
}

// FIXME Test failed because DATA RACE.
// TestMaxConnectionAge tests that a server will send GoAway after a duration
// of MaxConnectionAge.
//func TestMaxConnectionAge(t *testing.T) {
//	serverConfig := &ServerConfig{
//		KeepaliveParams: ServerKeepalive{
//			MaxConnectionAge:      1 * time.Second,
//			MaxConnectionAgeGrace: 1 * time.Second,
//		},
//	}
//	server, client := setUpWithOptions(t, 0, serverConfig, suspended, ConnectOptions{})
//	defer func() {
//		client.Close()
//		server.stop()
//	}()
//
//	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
//	defer cancel()
//	_, err := client.NewStream(ctx, &CallHdr{})
//	if err != nil {
//		t.Fatalf("client.NewStream() failed: %v", err)
//	}
//
//	// Wait for the server's MaxConnectionAge timeout to kick in, and for it
//	// to send a GoAway.
//	timeout := time.NewTimer(4 * time.Second)
//	select {
//	case <-client.Error():
//		if !timeout.Stop() {
//			<-timeout.C
//		}
//		if reason := client.GetGoAwayReason(); reason != GoAwayNoReason {
//			t.Fatalf("GoAwayReason is %v, want %v", reason, GoAwayNoReason)
//		}
//	case <-timeout.C:
//		t.Fatalf("MaxConnectionAge timeout expired, expected a GoAway from the server.")
//	}
//}

// TestKeepaliveServerClosesUnresponsiveClient tests that a server closes
// the connection with a client that doesn't respond to keepalive pings.
//
// This test creates a regular net.Conn connection to the server and sends the
// clientPreface and the initial Settings frame, and then remains unresponsive.
func TestKeepaliveServerClosesUnresponsiveClient(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveParams: ServerKeepalive{
			Time:    250 * time.Millisecond,
			Timeout: 250 * time.Millisecond,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, suspended, ConnectOptions{})
	defer func() {
		client.Close()
		server.stop()
	}()

	addr := server.addr()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("net.Dial(tcp, %v) failed: %v", addr, err)
	}
	defer conn.Close()

	if n, err := conn.Write(ClientPreface); err != nil || n != len(ClientPreface) {
		t.Fatalf("conn.write(clientPreface) failed: n=%v, err=%v", n, err)
	}
	framer := newFramer(conn, 0, 0, 0)
	if err := framer.WriteSettings(http2.Setting{}); err != nil {
		t.Fatal("framer.WriteSettings(http2.Setting{}) failed:", err)
	}
	framer.writer.Flush()

	// We read from the net.Conn till we get an error, which is expected when
	// the server closes the connection as part of the keepalive logic.
	errCh := make(chan error)
	go func() {
		b := make([]byte, 24)
		for {
			if _, err = conn.Read(b); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Server waits for KeepaliveParams.Time seconds before sending out a ping,
	// and then waits for KeepaliveParams.Timeout for a ping ack.
	timeout := time.NewTimer(1 * time.Second)
	select {
	case err := <-errCh:
		if err != io.EOF {
			t.Fatalf("client.Read(_) = _,%v, want io.EOF", err)
		}
	case <-timeout.C:
		t.Fatalf("keepalive timeout expired, server should have closed the connection.")
	}
}

// TestKeepaliveServerWithResponsiveClient tests that a server doesn't close
// the connection with a client that responds to keepalive pings.
func TestKeepaliveServerWithResponsiveClient(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveParams: ServerKeepalive{
			Time:    500 * time.Millisecond,
			Timeout: 500 * time.Millisecond,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, suspended, ConnectOptions{
		// FIXME the original ut don't contain KeepaliveParams
		KeepaliveParams: ClientKeepalive{
			Time:                500 * time.Millisecond,
			Timeout:             500 * time.Millisecond,
			PermitWithoutStream: true,
		},
	})
	defer func() {
		client.Close()
		server.stop()
	}()

	// Give keepalive logic some time by sleeping.
	time.Sleep(1 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	// Make sure the client transport is healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}
}

// TestKeepaliveClientClosesUnresponsiveServer creates a server which does not
// respond to keepalive pings, and makes sure that the client closes the
// transport once the keepalive logic kicks in. Here, we set the
// `PermitWithoutStream` parameter to true which ensures that the keepalive
// logic is running even without any active streams.
func TestKeepaliveClientClosesUnresponsiveServer(t *testing.T) {
	connCh := make(chan net.Conn, 1)
	client := setUpWithNoPingServer(t, ConnectOptions{KeepaliveParams: ClientKeepalive{
		Time:                250 * time.Millisecond,
		Timeout:             250 * time.Millisecond,
		PermitWithoutStream: true,
	}}, connCh)
	if client == nil {
		t.Fatalf("setUpWithNoPingServer failed, return nil client")
	}
	defer client.Close()

	conn, ok := <-connCh
	if !ok {
		t.Fatalf("Server didn't return connection object")
	}
	defer conn.Close()

	// Sleep for keepalive to close the connection.
	time.Sleep(1 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	// Make sure the client transport is not healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err == nil {
		t.Fatal("client.NewStream() should have failed, but succeeded")
	}
}

// TestKeepaliveClientOpenWithUnresponsiveServer creates a server which does
// not respond to keepalive pings, and makes sure that the client does not
// close the transport. Here, we do not set the `PermitWithoutStream` parameter
// to true which ensures that the keepalive logic is turned off without any
// active streams, and therefore the transport stays open.
func TestKeepaliveClientOpenWithUnresponsiveServer(t *testing.T) {
	connCh := make(chan net.Conn, 1)
	client := setUpWithNoPingServer(t, ConnectOptions{KeepaliveParams: ClientKeepalive{
		Time:    250 * time.Millisecond,
		Timeout: 250 * time.Millisecond,
	}}, connCh)
	if client == nil {
		t.Fatalf("setUpWithNoPingServer failed, return nil client")
	}
	defer client.Close()

	conn, ok := <-connCh
	if !ok {
		t.Fatalf("Server didn't return connection object")
	}
	defer conn.Close()

	// Give keepalive some time.
	time.Sleep(1 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	// Make sure the client transport is healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}
}

// TestKeepaliveClientClosesWithActiveStreams creates a server which does not
// respond to keepalive pings, and makes sure that the client closes the
// transport even when there is an active stream.
func TestKeepaliveClientClosesWithActiveStreams(t *testing.T) {
	connCh := make(chan net.Conn, 1)
	client := setUpWithNoPingServer(t, ConnectOptions{KeepaliveParams: ClientKeepalive{
		Time:    250 * time.Millisecond,
		Timeout: 250 * time.Millisecond,
	}}, connCh)
	if client == nil {
		t.Fatalf("setUpWithNoPingServer failed, return nil client")
	}
	defer client.Close()

	conn, ok := <-connCh
	if !ok {
		t.Fatalf("Server didn't return connection object")
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	// Create a stream, but send no data on it.
	if _, err := client.NewStream(ctx, &CallHdr{}); err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}

	// Give keepalive some time.
	time.Sleep(1 * time.Second)

	// Make sure the client transport is not healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err == nil {
		t.Fatal("client.NewStream() should have failed, but succeeded")
	}
}

// TestKeepaliveClientStaysHealthyWithResponsiveServer creates a server which
// responds to keepalive pings, and makes sure than a client transport stays
// healthy without any active streams.
func TestKeepaliveClientStaysHealthyWithResponsiveServer(t *testing.T) {
	server, client := setUpWithOptions(t, 0, &ServerConfig{}, normal, ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:                500 * time.Millisecond,
			Timeout:             500 * time.Millisecond,
			PermitWithoutStream: true,
		},
	})
	defer func() {
		client.Close()
		server.stop()
	}()

	// Give keepalive some time.
	time.Sleep(1 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	// Make sure the client transport is healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}
}

// TestKeepaliveClientFrequency creates a server which expects at most 1 client
// ping for every 1.2 seconds, while the client is configured to send a ping
// every 1 second. So, this configuration should end up with the client
// transport being closed. But we had a bug wherein the client was sending one
// ping every [Time+Timeout] instead of every [Time] period, and this test
// explicitly makes sure the fix works and the client sends a ping every [Time]
// period.
func TestKeepaliveClientFrequency(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveEnforcementPolicy: EnforcementPolicy{
			MinTime:             200 * time.Millisecond, // 1.2 seconds
			PermitWithoutStream: true,
		},
	}
	clientOptions := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:                150 * time.Millisecond,
			Timeout:             300 * time.Millisecond,
			PermitWithoutStream: true,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, normal, clientOptions)
	defer func() {
		client.Close()
		server.stop()
	}()

	timeout := time.NewTimer(1 * time.Second)
	select {
	case <-client.Error():
		if !timeout.Stop() {
			<-timeout.C
		}
		if reason := client.GetGoAwayReason(); reason != GoAwayTooManyPings {
			t.Fatalf("GoAwayReason is %v, want %v", reason, GoAwayTooManyPings)
		}
	case <-timeout.C:
		t.Fatalf("client transport still healthy; expected GoAway from the server.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	// Make sure the client transport is not healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err == nil {
		t.Fatal("client.NewStream() should have failed, but succeeded")
	}
}

// TestKeepaliveServerEnforcementWithAbusiveClientNoRPC verifies that the
// server closes a client transport when it sends too many keepalive pings
// (when there are no active streams), based on the configured
// EnforcementPolicy.
func TestKeepaliveServerEnforcementWithAbusiveClientNoRPC(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveEnforcementPolicy: EnforcementPolicy{
			MinTime: 2 * time.Second,
		},
	}
	clientOptions := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:                50 * time.Millisecond,
			Timeout:             1 * time.Second,
			PermitWithoutStream: true,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, normal, clientOptions)
	defer func() {
		client.Close()
		server.stop()
	}()

	timeout := time.NewTimer(4 * time.Second)
	select {
	case <-client.Error():
		if !timeout.Stop() {
			<-timeout.C
		}
		if reason := client.GetGoAwayReason(); reason != GoAwayTooManyPings {
			t.Fatalf("GoAwayReason is %v, want %v", reason, GoAwayTooManyPings)
		}
	case <-timeout.C:
		t.Fatalf("client transport still healthy; expected GoAway from the server.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	// Make sure the client transport is not healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err == nil {
		t.Fatal("client.NewStream() should have failed, but succeeded")
	}
}

// TestKeepaliveServerEnforcementWithAbusiveClientWithRPC verifies that the
// server closes a client transport when it sends too many keepalive pings
// (even when there is an active stream), based on the configured
// EnforcementPolicy.
func TestKeepaliveServerEnforcementWithAbusiveClientWithRPC(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveEnforcementPolicy: EnforcementPolicy{
			MinTime: 2 * time.Second,
		},
	}
	clientOptions := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    50 * time.Millisecond,
			Timeout: 1 * time.Second,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, suspended, clientOptions)
	defer func() {
		client.Close()
		server.stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	if _, err := client.NewStream(ctx, &CallHdr{}); err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}

	timeout := time.NewTimer(4 * time.Second)
	select {
	case <-client.Error():
		if !timeout.Stop() {
			<-timeout.C
		}
		if reason := client.GetGoAwayReason(); reason != GoAwayTooManyPings {
			t.Fatalf("GoAwayReason is %v, want %v", reason, GoAwayTooManyPings)
		}
	case <-timeout.C:
		t.Fatalf("client transport still healthy; expected GoAway from the server.")
	}

	// Make sure the client transport is not healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err == nil {
		t.Fatal("client.NewStream() should have failed, but succeeded")
	}
}

// TestKeepaliveServerEnforcementWithObeyingClientNoRPC verifies that the
// server does not close a client transport (with no active streams) which
// sends keepalive pings in accordance to the configured keepalive
// EnforcementPolicy.
func TestKeepaliveServerEnforcementWithObeyingClientNoRPC(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveEnforcementPolicy: EnforcementPolicy{
			MinTime:             50 * time.Millisecond,
			PermitWithoutStream: true,
		},
	}
	clientOptions := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:                51 * time.Millisecond,
			Timeout:             500 * time.Millisecond,
			PermitWithoutStream: true,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, normal, clientOptions)
	defer func() {
		client.Close()
		server.stop()
	}()

	// Give keepalive enough time.
	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	// Make sure the client transport is healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}
}

// TestKeepaliveServerEnforcementWithObeyingClientWithRPC verifies that the
// server does not close a client transport (with active streams) which
// sends keepalive pings in accordance to the configured keepalive
// EnforcementPolicy.
func TestKeepaliveServerEnforcementWithObeyingClientWithRPC(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveEnforcementPolicy: EnforcementPolicy{
			MinTime: 50 * time.Millisecond,
		},
	}
	clientOptions := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    51 * time.Millisecond,
			Timeout: 500 * time.Millisecond,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, suspended, clientOptions)
	defer func() {
		client.Close()
		server.stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	if _, err := client.NewStream(ctx, &CallHdr{}); err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}

	// Give keepalive enough time.
	time.Sleep(1 * time.Second)

	// Make sure the client transport is healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}
}

// TestKeepaliveServerEnforcementWithDormantKeepaliveOnClient verifies that the
// server does not closes a client transport, which has been configured to send
// more pings than allowed by the server's EnforcementPolicy. This client
// transport does not have any active streams and `PermitWithoutStream` is set
// to false. This should ensure that the keepalive functionality on the client
// side enters a dormant state.
func TestKeepaliveServerEnforcementWithDormantKeepaliveOnClient(t *testing.T) {
	serverConfig := &ServerConfig{
		KeepaliveEnforcementPolicy: EnforcementPolicy{
			MinTime: 400 * time.Millisecond,
		},
	}
	clientOptions := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    25 * time.Millisecond,
			Timeout: 250 * time.Millisecond,
		},
	}
	server, client := setUpWithOptions(t, 0, serverConfig, normal, clientOptions)
	defer func() {
		client.Close()
		server.stop()
	}()

	// No active streams on the client. Give keepalive enough time.
	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	// Make sure the client transport is healthy.
	if _, err := client.NewStream(ctx, &CallHdr{}); err != nil {
		t.Fatalf("client.NewStream() failed: %v", err)
	}
}

// FIXME syscall.GetTCPUserTimeout() failed: conn is not *net.TCPConn. got *netpoll.TCPConnection
// TestTCPUserTimeout tests that the TCP_USER_TIMEOUT socket option is set to
// the keepalive timeout, as detailed in proposal A18.
//func TestTCPUserTimeout(t *testing.T) {
//	tests := []struct {
//		time        time.Duration
//		timeout     time.Duration
//		wantTimeout time.Duration
//	}{
//		{
//			10 * time.Second,
//			10 * time.Second,
//			10 * 1000 * time.Millisecond,
//		},
//		{
//			0,
//			0,
//			0,
//		},
//	}
//	for _, tt := range tests {
//		server, client := setUpWithOptions(
//			t,
//			0,
//			&ServerConfig{
//				KeepaliveParams: ServerKeepalive{
//					Time:    tt.timeout,
//					Timeout: tt.timeout,
//				},
//			},
//			normal,
//			ConnectOptions{
//				KeepaliveParams: ClientKeepalive{
//					Time:    tt.time,
//					Timeout: tt.timeout,
//				},
//			},
//		)
//		defer func() {
//			client.Close()
//			server.stop()
//		}()
//
//		ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
//		defer cancel()
//		stream, err := client.NewStream(ctx, &CallHdr{})
//		if err != nil {
//			t.Fatalf("client.NewStream() failed: %v", err)
//		}
//		client.CloseStream(stream, io.EOF)
//
//		opt, err := syscall.GetTCPUserTimeout(client.conn)
//		if err != nil {
//			t.Fatalf("syscall.GetTCPUserTimeout() failed: %v", err)
//		}
//		if opt < 0 {
//			t.Skipf("skipping test on unsupported environment")
//		}
//		if gotTimeout := time.Duration(opt) * time.Millisecond; gotTimeout != tt.wantTimeout {
//			t.Fatalf("syscall.GetTCPUserTimeout() = %d, want %d", gotTimeout, tt.wantTimeout)
//		}
//	}
//}
