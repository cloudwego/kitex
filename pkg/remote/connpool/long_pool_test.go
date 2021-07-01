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

package connpool

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/connpool"
	dialer "github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
)

var (
	mockAddr0 = "127.0.0.1:8000"
	mockAddr1 = "127.0.0.1:8001"
)

// fakeNewLongPool creates a LongPool object and modifies it to fit tests.
func newLongPoolForTest(peerAddr, global int, timeout time.Duration) *LongPool {
	cfg := connpool.IdleConfig{
		MaxIdlePerAddress: peerAddr,
		MaxIdleGlobal:     global,
		MaxIdleTimeout:    timeout,
	}
	lp := NewLongPool("test", cfg)

	limit := utils.NewMaxCounter(global)

	lp.newPeer = func(addr net.Addr) *peer {
		return newPeer("test", addr, peerAddr, timeout, limit)
	}
	return lp
}

func TestLongConnPoolGetTimeout(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 3, time.Second)

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		connectCost := time.Millisecond * 10
		if timeout < connectCost {
			return nil, errors.New("connect timeout")
		}
		na := utils.NewNetAddr(network, address)
		return &mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
		}, nil
	}
	var err error

	_, err = p.Get(nil, "tcp", mockAddr0, &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second})
	test.Assert(t, err == nil)

	_, err = p.Get(nil, "tcp", mockAddr0, &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Millisecond})
	test.Assert(t, err != nil)
}

func TestLongConnPoolReuse(t *testing.T) {
	idleTime := time.Second
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 3, idleTime)

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		na := utils.NewNetAddr(network, address)
		return &mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
		}, nil
	}

	addr1, addr2 := mockAddr1, "127.0.0.1:8002"
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	count := make(map[net.Conn]int)
	for i := 0; i < 10; i++ {
		c, err := p.Get(nil, "tcp", addr1, opt)
		test.Assert(t, err == nil)
		count[c]++
	}
	test.Assert(t, len(count) == 10)

	count = make(map[net.Conn]int)
	for i := 0; i < 10; i++ {
		c, err := p.Get(nil, "tcp", addr1, opt)
		test.Assert(t, err == nil)
		err = p.Put(c)
		test.Assert(t, err == nil)
		count[c]++
	}
	test.Assert(t, len(count) == 1)

	count = make(map[net.Conn]int)
	for i := 0; i < 10; i++ {
		c, err := p.Get(nil, "tcp", addr1, opt)
		test.Assert(t, err == nil)
		err = p.Put(c)
		test.Assert(t, err == nil)
		count[c]++

		c, err = p.Get(nil, "tcp", addr2, opt)
		test.Assert(t, err == nil)
		err = p.Put(c)
		test.Assert(t, err == nil)
		count[c]++
	}
	test.Assert(t, len(count) == 2)

	// test exceed idleTime
	count = make(map[net.Conn]int)
	for i := 0; i < 3; i++ {
		c, err := p.Get(nil, "tcp", addr1, opt)
		test.Assert(t, err == nil)
		err = p.Put(c)
		test.Assert(t, err == nil)
		count[c]++
		time.Sleep(idleTime + 2)
	}
	test.Assert(t, len(count) == 3)
}

func TestLongConnPoolMaxIdle(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 5, time.Second)

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		na := utils.NewNetAddr(network, address)
		return &mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
		}, nil
	}

	addr := mockAddr1
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	var conns []net.Conn
	count := make(map[net.Conn]int)
	for i := 0; i < 10; i++ {
		c, err := p.Get(nil, "tcp", addr, opt)
		test.Assert(t, err == nil)
		count[c]++
		conns = append(conns, c)
	}
	test.Assert(t, len(count) == 10)

	for _, c := range conns {
		err := p.Put(c)
		test.Assert(t, err == nil)
	}

	for i := 0; i < 10; i++ {
		c, err := p.Get(nil, "tcp", addr, opt)
		test.Assert(t, err == nil)
		count[c]++
	}
	test.Assert(t, len(count) == 18)
}

func TestLongConnPoolGlobalMaxIdle(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 3, time.Second)

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		na := utils.NewNetAddr(network, address)
		return &mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
		}, nil
	}

	addr1, addr2 := mockAddr1, "127.0.0.1:8002"
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	var conns []net.Conn
	count := make(map[net.Conn]int)
	for i := 0; i < 10; i++ {
		c, err := p.Get(nil, "tcp", addr1, opt)
		test.Assert(t, err == nil)
		count[c]++
		conns = append(conns, c)

		c, err = p.Get(nil, "tcp", addr2, opt)
		test.Assert(t, err == nil)
		count[c]++
		conns = append(conns, c)
	}
	test.Assert(t, len(count) == 20)

	for _, c := range conns {
		err := p.Put(c)
		test.Assert(t, err == nil)
	}

	for i := 0; i < 10; i++ {
		c, err := p.Get(nil, "tcp", addr1, opt)
		test.Assert(t, err == nil)
		count[c]++

		c, err = p.Get(nil, "tcp", addr2, opt)
		test.Assert(t, err == nil)
		count[c]++
	}
	test.Assert(t, len(count) == 37)
}

func TestLongConnPoolCloseOnDiscard(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 5, time.Second)

	var closed bool
	closer := func() error {
		if closed {
			return errors.New("connection already closed")
		}
		closed = true
		return nil
	}

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		na := utils.NewNetAddr(network, address)
		return &mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
			CloseFunc:      closer,
		}, nil

	}

	addr := mockAddr1
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	c, err := p.Get(nil, "tcp", addr, opt)
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	err = c.Close()
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	err = p.Put(c)
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	c2, err := p.Get(nil, "tcp", addr, opt)
	test.Assert(t, err == nil)
	test.Assert(t, c == c2)
	test.Assert(t, !closed)

	err = p.Discard(c2)
	test.Assert(t, err == nil)
	test.Assert(t, closed)

	c3, err := p.Get(nil, "tcp", addr, opt)
	test.Assert(t, err == nil)
	test.Assert(t, c3 != c2)
	test.Assert(t, closed)
}

func TestLongConnPoolCloseOnError(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 5, time.Second)

	var closed, read bool
	closer := func() error {
		if closed {
			return errors.New("connection already closed")
		}
		closed = true
		return nil
	}
	reader := func(b []byte) (n int, err error) {
		// Error on second time reading
		if read {
			return 0, errors.New("read timeout")
		}
		read = true
		return 0, nil
	}

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		na := utils.NewNetAddr(network, address)
		return &mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
			CloseFunc:      closer,
			ReadFunc:       reader,
		}, nil
	}

	addr := mockAddr1
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	c, err := p.Get(nil, "tcp", addr, opt)
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	var buf []byte
	_, err = c.Read(buf)
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	err = p.Put(c)
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	c2, err := p.Get(nil, "tcp", addr, opt)
	test.Assert(t, err == nil)
	test.Assert(t, c == c2)
	test.Assert(t, !closed)

	_, err = c.Read(buf)
	test.Assert(t, err != nil)
	test.Assert(t, !closed)

	c3, err := p.Get(nil, "tcp", addr, opt)
	test.Assert(t, err == nil)
	test.Assert(t, c2 != c3)
}

func TestLongConnPoolCloseOnIdleTimeout(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 5, time.Millisecond)

	var closed bool
	closer := func() error {
		if closed {
			return errors.New("connection already closed")
		}
		closed = true
		return nil
	}
	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		na := utils.NewNetAddr(network, address)
		return &mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
			CloseFunc:      closer}, nil
	}

	addr := mockAddr1
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	c, err := p.Get(nil, "tcp", addr, opt)
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	err = p.Put(c)
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	time.Sleep(time.Millisecond * 2)

	c2, err := p.Get(nil, "tcp", addr, opt)
	test.Assert(t, err == nil)
	test.Assert(t, c != c2)
	test.Assert(t, closed)
}

func TestLongConnPoolCloseOnClean(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 5, time.Second)

	var closed bool
	var mu sync.RWMutex // fix data race in test
	closer := func() error {
		mu.RLock()
		if closed {
			mu.RUnlock()
			return errors.New("connection already closed")
		}
		mu.RUnlock()
		mu.Lock()
		closed = true
		mu.Unlock()
		return nil
	}
	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		na := utils.NewNetAddr(network, address)
		return &mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
			CloseFunc:      closer}, nil
	}

	addr := mockAddr1
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	c, err := p.Get(nil, "tcp", addr, opt)
	test.Assert(t, err == nil)
	mu.RLock()
	test.Assert(t, !closed)
	mu.RUnlock()

	err = p.Put(c)
	test.Assert(t, err == nil)
	mu.RLock()
	test.Assert(t, !closed)
	mu.RUnlock()

	p.Clean("tcp", addr)
	time.Sleep(time.Millisecond) // Wait for the clean goroutine to finish
	mu.RLock()
	test.Assert(t, closed)
	mu.RUnlock()
}

func TestLongConnPoolDiscardUnknownConnection(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 5, time.Second)

	var closed bool
	closer := func() error {
		if closed {
			return errors.New("connection already closed")
		}
		closed = true
		return nil
	}
	network, address := "tcp", mockAddr1
	na := utils.NewNetAddr(network, address)
	mc := &mocks.Conn{
		RemoteAddrFunc: func() net.Addr { return na },
		CloseFunc:      closer}

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return mc, nil
	}

	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}
	c, err := p.Get(nil, network, address, opt)
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	err = c.Close()
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	lc, ok := c.(*longConn)
	test.Assert(t, ok)
	test.Assert(t, lc.Conn == mc)

	err = p.Discard(lc.Conn)
	test.Assert(t, err == nil)
	test.Assert(t, closed)
}

func TestConnPoolClose(t *testing.T) {
	p := newLongPoolForTest(2, 3, time.Second)
	d := &dialer.SynthesizedDialer{}

	var closed int
	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return &mocks.Conn{
			CloseFunc: func() (e error) {
				closed++
				return nil
			},
			RemoteAddrFunc: func() (r net.Addr) {
				return utils.NewNetAddr(network, address)
			},
		}, nil
	}

	network, address := "tcp", mockAddr0
	conn, err := p.Get(nil, network, address, &dialer.ConnOption{Dialer: d})
	test.Assert(t, err == nil)
	p.Put(conn)

	network, address = "tcp", mockAddr1
	_, err = p.Get(nil, network, address, &dialer.ConnOption{Dialer: d})
	p.Put(conn)

	connCount := 0
	p.peerMap.Range(func(key, value interface{}) bool {
		connCount++
		return true
	})
	test.Assert(t, connCount == 2)

	p.Close()
	test.Assert(t, closed == 2, closed)

	connCount = 0
	p.peerMap.Range(func(key, value interface{}) bool {
		connCount++
		return true
	})
	test.Assert(t, connCount == 0)
}

func TestLongConnPoolPutUnknownConnection(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := newLongPoolForTest(2, 5, time.Second)

	var closed bool
	closer := func() error {
		if closed {
			return errors.New("connection already closed")
		}
		closed = true
		return nil
	}
	network, address := "tcp", mockAddr1
	na := utils.NewNetAddr(network, address)
	mc := &mocks.Conn{
		RemoteAddrFunc: func() net.Addr { return na },
		CloseFunc:      closer}

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return mc, nil
	}

	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}
	c, err := p.Get(nil, network, address, opt)
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	err = c.Close()
	test.Assert(t, err == nil)
	test.Assert(t, !closed)

	lc, ok := c.(*longConn)
	test.Assert(t, ok)
	test.Assert(t, lc.Conn == mc)

	err = p.Put(lc.Conn)
	test.Assert(t, err == nil)
	test.Assert(t, closed)
}

func BenchmarkLongPoolGetOne(b *testing.B) {
	d := &dialer.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			na := utils.NewNetAddr(network, address)
			return &mocks.Conn{
				RemoteAddrFunc: func() net.Addr { return na },
			}, nil
		},
	}
	p := newLongPoolForTest(100000, 1<<20, time.Second)
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Get(nil, "tcp", mockAddr1, opt)
		}
	})
}

func BenchmarkLongPoolGetRand2000(b *testing.B) {
	d := &dialer.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			na := utils.NewNetAddr(network, address)
			return &mocks.Conn{
				RemoteAddrFunc: func() net.Addr { return na },
			}, nil
		},
	}
	p := newLongPoolForTest(50, 50, time.Second)
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	var addrs []string
	for i := 0; i < 2000; i++ {
		addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", 8000+rand.Intn(10000)))
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Get(nil, "tcp", addrs[rand.Intn(2000)], opt)
		}
	})
}

func BenchmarkLongPoolGetRand2000Mesh(b *testing.B) {
	d := &dialer.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			na := utils.NewNetAddr(network, address)
			return &mocks.Conn{
				RemoteAddrFunc: func() net.Addr { return na },
			}, nil
		},
	}
	p := newLongPoolForTest(100000, 1<<20, time.Second)
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	var addrs []string
	for i := 0; i < 2000; i++ {
		addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", 8000+rand.Intn(10000)))
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Get(nil, "tcp", addrs[rand.Intn(2000)], opt)
		}
	})
}
