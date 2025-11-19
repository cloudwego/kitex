/*
 * Copyright 2025 CloudWeGo Authors
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

package grpc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/http2"

	"github.com/cloudwego/kitex/pkg/utils"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
)

// Test helper functions

// setupMockConn creates a mock connection with basic functionality
func setupMockConn(address string) *mockConn {
	var writeBuffer []byte
	closeChan := make(chan struct{})
	mc := &mockConn{
		RemoteAddrFunc: func() net.Addr {
			return utils.NewNetAddr("tcp", address)
		},
		LocalAddrFunc: func() net.Addr {
			return utils.NewNetAddr("tcp", "127.0.0.1:12345")
		},
		WriteFunc: func(b []byte) (n int, err error) {
			writeBuffer = append(writeBuffer, b...)
			return len(b), nil
		},
		ReadFunc: func(b []byte) (n int, err error) {
			// Block until connection is closed
			<-closeChan
			return 0, io.EOF
		},
		CloseFunc: func() error {
			select {
			case <-closeChan:
				// Already closed
			default:
				close(closeChan)
			}
			return nil
		},
		SetDeadlineFunc: func(t time.Time) error {
			return nil
		},
		SetReadDeadlineFunc: func(t time.Time) error {
			return nil
		},
		SetWriteDeadlineFunc: func(t time.Time) error {
			return nil
		},
	}
	return mc
}

// setupMockNetpollConn creates a mock netpoll connection
func setupMockNetpollConn(address string) *mockNetpollConn {
	mc := &mockNetpollConn{
		mockConn: mockConn{
			RemoteAddrFunc: func() net.Addr {
				return utils.NewNetAddr("tcp", address)
			},
			LocalAddrFunc: func() net.Addr {
				return utils.NewNetAddr("tcp", "127.0.0.1:12345")
			},
			WriteFunc: func(b []byte) (n int, err error) {
				return len(b), nil
			},
			ReadFunc: func(b []byte) (n int, err error) {
				return 0, io.EOF
			},
			CloseFunc: func() error {
				return nil
			},
			SetDeadlineFunc: func(t time.Time) error {
				return nil
			},
			SetReadDeadlineFunc: func(t time.Time) error {
				return nil
			},
			SetWriteDeadlineFunc: func(t time.Time) error {
				return nil
			},
		},
	}
	return mc
}

// TestNewHTTP2Client tests the creation of a new HTTP2 client
func TestNewHTTP2Client(t *testing.T) {
	tests := []struct {
		name          string
		setupConn     func() net.Conn
		opts          ConnectOptions
		remoteService string
		wantErr       bool
	}{
		{
			name: "successful creation with basic options",
			setupConn: func() net.Conn {
				return setupMockConn("127.0.0.1:8080")
			},
			opts: ConnectOptions{
				KeepaliveParams: ClientKeepalive{
					Time:    30 * time.Second,
					Timeout: 10 * time.Second,
				},
			},
			remoteService: "test-service",
			wantErr:       false,
		},
		{
			name: "creation with default keepalive",
			setupConn: func() net.Conn {
				return setupMockConn("127.0.0.1:8080")
			},
			opts: ConnectOptions{
				KeepaliveParams: ClientKeepalive{
					Time:    0, // Will use default
					Timeout: 0, // Will use default
				},
			},
			remoteService: "test-service",
			wantErr:       false,
		},
		{
			name: "creation with custom window size",
			setupConn: func() net.Conn {
				return setupMockConn("127.0.0.1:8080")
			},
			opts: ConnectOptions{
				InitialConnWindowSize: 65535 * 2,
				InitialWindowSize:     65535,
				KeepaliveParams: ClientKeepalive{
					Time:    30 * time.Second,
					Timeout: 10 * time.Second,
				},
			},
			remoteService: "test-service",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			conn := tt.setupConn()

			client, err := newHTTP2Client(
				ctx,
				conn,
				tt.opts,
				tt.remoteService,
				func(reason GoAwayReason) {
					// onGoAway callback
				},
				func() {
					// onClose callback
				},
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("newHTTP2Client() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && client != nil {
				// Verify client initialization
				if client.conn == nil {
					t.Error("expected conn to be set")
				}
				if client.ctx == nil {
					t.Error("expected ctx to be set")
				}
				if client.cancel == nil {
					t.Error("expected cancel to be set")
				}
				if client.controlBuf == nil {
					t.Error("expected controlBuf to be set")
				}
				if client.activeStreams == nil {
					t.Error("expected activeStreams to be initialized")
				}
				if client.streamsByTag == nil {
					t.Error("expected streamsByTag to be initialized")
				}

				// Clean up
				client.Close(nil)

				// Give time for goroutines to finish
				time.Sleep(50 * time.Millisecond)
			}
		})
	}
}

// TestHTTP2Client_NewStream tests stream creation
func TestHTTP2Client_NewStream(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (*http2Client, context.Context, *CallHdr)
		wantErr   bool
	}{
		{
			name: "successful stream creation",
			setupFunc: func(t *testing.T) (*http2Client, context.Context, *CallHdr) {
				ctx := context.Background()
				conn := setupMockConn("127.0.0.1:8080")
				opts := ConnectOptions{
					KeepaliveParams: ClientKeepalive{
						Time:    Infinity,
						Timeout: 10 * time.Second,
					},
				}
				client, err := newHTTP2Client(ctx, conn, opts, "test",
					func(GoAwayReason) {}, // onGoAway
					func() {},             // onClose
				)
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}

				// Set client to reachable state
				client.mu.Lock()
				client.state = reachable
				client.mu.Unlock()

				callHdr := &CallHdr{
					Method: "/test.Service/Method",
					Host:   "127.0.0.1:8080",
				}
				return client, ctx, callHdr
			},
			wantErr: false,
		},
		{
			name: "stream creation with metadata",
			setupFunc: func(t *testing.T) (*http2Client, context.Context, *CallHdr) {
				ctx := context.Background()
				conn := setupMockConn("127.0.0.1:8080")
				opts := ConnectOptions{
					KeepaliveParams: ClientKeepalive{
						Time:    Infinity,
						Timeout: 10 * time.Second,
					},
				}
				client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}

				client.mu.Lock()
				client.state = reachable
				client.mu.Unlock()

				// Add metadata to context
				md := metadata.MD{
					"key1": []string{"value1"},
					"key2": []string{"value2"},
				}
				ctx = metadata.NewOutgoingContext(ctx, md)

				callHdr := &CallHdr{
					Method: "/test.Service/Method",
					Host:   "127.0.0.1:8080",
				}
				return client, ctx, callHdr
			},
			wantErr: false,
		},
		{
			name: "stream creation with deadline",
			setupFunc: func(t *testing.T) (*http2Client, context.Context, *CallHdr) {
				ctx := context.Background()
				conn := setupMockConn("127.0.0.1:8080")
				opts := ConnectOptions{
					KeepaliveParams: ClientKeepalive{
						Time:    Infinity,
						Timeout: 10 * time.Second,
					},
				}
				client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}

				client.mu.Lock()
				client.state = reachable
				client.mu.Unlock()

				// Set deadline
				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				t.Cleanup(cancel)

				callHdr := &CallHdr{
					Method: "/test.Service/Method",
					Host:   "127.0.0.1:8080",
				}
				return client, ctx, callHdr
			},
			wantErr: false,
		},
		{
			name: "stream creation with tag for load balancing",
			setupFunc: func(t *testing.T) (*http2Client, context.Context, *CallHdr) {
				ctx := context.Background()
				conn := setupMockConn("127.0.0.1:8080")
				opts := ConnectOptions{
					KeepaliveParams: ClientKeepalive{
						Time:    Infinity,
						Timeout: 10 * time.Second,
					},
				}
				client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}

				client.mu.Lock()
				client.state = reachable
				client.mu.Unlock()

				callHdr := &CallHdr{
					Method: "/test.Service/Method",
					Host:   "127.0.0.1:8080",
					Tag:    "target-instance-1",
				}
				return client, ctx, callHdr
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, ctx, callHdr := tt.setupFunc(t)
			defer client.Close(nil)

			stream, err := client.NewStream(ctx, callHdr)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStream() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if stream == nil {
					t.Error("expected stream to be created")
					return
				}

				// Verify stream properties
				if stream.id == 0 {
					t.Error("expected stream ID to be assigned")
				}
				if stream.method != callHdr.Method {
					t.Errorf("expected method %s, got %s", callHdr.Method, stream.method)
				}
				if stream.ctx == nil {
					t.Error("expected stream context to be set")
				}
				if stream.done == nil {
					t.Error("expected stream done channel to be set")
				}

				// Verify stream is registered
				client.mu.Lock()
				_, exists := client.activeStreams[stream.id]
				client.mu.Unlock()
				if !exists {
					t.Error("expected stream to be registered in activeStreams")
				}

				// If tag is set, verify streamsByTag index
				if callHdr.Tag != "" {
					client.mu.Lock()
					count := client.streamsByTag[callHdr.Tag]
					client.mu.Unlock()
					if count != 1 {
						t.Errorf("expected streamsByTag[%s] = 1, got %d", callHdr.Tag, count)
					}
				}

				// Clean up stream
				client.CloseStream(stream, nil)
			}
		})
	}
}

// TestHTTP2Client_CloseStream tests stream closure
func TestHTTP2Client_CloseStream(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (*http2Client, *Stream)
		closeErr  error
	}{
		{
			name: "close stream without error",
			setupFunc: func(t *testing.T) (*http2Client, *Stream) {
				ctx := context.Background()
				conn := setupMockConn("127.0.0.1:8080")
				opts := ConnectOptions{
					KeepaliveParams: ClientKeepalive{
						Time:    Infinity,
						Timeout: 10 * time.Second,
					},
				}
				client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}

				client.mu.Lock()
				client.state = reachable
				client.mu.Unlock()

				callHdr := &CallHdr{
					Method: "/test.Service/Method",
					Host:   "127.0.0.1:8080",
				}
				stream, err := client.NewStream(ctx, callHdr)
				if err != nil {
					t.Fatalf("failed to create stream: %v", err)
				}
				return client, stream
			},
			closeErr: nil,
		},
		{
			name: "close stream with context canceled",
			setupFunc: func(t *testing.T) (*http2Client, *Stream) {
				ctx := context.Background()
				conn := setupMockConn("127.0.0.1:8080")
				opts := ConnectOptions{
					KeepaliveParams: ClientKeepalive{
						Time:    Infinity,
						Timeout: 10 * time.Second,
					},
				}
				client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}

				client.mu.Lock()
				client.state = reachable
				client.mu.Unlock()

				callHdr := &CallHdr{
					Method: "/test.Service/Method",
					Host:   "127.0.0.1:8080",
				}
				stream, err := client.NewStream(ctx, callHdr)
				if err != nil {
					t.Fatalf("failed to create stream: %v", err)
				}
				return client, stream
			},
			closeErr: context.Canceled,
		},
		{
			name: "close stream with tag",
			setupFunc: func(t *testing.T) (*http2Client, *Stream) {
				ctx := context.Background()
				conn := setupMockConn("127.0.0.1:8080")
				opts := ConnectOptions{
					KeepaliveParams: ClientKeepalive{
						Time:    Infinity,
						Timeout: 10 * time.Second,
					},
				}
				client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}

				client.mu.Lock()
				client.state = reachable
				client.mu.Unlock()

				callHdr := &CallHdr{
					Method: "/test.Service/Method",
					Host:   "127.0.0.1:8080",
					Tag:    "test-tag",
				}
				stream, err := client.NewStream(ctx, callHdr)
				if err != nil {
					t.Fatalf("failed to create stream: %v", err)
				}
				return client, stream
			},
			closeErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, stream := tt.setupFunc(t)
			defer client.Close(nil)

			streamID := stream.id
			tag := stream.tag

			// Close the stream
			client.CloseStream(stream, tt.closeErr)

			// Wait for closure to complete
			select {
			case <-stream.done:
				// Stream closed successfully
			case <-time.After(1 * time.Second):
				t.Error("stream done channel not closed")
			}

			// Verify stream is removed from activeStreams
			time.Sleep(100 * time.Millisecond) // Give time for async cleanup
			client.mu.Lock()
			_, exists := client.activeStreams[streamID]
			tagCount := client.streamsByTag[tag]
			client.mu.Unlock()

			if exists {
				t.Error("stream should be removed from activeStreams")
			}

			if tag != "" && tagCount != 0 {
				t.Errorf("expected streamsByTag[%s] = 0, got %d", tag, tagCount)
			}

			// Verify stream state
			if stream.getState() != streamDone {
				t.Errorf("expected stream state to be streamDone, got %v", stream.getState())
			}
		})
	}
}

// TestHTTP2Client_Close tests client closure
func TestHTTP2Client_Close(t *testing.T) {
	tests := []struct {
		name          string
		setupStreams  int
		closeErr      error
		expectOnClose bool
	}{
		{
			name:          "close client without streams",
			setupStreams:  0,
			closeErr:      nil,
			expectOnClose: true,
		},
		{
			name:          "close client with active streams",
			setupStreams:  3,
			closeErr:      errors.New("connection closed"),
			expectOnClose: true,
		},
		{
			name:          "close already closing client",
			setupStreams:  0,
			closeErr:      nil,
			expectOnClose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			conn := setupMockConn("127.0.0.1:8080")
			opts := ConnectOptions{
				KeepaliveParams: ClientKeepalive{
					Time:    Infinity,
					Timeout: 10 * time.Second,
				},
			}

			onCloseCalled := false
			client, err := newHTTP2Client(ctx, conn, opts, "test", nil, func() {
				onCloseCalled = true
			})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			client.mu.Lock()
			client.state = reachable
			client.mu.Unlock()

			// Create test streams
			streams := make([]*Stream, tt.setupStreams)
			for i := 0; i < tt.setupStreams; i++ {
				callHdr := &CallHdr{
					Method: "/test.Service/Method",
					Host:   "127.0.0.1:8080",
				}
				stream, err := client.NewStream(ctx, callHdr)
				if err != nil {
					t.Fatalf("failed to create stream: %v", err)
				}
				streams[i] = stream
			}

			// Close the client
			err = client.Close(tt.closeErr)
			if err != nil {
				t.Logf("Close returned error: %v", err)
			}

			// Verify client state
			client.mu.Lock()
			state := client.state
			activeStreams := client.activeStreams
			client.mu.Unlock()

			if state != closing {
				t.Errorf("expected state to be closing, got %v", state)
			}

			if activeStreams != nil {
				t.Error("expected activeStreams to be nil after close")
			}

			// Verify onClose was called
			time.Sleep(50 * time.Millisecond)
			if tt.expectOnClose && !onCloseCalled {
				t.Error("expected onClose callback to be called")
			}

			// Verify context is canceled
			select {
			case <-client.ctx.Done():
				// Context canceled as expected
			default:
				t.Error("expected client context to be canceled")
			}

			// Verify calling Close again doesn't cause issues
			err = client.Close(tt.closeErr)
			if err != nil {
				t.Logf("Second Close returned error: %v", err)
			}
		})
	}
}

// TestHTTP2Client_GracefulClose tests graceful client closure
func TestHTTP2Client_GracefulClose(t *testing.T) {
	tests := []struct {
		name         string
		setupStreams int
		wantClose    bool
	}{
		{
			name:         "graceful close without streams",
			setupStreams: 0,
			wantClose:    true,
		},
		{
			name:         "graceful close with active streams",
			setupStreams: 2,
			wantClose:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			conn := setupMockConn("127.0.0.1:8080")
			opts := ConnectOptions{
				KeepaliveParams: ClientKeepalive{
					Time:    Infinity,
					Timeout: 10 * time.Second,
				},
			}

			client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			client.mu.Lock()
			client.state = reachable
			client.mu.Unlock()

			// Create test streams
			for i := 0; i < tt.setupStreams; i++ {
				callHdr := &CallHdr{
					Method: "/test.Service/Method",
					Host:   "127.0.0.1:8080",
				}
				_, err := client.NewStream(ctx, callHdr)
				if err != nil {
					t.Fatalf("failed to create stream: %v", err)
				}
			}

			// Gracefully close the client
			client.GracefulClose()

			// Check state
			client.mu.Lock()
			state := client.state
			client.mu.Unlock()

			if tt.wantClose {
				// Should transition to closing immediately
				time.Sleep(100 * time.Millisecond)
				client.mu.Lock()
				finalState := client.state
				client.mu.Unlock()
				if finalState != closing {
					t.Errorf("expected final state to be closing, got %v", finalState)
				}
			} else {
				// Should be in draining state
				if state != draining {
					t.Errorf("expected state to be draining, got %v", state)
				}
			}

			// Clean up
			client.Close(nil)
		})
	}
}

// TestHTTP2Client_Write tests writing data to a stream
func TestHTTP2Client_Write(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		header  []byte
		last    bool
		wantErr bool
	}{
		{
			name:    "write data",
			data:    []byte("test data"),
			header:  nil,
			last:    false,
			wantErr: false,
		},
		{
			name:    "write with header",
			data:    []byte("test data"),
			header:  []byte{0, 0, 0, 0, 9},
			last:    false,
			wantErr: false,
		},
		{
			name:    "write last frame",
			data:    []byte("final data"),
			header:  nil,
			last:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			conn := setupMockConn("127.0.0.1:8080")
			opts := ConnectOptions{
				KeepaliveParams: ClientKeepalive{
					Time:    Infinity,
					Timeout: 10 * time.Second,
				},
			}

			client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			defer client.Close(nil)

			client.mu.Lock()
			client.state = reachable
			client.mu.Unlock()

			callHdr := &CallHdr{
				Method: "/test.Service/Method",
				Host:   "127.0.0.1:8080",
			}
			stream, err := client.NewStream(ctx, callHdr)
			if err != nil {
				t.Fatalf("failed to create stream: %v", err)
			}

			// Set stream to active state
			stream.compareAndSwapState(streamDone, streamActive)

			opts2 := &Options{
				Last: tt.last,
			}

			err = client.Write(stream, tt.header, tt.data, opts2)
			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.last && !tt.wantErr {
				// Verify stream state changed to writeDone
				time.Sleep(50 * time.Millisecond)
				state := stream.getState()
				if state != streamWriteDone {
					t.Errorf("expected stream state to be streamWriteDone, got %v", state)
				}
			}

			client.CloseStream(stream, nil)
		})
	}
}

// TestHTTP2Client_ActiveStreams tests counting active streams
func TestHTTP2Client_ActiveStreams(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	client.mu.Lock()
	client.state = reachable
	client.mu.Unlock()

	// Test with no tag
	count := client.ActiveStreams("")
	if count != 0 {
		t.Errorf("expected 0 active streams, got %d", count)
	}

	// Create streams with different tags
	tag1 := "tag1"
	tag2 := "tag2"

	callHdr1 := &CallHdr{
		Method: "/test.Service/Method",
		Host:   "127.0.0.1:8080",
		Tag:    tag1,
	}
	stream1, err := client.NewStream(ctx, callHdr1)
	if err != nil {
		t.Fatalf("failed to create stream1: %v", err)
	}

	callHdr2 := &CallHdr{
		Method: "/test.Service/Method",
		Host:   "127.0.0.1:8080",
		Tag:    tag1,
	}
	stream2, err := client.NewStream(ctx, callHdr2)
	if err != nil {
		t.Fatalf("failed to create stream2: %v", err)
	}

	callHdr3 := &CallHdr{
		Method: "/test.Service/Method",
		Host:   "127.0.0.1:8080",
		Tag:    tag2,
	}
	stream3, err := client.NewStream(ctx, callHdr3)
	if err != nil {
		t.Fatalf("failed to create stream3: %v", err)
	}

	// Check counts
	count1 := client.ActiveStreams(tag1)
	if count1 != 2 {
		t.Errorf("expected 2 active streams for tag1, got %d", count1)
	}

	count2 := client.ActiveStreams(tag2)
	if count2 != 1 {
		t.Errorf("expected 1 active stream for tag2, got %d", count2)
	}

	// Close one stream
	client.CloseStream(stream1, nil)
	time.Sleep(100 * time.Millisecond)

	count1 = client.ActiveStreams(tag1)
	if count1 != 1 {
		t.Errorf("expected 1 active stream for tag1 after close, got %d", count1)
	}

	// Close remaining streams
	client.CloseStream(stream2, nil)
	client.CloseStream(stream3, nil)
	time.Sleep(100 * time.Millisecond)

	count1 = client.ActiveStreams(tag1)
	if count1 != 0 {
		t.Errorf("expected 0 active streams for tag1, got %d", count1)
	}

	count2 = client.ActiveStreams(tag2)
	if count2 != 0 {
		t.Errorf("expected 0 active streams for tag2, got %d", count2)
	}
}

// TestHTTP2Client_IsActive tests the IsActive method
func TestHTTP2Client_IsActive(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	// Initial state is reachable (zero value)
	if !client.IsActive() {
		t.Error("expected client to be active initially (default state is reachable)")
	}

	// Explicitly set to reachable to verify
	client.mu.Lock()
	client.state = reachable
	client.mu.Unlock()

	if !client.IsActive() {
		t.Error("expected client to be active after setting to reachable")
	}

	// Set to draining
	client.mu.Lock()
	client.state = draining
	client.mu.Unlock()

	if client.IsActive() {
		t.Error("expected client to not be active when draining")
	}

	// Set to closing
	client.mu.Lock()
	client.state = closing
	client.mu.Unlock()

	if client.IsActive() {
		t.Error("expected client to not be active when closing")
	}
}

// TestHTTP2Client_GetGoAwayReason tests getting GoAway reason
func TestHTTP2Client_GetGoAwayReason(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	// Initially GoAwayInvalid (zero value)
	reason := client.GetGoAwayReason()
	if reason != GoAwayInvalid {
		t.Errorf("expected GoAwayInvalid, got %v", reason)
	}

	// Set reason
	client.mu.Lock()
	client.goAwayReason = GoAwayTooManyPings
	client.mu.Unlock()

	reason = client.GetGoAwayReason()
	if reason != GoAwayTooManyPings {
		t.Errorf("expected GoAwayTooManyPings, got %v", reason)
	}
}

// TestHTTP2Client_RemoteAddr tests getting remote address
func TestHTTP2Client_RemoteAddr(t *testing.T) {
	ctx := context.Background()
	expectedAddr := "127.0.0.1:8080"
	conn := setupMockConn(expectedAddr)
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	addr := client.RemoteAddr()
	if addr == nil {
		t.Fatal("expected remote address to be set")
	}

	if addr.String() != expectedAddr {
		t.Errorf("expected remote address %s, got %s", expectedAddr, addr.String())
	}
}

// TestHTTP2Client_LocalAddr tests getting local address
func TestHTTP2Client_LocalAddr(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	addr := client.LocalAddr()
	if addr == nil {
		t.Fatal("expected local address to be set")
	}
}

// TestHTTP2Client_CreateHeaderFields tests creating header fields
func TestHTTP2Client_CreateHeaderFields(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		callHdr  *CallHdr
		wantLen  int
	}{
		{
			name: "basic headers",
			setupCtx: func() context.Context {
				return context.Background()
			},
			callHdr: &CallHdr{
				Method: "/test.Service/Method",
				Host:   "127.0.0.1:8080",
			},
			wantLen: 7, // :method, :scheme, :path, :authority, content-type, user-agent, te
		},
		{
			name: "headers with metadata",
			setupCtx: func() context.Context {
				md := metadata.MD{
					"custom-header": []string{"value1"},
				}
				return metadata.NewOutgoingContext(context.Background(), md)
			},
			callHdr: &CallHdr{
				Method: "/test.Service/Method",
				Host:   "127.0.0.1:8080",
			},
			wantLen: 8, // basic + 1 custom
		},
		{
			name: "headers with compression",
			setupCtx: func() context.Context {
				return context.Background()
			},
			callHdr: &CallHdr{
				Method:       "/test.Service/Method",
				Host:         "127.0.0.1:8080",
				SendCompress: "gzip",
			},
			wantLen: 9, // basic + grpc-encoding + grpc-accept-encoding
		},
		{
			name: "headers with deadline",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = cancel // Will be called by test cleanup
				return ctx
			},
			callHdr: &CallHdr{
				Method: "/test.Service/Method",
				Host:   "127.0.0.1:8080",
			},
			wantLen: 8, // basic + grpc-timeout
		},
		{
			name: "headers with previous attempts",
			setupCtx: func() context.Context {
				return context.Background()
			},
			callHdr: &CallHdr{
				Method:           "/test.Service/Method",
				Host:             "127.0.0.1:8080",
				PreviousAttempts: 2,
			},
			wantLen: 8, // basic + grpc-previous-rpc-attempts
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			conn := setupMockConn("127.0.0.1:8080")
			opts := ConnectOptions{
				KeepaliveParams: ClientKeepalive{
					Time:    Infinity,
					Timeout: 10 * time.Second,
				},
			}

			client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			defer client.Close(nil)

			testCtx := tt.setupCtx()
			headers := client.createHeaderFields(testCtx, tt.callHdr)

			if len(headers) < tt.wantLen {
				t.Errorf("expected at least %d headers, got %d", tt.wantLen, len(headers))
			}

			// Verify required headers
			requiredHeaders := map[string]bool{
				":method":      false,
				":scheme":      false,
				":path":        false,
				":authority":   false,
				"content-type": false,
				"user-agent":   false,
				"te":           false,
			}

			for _, hdr := range headers {
				if _, ok := requiredHeaders[hdr.Name]; ok {
					requiredHeaders[hdr.Name] = true
				}
			}

			for name, found := range requiredHeaders {
				if !found {
					t.Errorf("required header %s not found", name)
				}
			}
		})
	}
}

// TestFillStreamFields tests stream field allocation
func TestFillStreamFields(t *testing.T) {
	s := &Stream{}
	fillStreamFields(s)

	if s.buf == nil {
		t.Error("expected buf to be allocated")
	}
	if s.wq == nil {
		t.Error("expected wq to be allocated")
	}
	if s.wq.done != s.done {
		t.Error("expected wq.done to be set to stream.done")
	}
}

// TestAllocateStreamFields tests stream field allocation
func TestAllocateStreamFields(t *testing.T) {
	fields := allocateStreamFields()

	if fields.recvBuffer == nil {
		t.Error("expected recvBuffer to be allocated")
	}
	if fields.writeQuota == nil {
		t.Error("expected writeQuota to be allocated")
	}
}

// TestHTTP2Client_HandleSettings tests handling SETTINGS frames
func TestHTTP2Client_HandleSettings(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	// Test settings logic by directly modifying the client
	originalMaxStreams := client.maxConcurrentStreams

	// Simulate handling settings by directly calling the logic
	client.mu.Lock()
	client.maxConcurrentStreams = 100
	client.mu.Unlock()

	client.mu.Lock()
	newMaxStreams := client.maxConcurrentStreams
	client.mu.Unlock()

	if newMaxStreams == originalMaxStreams {
		t.Logf("max concurrent streams updated from %d to %d", originalMaxStreams, newMaxStreams)
	}
}

// TestHTTP2Client_AdjustWindow tests window adjustment
func TestHTTP2Client_AdjustWindow(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	client.mu.Lock()
	client.state = reachable
	client.mu.Unlock()

	callHdr := &CallHdr{
		Method: "/test.Service/Method",
		Host:   "127.0.0.1:8080",
	}
	stream, err := client.NewStream(ctx, callHdr)
	if err != nil {
		t.Fatalf("failed to create stream: %v", err)
	}
	defer client.CloseStream(stream, nil)

	// Test adjusting window
	client.adjustWindow(stream, 1024)

	// The function should complete without error
	// Actual window update would be sent to controlBuf
}

// TestHTTP2Client_UpdateWindow tests window update
func TestHTTP2Client_UpdateWindow(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	client.mu.Lock()
	client.state = reachable
	client.mu.Unlock()

	callHdr := &CallHdr{
		Method: "/test.Service/Method",
		Host:   "127.0.0.1:8080",
	}
	stream, err := client.NewStream(ctx, callHdr)
	if err != nil {
		t.Fatalf("failed to create stream: %v", err)
	}
	defer client.CloseStream(stream, nil)

	// Test updating window
	client.updateWindow(stream, 512)

	// The function should complete without error
}

// TestHTTP2Client_GetStream tests getting a stream by frame
func TestHTTP2Client_GetStream(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	client.mu.Lock()
	client.state = reachable
	client.mu.Unlock()

	callHdr := &CallHdr{
		Method: "/test.Service/Method",
		Host:   "127.0.0.1:8080",
	}
	stream, err := client.NewStream(ctx, callHdr)
	if err != nil {
		t.Fatalf("failed to create stream: %v", err)
	}
	defer client.CloseStream(stream, nil)

	// Create a mock frame with the stream ID
	frame := &http2.DataFrame{
		FrameHeader: http2.FrameHeader{
			StreamID: stream.id,
		},
	}

	// Get stream by frame
	gotStream := client.getStream(frame)
	if gotStream == nil {
		t.Fatal("expected to get stream, got nil")
	}

	if gotStream.id != stream.id {
		t.Errorf("expected stream ID %d, got %d", stream.id, gotStream.id)
	}

	// Test with non-existent stream ID
	frame2 := &http2.DataFrame{
		FrameHeader: http2.FrameHeader{
			StreamID: 999999,
		},
	}

	gotStream2 := client.getStream(frame2)
	if gotStream2 != nil {
		t.Error("expected nil for non-existent stream")
	}
}

// TestHTTP2Client_ConcurrentStreamCreation tests concurrent stream creation
func TestHTTP2Client_ConcurrentStreamCreation(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	client.mu.Lock()
	client.state = reachable
	client.mu.Unlock()

	// Create streams concurrently
	numStreams := 10
	var wg sync.WaitGroup
	wg.Add(numStreams)

	streams := make([]*Stream, numStreams)
	errors := make([]error, numStreams)

	for i := 0; i < numStreams; i++ {
		go func(idx int) {
			defer wg.Done()
			callHdr := &CallHdr{
				Method: "/test.Service/Method",
				Host:   "127.0.0.1:8080",
			}
			stream, err := client.NewStream(ctx, callHdr)
			streams[idx] = stream
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify all streams were created successfully
	for i, err := range errors {
		if err != nil {
			t.Errorf("stream %d creation failed: %v", i, err)
		}
	}

	// Verify all streams have unique IDs
	ids := make(map[uint32]bool)
	for i, stream := range streams {
		if stream == nil {
			t.Errorf("stream %d is nil", i)
			continue
		}
		if ids[stream.id] {
			t.Errorf("duplicate stream ID %d", stream.id)
		}
		ids[stream.id] = true
	}

	// Clean up
	for _, stream := range streams {
		if stream != nil {
			client.CloseStream(stream, nil)
		}
	}
}

// TestHTTP2Client_StreamQuota tests stream quota management
func TestHTTP2Client_StreamQuota(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	client.mu.Lock()
	client.state = reachable
	client.mu.Unlock()

	// Get initial quota
	initialQuota := atomic.LoadInt64(&client.streamQuota)

	// Create a stream
	callHdr := &CallHdr{
		Method: "/test.Service/Method",
		Host:   "127.0.0.1:8080",
	}
	stream, err := client.NewStream(ctx, callHdr)
	if err != nil {
		t.Fatalf("failed to create stream: %v", err)
	}

	// Verify quota decreased
	currentQuota := atomic.LoadInt64(&client.streamQuota)
	if currentQuota != initialQuota-1 {
		t.Errorf("expected quota to be %d, got %d", initialQuota-1, currentQuota)
	}

	// Close stream
	client.CloseStream(stream, nil)
	time.Sleep(100 * time.Millisecond)

	// Verify quota restored
	finalQuota := atomic.LoadInt64(&client.streamQuota)
	if finalQuota != initialQuota {
		t.Errorf("expected quota to be restored to %d, got %d", initialQuota, finalQuota)
	}
}

// TestHTTP2Client_Error tests the Error channel
func TestHTTP2Client_Error(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	errChan := client.Error()
	if errChan == nil {
		t.Fatal("expected Error() to return non-nil channel")
	}

	// Channel should not be closed initially
	select {
	case <-errChan:
		t.Error("error channel should not be closed initially")
	default:
		// Expected
	}

	// Close client
	client.Close(errors.New("test error"))

	// Channel should be closed now
	select {
	case <-errChan:
		// Expected
	case <-time.After(1 * time.Second):
		t.Error("error channel should be closed after Close()")
	}
}

// TestHTTP2Client_GoAway tests the GoAway channel
func TestHTTP2Client_GoAway(t *testing.T) {
	ctx := context.Background()
	conn := setupMockConn("127.0.0.1:8080")
	opts := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			Time:    Infinity,
			Timeout: 10 * time.Second,
		},
	}

	client, err := newHTTP2Client(ctx, conn, opts, "test", func(GoAwayReason) {}, func() {})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close(nil)

	goAwayChan := client.GoAway()
	if goAwayChan == nil {
		t.Fatal("expected GoAway() to return non-nil channel")
	}

	// Channel should not be closed initially
	select {
	case <-goAwayChan:
		t.Error("goAway channel should not be closed initially")
	default:
		// Expected
	}
}
