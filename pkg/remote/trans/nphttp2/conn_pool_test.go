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
	"crypto/tls"
	"errors"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

func TestConnPool(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		// mock init
		connPool := newMockConnPool(nil)
		defer connPool.Close()
		opt := newMockConnOption()
		ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)

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

		// test Dump()
		dump := connPool.Dump()
		test.Assert(t, dump != nil)
		m := dump.(map[string]interface{})
		test.Assert(t, m[mockAddr0] != nil)
		test.Assert(t, m[mockAddr1] != nil)
		_, err = sonic.Marshal(dump)
		test.Assert(t, err == nil, err)

		// test Clean()
		connPool.Clean("tcp", mockAddr0)
		_, ok := connPool.conns.Load(mockAddr0)
		test.Assert(t, !ok)
	})
	t.Run("ctx canceled", func(t *testing.T) {
		// When stream quota is exhausted, NewStream blocks. If the user's ctx
		// expires while blocked, Get() should return the context error immediately
		// without entering singleflight (which would waste a dial or block others).
		//
		// Setup: mockNetpollConn with MaxConcurrentStreams=1 + blockOnQueueEmpty
		// to keep the reader goroutine alive after processing the SETTINGS frame.
		// Hold 1 stream → quota=0 → 2nd Get with short deadline blocks → timeout.
		blockCh := make(chan struct{})
		t.Cleanup(func() {
			select {
			case <-blockCh:
			default:
				close(blockCh)
			}
		})
		dialFunc := func(network, address string, timeout time.Duration) (net.Conn, error) {
			npConn := newMockNpConn(address)
			npConn.autoStartReader = true
			npConn.blockOnQueueEmpty = blockCh
			npConn.mockSettingFrameWithMaxStreams(1)
			return npConn, nil
		}
		opt := remote.ConnOption{
			Dialer:         newMockDialerWithDialFunc(dialFunc),
			ConnectTimeout: time.Second,
		}
		// Use size=1 so round-robin always hits the same slot.
		pool := NewConnPool("test", 1, grpc.ConnectOptions{
			WriteBufferSize: defaultMockReadWriteBufferSize,
			ReadBufferSize:  defaultMockReadWriteBufferSize,
		})
		defer pool.Close()

		// Create the first stream — triggers transport creation + reader goroutine.
		ctx1 := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
		ctx1Timeout, cancel1 := context.WithTimeout(ctx1, 2*time.Second)
		defer cancel1()
		conn1, err := pool.Get(ctx1Timeout, "tcp", mockAddr0, opt)
		test.Assert(t, err == nil, err)
		cc1 := conn1.(*clientConn)

		// Wait for the reader goroutine to process SETTINGS (MaxConcurrentStreams=1).
		// After our held stream, quota=0.
		time.Sleep(50 * time.Millisecond)

		// Second Get with a very short deadline — should block on quota then timeout.
		ctx2 := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
		ctx2Timeout, cancel2 := context.WithTimeout(ctx2, 80*time.Millisecond)
		defer cancel2()

		_, err = pool.Get(ctx2Timeout, "tcp", mockAddr0, opt)
		test.Assert(t, err != nil)
		test.Assert(t, ctx2Timeout.Err() != nil)

		// Verify the pool still has only the original transport —
		// the ctx.Done() fast exit in Get() should have prevented singleflight entry.
		v, ok := pool.conns.Load(mockAddr0)
		test.Assert(t, ok)
		allTransports := v.(*transports).loadAll()
		test.Assert(t, len(allTransports) == 1, len(allTransports))
		test.Assert(t, allTransports[0] == cc1.tr, allTransports[0])

		cc1.tr.CloseStream(cc1.s, nil)
	})
}

// getAndReleaseStream wraps pool.Get with a deadline context,
// holds the stream for a random duration (0~200ms) simulating real RPC latency,
// then closes it.
func getAndReleaseStream(pool *connPool, parent context.Context, network, address string, opt remote.ConnOption) error {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	conn, err := pool.Get(ctx, network, address, opt)
	if err != nil {
		return err
	}
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
	cc := conn.(*clientConn)
	cc.tr.CloseStream(cc.s, nil)
	return nil
}

func isExpectedShutdownErr(err error) bool {
	if err == nil || err == errTransportsClosed || err == errConnPoolClosed || err == grpc.ErrConnClosing {
		return true
	}
	st, ok := status.FromError(err)
	if ok {
		// create stream timeouts
		if st.Code() == codes.DeadlineExceeded && st.Message() == context.DeadlineExceeded.Error() {
			return true
		}
	}
	return false
}

func TestConnPoolConcurrent(t *testing.T) {
	t.Run("Get() and Clean() concurrently", func(t *testing.T) {
		// Simulates service discovery repeatedly marking an instance as
		// offline while traffic is still flowing.
		// Some Get calls are expected to fail with errTransportsClosed;
		// that is the intended semantic of Clean.
		pool := newMockConnPool(nil)
		t.Cleanup(func() { pool.Close() })
		opt := newMockConnOption()

		iterNum := 50
		var wg sync.WaitGroup
		var successNum, failNum, unexpectedErrs atomic.Int32

		stopDone := make(chan struct{})
		cleanerDone := make(chan struct{})
		go func() {
			defer close(cleanerDone)
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopDone:
					return
				case <-ticker.C:
					pool.Clean("tcp", mockAddr0)
				}
			}
		}()

		wg.Add(iterNum)
		for i := 0; i < iterNum; i++ {
			go func() {
				defer wg.Done()
				ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
				err := getAndReleaseStream(pool, ctx, "tcp", mockAddr0, opt)
				if err == nil {
					successNum.Add(1)
				} else {
					failNum.Add(1)
					if !isExpectedShutdownErr(err) {
						unexpectedErrs.Add(1)
						t.Error(err)
					}
				}
			}()
		}
		wg.Wait()
		close(stopDone)
		<-cleanerDone

		test.Assert(t, successNum.Load() > 0)
		test.Assert(t, successNum.Load()+failNum.Load() == int32(iterNum))
		test.Assert(t, unexpectedErrs.Load() == 0, unexpectedErrs.Load())
		t.Logf("success=%d fail=%d", successNum.Load(), failNum.Load())

		// After cleaning, Get() with the same addr should still succeed
		// (recovery). Use the helper so the stream quota is released too.
		ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
		test.Assert(t, getAndReleaseStream(pool, ctx, "tcp", mockAddr0, opt) == nil)
	})
	t.Run("Get() and Close() concurrently", func(t *testing.T) {
		// Simulates process shutdown: a flood of in-flight Gets racing with
		// pool.Close. Each Get immediately closes its stream so the per-
		// transport stream quota does not saturate, allowing pool.Close to
		// gracefully drain.
		pool := newMockConnPool(nil)
		opt := newMockConnOption()

		iterNum := 50
		var wg sync.WaitGroup
		var unexpectedErrs atomic.Int32
		wg.Add(iterNum)
		for i := 0; i < iterNum; i++ {
			go func() {
				defer wg.Done()
				ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
				err := getAndReleaseStream(pool, ctx, "tcp", mockAddr0, opt)
				if !isExpectedShutdownErr(err) {
					unexpectedErrs.Add(1)
					t.Error(err)
				}
			}()
		}

		time.Sleep(10 * time.Millisecond)
		test.Assert(t, pool.Close() == nil)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("goroutines did not exit after pool.Close")
		}
		test.Assert(t, unexpectedErrs.Load() == 0, "unexpected errs:", unexpectedErrs.Load())
	})
	t.Run("Dump() during concurrent Get", func(t *testing.T) {
		// Observability hit while traffic is in-flight. Dump must not
		// panic and its output must remain JSON-serializable.
		pool := newMockConnPool(nil)
		t.Cleanup(func() { pool.Close() })
		opt := newMockConnOption()

		var wg sync.WaitGroup
		stop := make(chan struct{})
		var getErrs, dumpNilCount, dumpMarshalErrs atomic.Int32
		// Concurrent Get goroutines. Each loop iteration closes its stream
		// to release the per-transport stream quota.
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
				addr := mockAddr0
				if idx%2 == 0 {
					addr = mockAddr1
				}
				for {
					select {
					case <-stop:
						return
					default:
						if err := getAndReleaseStream(pool, ctx, "tcp", addr, opt); err != nil {
							getErrs.Add(1)
						}
					}
				}
			}(i)
		}

		// Concurrent Dump + Marshal.
		var dumpCount atomic.Int32
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						dump := pool.Dump()
						if dump == nil {
							dumpNilCount.Add(1)
							continue
						}
						if _, err := sonic.Marshal(dump); err != nil {
							dumpMarshalErrs.Add(1)
						}
						dumpCount.Add(1)
					}
				}
			}()
		}

		time.Sleep(200 * time.Millisecond)
		close(stop)
		wg.Wait()
		test.Assert(t, dumpCount.Load() > 0)
		test.Assert(t, dumpNilCount.Load() == 0, "Dump returned nil:", dumpNilCount.Load())
		test.Assert(t, dumpMarshalErrs.Load() == 0, "Dump output not marshalable:", dumpMarshalErrs.Load())
		test.Assert(t, getErrs.Load() == 0, "unexpected Get errors:", getErrs.Load())
		t.Logf("dumped %d times", dumpCount.Load())
	})
	t.Run("Get() across multiple addresses", func(t *testing.T) {
		// Many addresses, many goroutines per address.
		// Verifies per-address transports are created independently without
		// cross-address races.
		pool := newMockConnPool(nil)
		t.Cleanup(func() { pool.Close() })
		opt := newMockConnOption()

		addrCount := 8
		perAddrGoroutines := 50

		addrs := make([]string, addrCount)
		for i := 0; i < addrCount; i++ {
			addrs[i] = "127.0.0.1:3000" + strconv.Itoa(i)
		}

		var wg sync.WaitGroup
		var successNum atomic.Int32
		for _, addr := range addrs {
			for j := 0; j < perAddrGoroutines; j++ {
				wg.Add(1)
				go func(a string) {
					defer wg.Done()
					ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
					if err := getAndReleaseStream(pool, ctx, "tcp", a, opt); err == nil {
						successNum.Add(1)
					}
				}(addr)
			}
		}
		wg.Wait()

		test.Assert(t, successNum.Load() == int32(addrCount*perAddrGoroutines),
			successNum.Load(), addrCount*perAddrGoroutines)

		for _, addr := range addrs {
			_, ok := pool.conns.Load(addr)
			test.Assert(t, ok, addr)
		}
	})
	t.Run("Get() after Clean()", func(t *testing.T) {
		// Instance goes offline (Clean) then back online. New Get must
		// build a fresh transports bag instead of reusing the closed one.
		// Use the helper for both Gets so stream quota is released and
		// pool.Close in t.Cleanup can drain cleanly.
		pool := newMockConnPool(nil)
		t.Cleanup(func() { pool.Close() })

		opt := newMockConnOption()
		ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
		test.Assert(t, getAndReleaseStream(pool, ctx, "tcp", mockAddr0, opt) == nil)

		// Grab the old transports pointer to verify it is discarded.
		oldV, ok := pool.conns.Load(mockAddr0)
		test.Assert(t, ok)
		oldTrans := oldV.(*transports)

		pool.Clean("tcp", mockAddr0)
		_, ok = pool.conns.Load(mockAddr0)
		test.Assert(t, !ok)

		// create a new one
		test.Assert(t, getAndReleaseStream(pool, ctx, "tcp", mockAddr0, opt) == nil)
		newV, ok := pool.conns.Load(mockAddr0)
		test.Assert(t, ok)
		test.Assert(t, newV.(*transports) != oldTrans)
	})
	t.Run("Get() after Close()", func(t *testing.T) {
		pool := newMockConnPool(nil)

		opt := newMockConnOption()
		ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
		test.Assert(t, getAndReleaseStream(pool, ctx, "tcp", mockAddr0, opt) == nil)

		_, ok := pool.conns.Load(mockAddr0)
		test.Assert(t, ok)

		test.Assert(t, pool.Close() == nil)

		// After Close, all entries should be cleaned up.
		_, ok = pool.conns.Load(mockAddr0)
		test.Assert(t, !ok)

		// Get after Close should not leak a new transports
		err := getAndReleaseStream(pool, ctx, "tcp", mockAddr0, opt)
		test.Assert(t, err != nil)
		test.Assert(t, err == errConnPoolClosed, err)
	})
	t.Run("onClose() triggered under sustained Get()", func(t *testing.T) {
		// Continuous connection churn (simulates flapping instances).
		// Transports are killed via onClose while Gets keep coming;
		// the pool must keep serving without unexpected error classes.
		pool := newMockConnPool(nil)
		t.Cleanup(func() { pool.Close() })
		opt := newMockConnOption()

		var getWG sync.WaitGroup
		stop := make(chan struct{})

		closerDone := make(chan struct{})
		go func() {
			defer close(closerDone)
			ticker := time.NewTicker(2 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					pool.conns.Range(func(_, v any) bool {
						for _, tr := range v.(*transports).loadAll() {
							if checkActive(tr) {
								tr.Close(grpc.ErrConnClosing)
								return false
							}
						}
						return true
					})
				}
			}
		}()

		// Get loop. Each iteration releases its stream quota.
		var successNum, failNum, unexpectedErrs atomic.Int32
		for i := 0; i < 20; i++ {
			getWG.Add(1)
			go func() {
				defer getWG.Done()
				ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional)
				for {
					select {
					case <-stop:
						return
					default:
						err := getAndReleaseStream(pool, ctx, "tcp", mockAddr0, opt)
						if err == nil {
							successNum.Add(1)
							continue
						}
						failNum.Add(1)
						if !isExpectedShutdownErr(err) {
							unexpectedErrs.Add(1)
							t.Error(err)
						}
					}
				}
			}()
		}

		time.Sleep(200 * time.Millisecond)
		close(stop)
		<-closerDone
		getWG.Wait()

		test.Assert(t, successNum.Load() > 0)
		test.Assert(t, unexpectedErrs.Load() == 0, unexpectedErrs.Load())
		t.Logf("success=%d fail=%d", successNum.Load(), failNum.Load())
	})
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
	shortCP := newMockConnPool(nil)
	shortCP.connOpts.ShortConn = true
	defer shortCP.Close()

	conn, err := shortCP.Get(newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional), "tcp", mockAddr0, remote.ConnOption{Dialer: newMockDialerWithDialFunc(dialFunc1)})
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
	longCP := newMockConnPool(nil)
	longCP.connOpts.ShortConn = false
	defer longCP.Close()

	longConn, err := longCP.Get(newMockCtxWithRPCInfo(serviceinfo.StreamingBidirectional), "tcp", mockAddr0, remote.ConnOption{Dialer: newMockDialerWithDialFunc(dialFunc2)})
	longConn.Close()
	test.Assert(t, err == nil, err)
	longCP.Put(longConn)
	test.Assert(t, atomic.LoadInt32(&closed2) == 0)
}

func Test_newTransport(t *testing.T) {
	t.Run("tls handshake failure closes dialed conn", func(t *testing.T) {
		var closed atomic.Int32
		dialFunc := func(network, address string, timeout time.Duration) (net.Conn, error) {
			return &mockConn{
				RemoteAddrFunc: func() net.Addr { return utils.NewNetAddr("tcp", address) },
				WriteFunc:      func(b []byte) (int, error) { return len(b), nil },
				ReadFunc:       func(b []byte) (int, error) { return 0, errors.New("not a TLS server") },
				CloseFunc:      func() error { closed.Add(1); return nil },
			}, nil
		}
		dialer := newMockDialerWithDialFunc(dialFunc)
		opts := grpc.ConnectOptions{
			TLSConfig:       &tls.Config{InsecureSkipVerify: true},
			WriteBufferSize: defaultMockReadWriteBufferSize,
			ReadBufferSize:  defaultMockReadWriteBufferSize,
		}

		tr, err := newTransport("test", dialer, "tcp", mockAddr0, time.Second, opts, nil, nil)
		test.Assert(t, err != nil)
		test.Assert(t, tr == nil)
		test.Assert(t, closed.Load() == 1, closed.Load())
	})
	t.Run("flush failed during init closes conn", func(t *testing.T) {
		var closed atomic.Int32
		var writeCount atomic.Int32
		closeCh := make(chan struct{})
		dialFunc := func(network, address string, timeout time.Duration) (net.Conn, error) {
			return &mockConn{
				RemoteAddrFunc: func() net.Addr { return utils.NewNetAddr("tcp", address) },
				ReadFunc: func(b []byte) (int, error) {
					<-closeCh // block until conn is closed
					return 0, errors.New("closed")
				},
				WriteFunc: func(b []byte) (int, error) {
					if writeCount.Add(1) >= 2 {
						return 0, errors.New("mock flush error")
					}
					return len(b), nil
				},
				CloseFunc: func() error {
					closed.Add(1)
					select {
					case <-closeCh:
					default:
						close(closeCh)
					}
					return nil
				},
			}, nil
		}
		dialer := newMockDialerWithDialFunc(dialFunc)

		_, err := newTransport("test", dialer, "tcp", mockAddr0, time.Second, slotTestOpts, nil, nil)
		test.Assert(t, err != nil)
		// newHTTP2Client calls t.Close(err) on Flush failure, which calls conn.Close().
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if closed.Load() >= 1 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		test.Assert(t, closed.Load() == 1, closed.Load())
	})
}
