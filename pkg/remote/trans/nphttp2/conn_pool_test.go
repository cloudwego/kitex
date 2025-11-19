/*
 * Copyright 2022 CloudWeGo Authors
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

package nphttp2

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
)

// mockClientTransport implements grpc.ClientTransport for testing
type mockClientTransport struct {
	activeStreams int
	isActive      bool
	closed        bool
}

func (m *mockClientTransport) Write(s *grpc.Stream, hdr, data []byte, opts *grpc.Options) error {
	return nil
}

func (m *mockClientTransport) NewStream(ctx context.Context, callHdr *grpc.CallHdr) (*grpc.Stream, error) {
	return nil, nil
}

func (m *mockClientTransport) CloseStream(stream *grpc.Stream, err error) {
}

func (m *mockClientTransport) Error() <-chan struct{} {
	return nil
}

func (m *mockClientTransport) GoAway() <-chan struct{} {
	return nil
}

func (m *mockClientTransport) GetGoAwayReason() grpc.GoAwayReason {
	return grpc.GoAwayInvalid
}

func (m *mockClientTransport) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

func (m *mockClientTransport) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9090}
}

func (m *mockClientTransport) GracefulClose() {
	m.closed = true
}

func (m *mockClientTransport) Close(err error) error {
	m.closed = true
	return nil
}

func (m *mockClientTransport) ActiveStreams(tag string) int {
	return m.activeStreams
}

func (m *mockClientTransport) IsActive() bool {
	return m.isActive
}

var _ grpc.ClientTransport = (*mockClientTransport)(nil)
var _ grpc.IsActive = (*mockClientTransport)(nil)

func TestTransports_ActiveNums(t *testing.T) {
	testAddr := "127.0.0.1:8080"
	// Test with empty transports
	trans := &transports{
		size:          3,
		cliTransports: make([]grpc.ClientTransport, 3),
	}

	if got := trans.activeNums(testAddr); got != 0 {
		t.Errorf("activeNums() on empty transports = %d, want 0", got)
	}

	// Test with one transport
	trans.cliTransports[0] = &mockClientTransport{activeStreams: 5, isActive: true}
	if got := trans.activeNums(testAddr); got != 5 {
		t.Errorf("activeNums() with one transport = %d, want 5", got)
	}

	// Test with multiple transports
	trans.cliTransports[1] = &mockClientTransport{activeStreams: 10, isActive: true}
	trans.cliTransports[2] = &mockClientTransport{activeStreams: 3, isActive: true}
	if got := trans.activeNums(testAddr); got != 18 {
		t.Errorf("activeNums() with multiple transports = %d, want 18", got)
	}

	// Test with nil transport in the middle
	trans.cliTransports[1] = nil
	if got := trans.activeNums(testAddr); got != 8 {
		t.Errorf("activeNums() with nil transport = %d, want 8", got)
	}
}

func TestTransports_ActiveNums_Concurrent(t *testing.T) {
	testAddr := "127.0.0.1:8080"
	trans := &transports{
		size:          3,
		cliTransports: make([]grpc.ClientTransport, 3),
	}

	// Initialize with mock transports
	for i := 0; i < 3; i++ {
		trans.cliTransports[i] = &mockClientTransport{activeStreams: i + 1, isActive: true}
	}

	var wg sync.WaitGroup
	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = trans.activeNums(testAddr)
		}()
	}

	// Concurrent writes (put)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			trans.put(&mockClientTransport{activeStreams: idx, isActive: true})
		}(i)
	}

	wg.Wait()
}

func TestConnPool_ActiveStreams(t *testing.T) {
	pool := NewConnPool("test-service", 3, grpc.ConnectOptions{})

	// Test with non-existent address
	if got := pool.ActiveStreams("addr1"); got != 0 {
		t.Errorf("ActiveStreams() for non-existent address = %d, want 0", got)
	}

	// Test with existing address
	trans := &transports{
		size:          3,
		cliTransports: make([]grpc.ClientTransport, 3),
	}
	trans.cliTransports[0] = &mockClientTransport{activeStreams: 5, isActive: true}
	trans.cliTransports[1] = &mockClientTransport{activeStreams: 3, isActive: true}
	trans.cliTransports[2] = &mockClientTransport{activeStreams: 2, isActive: true}

	pool.conns.Store("addr1", trans)

	if got := pool.ActiveStreams("addr1"); got != 10 {
		t.Errorf("ActiveStreams() for addr1 = %d, want 10", got)
	}

	// Test with multiple addresses
	trans2 := &transports{
		size:          2,
		cliTransports: make([]grpc.ClientTransport, 2),
	}
	trans2.cliTransports[0] = &mockClientTransport{activeStreams: 7, isActive: true}
	trans2.cliTransports[1] = &mockClientTransport{activeStreams: 8, isActive: true}

	pool.conns.Store("addr2", trans2)

	if got := pool.ActiveStreams("addr2"); got != 15 {
		t.Errorf("ActiveStreams() for addr2 = %d, want 15", got)
	}

	// First address should still return correct value
	if got := pool.ActiveStreams("addr1"); got != 10 {
		t.Errorf("ActiveStreams() for addr1 after adding addr2 = %d, want 10", got)
	}
}

func TestConnPool_ActiveStreams_Concurrent(t *testing.T) {
	pool := NewConnPool("test-service", 3, grpc.ConnectOptions{})

	// Setup initial state
	trans := &transports{
		size:          3,
		cliTransports: make([]grpc.ClientTransport, 3),
	}
	for i := 0; i < 3; i++ {
		trans.cliTransports[i] = &mockClientTransport{activeStreams: i + 1, isActive: true}
	}
	pool.conns.Store("addr1", trans)

	var wg sync.WaitGroup
	// Concurrent reads from multiple goroutines
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.ActiveStreams("addr1")
		}()
	}

	// Concurrent reads for different addresses
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.ActiveStreams("addr2") // Non-existent address
		}()
	}

	wg.Wait()
}

func TestTransports_Get_WithLock(t *testing.T) {
	trans := &transports{
		size:          3,
		cliTransports: make([]grpc.ClientTransport, 3),
	}

	// Initialize transports
	for i := 0; i < 3; i++ {
		trans.cliTransports[i] = &mockClientTransport{activeStreams: i, isActive: true}
	}

	var wg sync.WaitGroup
	// Concurrent get operations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr := trans.get()
			if tr == nil {
				t.Errorf("get() returned nil")
			}
		}()
	}

	// Concurrent activeNums operations (read lock)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = trans.activeNums("127.0.0.1:8080")
		}()
	}

	wg.Wait()
}

func TestConnPool_ActiveStreams_WithNilTransport(t *testing.T) {
	testAddr := "127.0.0.1:8080"
	trans := &transports{
		size:          3,
		cliTransports: make([]grpc.ClientTransport, 3),
	}

	// Only set some transports
	trans.cliTransports[0] = &mockClientTransport{activeStreams: 5, isActive: true}
	// trans.cliTransports[1] is nil
	trans.cliTransports[2] = &mockClientTransport{activeStreams: 3, isActive: true}

	// Should handle nil transport gracefully
	got := trans.activeNums(testAddr)
	want := 8 // 5 + 0 + 3
	if got != want {
		t.Errorf("activeNums() with nil transport = %d, want %d", got, want)
	}
}

func BenchmarkTransports_ActiveNums(b *testing.B) {
	testAddr := "127.0.0.1:8080"
	trans := &transports{
		size:          10,
		cliTransports: make([]grpc.ClientTransport, 10),
	}

	for i := 0; i < 10; i++ {
		trans.cliTransports[i] = &mockClientTransport{activeStreams: i, isActive: true}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trans.activeNums(testAddr)
	}
}

func BenchmarkConnPool_ActiveStreams(b *testing.B) {
	pool := NewConnPool("test-service", 3, grpc.ConnectOptions{})

	trans := &transports{
		size:          3,
		cliTransports: make([]grpc.ClientTransport, 3),
	}
	for i := 0; i < 3; i++ {
		trans.cliTransports[i] = &mockClientTransport{activeStreams: i + 1, isActive: true}
	}
	pool.conns.Store("addr1", trans)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.ActiveStreams("addr1")
	}
}

func BenchmarkTransports_ActiveNums_Parallel(b *testing.B) {
	testAddr := "127.0.0.1:8080"
	trans := &transports{
		size:          10,
		cliTransports: make([]grpc.ClientTransport, 10),
	}

	for i := 0; i < 10; i++ {
		trans.cliTransports[i] = &mockClientTransport{activeStreams: i, isActive: true}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			trans.activeNums(testAddr)
		}
	})
}

func TestConnPool(t *testing.T) {
	// mock init
	connPool := newMockConnPool()
	defer connPool.Close()
	opt := newMockConnOption()
	ctx := newMockCtxWithRPCInfo()

	// test Get()
	_, err := connPool.Get(ctx, "tcp", mockAddr0, opt)
	test.Assert(t, err == nil, err)

	// test connection reuse
	// keep create new connection until filling the pool size,
	// then Get() will reuse established connection by round-robin
	for i := 0; uint32(i) < connPool.size*2; i++ {
		_, err := connPool.Get(ctx, "tcp", mockAddr1, opt)
		test.Assert(t, err == nil, err)
	}

	// test TimeoutGet()
	opt.ConnectTimeout = time.Microsecond
	_, err = connPool.Get(ctx, "tcp", mockAddr0, opt)
	test.Assert(t, err != nil, err)

	// test Clean()
	connPool.Clean("tcp", mockAddr0)
	_, ok := connPool.conns.Load(mockAddr0)
	test.Assert(t, !ok)
}

func TestReleaseConn(t *testing.T) {
	// short connection
	// the connection will be released when put back to the pool
	var closed1 int32 = 0
	dialFunc1 := func(network, address string, timeout time.Duration) (net.Conn, error) {
		npConn := newMockNpConn(address)
		npConn.CloseFunc = func() error {
			atomic.StoreInt32(&closed1, 1)
			return nil
		}
		npConn.mockSettingFrame()
		return npConn, nil
	}
	shortCP := newMockConnPool()
	shortCP.connOpts.ShortConn = true
	defer shortCP.Close()

	conn, err := shortCP.Get(newMockCtxWithRPCInfo(), "tcp", mockAddr0, remote.ConnOption{Dialer: newMockDialerWithDialFunc(dialFunc1)})
	// close stream to ensure no active stream on this connection,
	// which will be released when put back to the connection pool and closed by GracefulClose
	s := conn.(*clientConn).s
	conn.(*clientConn).tr.CloseStream(s, nil)
	test.Assert(t, err == nil, err)
	time.Sleep(100 * time.Millisecond)
	shortCP.Put(conn)
	test.Assert(t, atomic.LoadInt32(&closed1) == 1)

	// long connection
	// the connection will not be released when put back to the pool
	var closed2 int32 = 0
	dialFunc2 := func(network, address string, timeout time.Duration) (net.Conn, error) {
		npConn := newMockNpConn(address)
		npConn.CloseFunc = func() error {
			atomic.StoreInt32(&closed2, 1)
			return nil
		}
		npConn.mockSettingFrame()
		return npConn, nil
	}
	longCP := newMockConnPool()
	longCP.connOpts.ShortConn = false
	defer longCP.Close()

	longConn, err := longCP.Get(newMockCtxWithRPCInfo(), "tcp", mockAddr0, remote.ConnOption{Dialer: newMockDialerWithDialFunc(dialFunc2)})
	longConn.Close()
	test.Assert(t, err == nil, err)
	longCP.Put(longConn)
	test.Assert(t, atomic.LoadInt32(&closed2) == 0)
}
