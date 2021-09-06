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

package netpollmux

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	mid_netpoll "github.com/cloudwego/netpoll"

	"github.com/jackedelic/kitex/internal/mocks"
	dialer "github.com/jackedelic/kitex/internal/pkg/remote"
	"github.com/jackedelic/kitex/internal/pkg/utils"
	"github.com/jackedelic/kitex/internal/test"
)

var (
	mockAddr0 = "127.0.0.1:8000"
	mockAddr1 = "127.0.0.1:8001"
)

func TestMuxConnPoolGetTimeout(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := NewMuxConnPool(1)

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		connectCost := time.Millisecond * 10
		if timeout < connectCost {
			return nil, errors.New("connect timeout")
		}
		na := utils.NewNetAddr(network, address)
		return &MockNetpollConn{
			Conn: mocks.Conn{
				RemoteAddrFunc: func() net.Addr { return na },
			},
			WriterFunc: func() (r mid_netpoll.Writer) { return &MockNetpollWriter{} },
			ReaderFunc: func() (r mid_netpoll.Reader) { return &MockNetpollReader{} },
		}, nil
	}
	var err error

	_, err = p.Get(context.TODO(), "tcp", mockAddr0, &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second})
	test.Assert(t, err == nil)

	_, err = p.Get(context.TODO(), "tcp", mockAddr0, &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Millisecond})
	test.Assert(t, err != nil)
}

func TestMuxConnPoolReuse(t *testing.T) {
	d := &dialer.SynthesizedDialer{}
	p := NewMuxConnPool(1)

	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		na := utils.NewNetAddr(network, address)
		return &MockNetpollConn{
			Conn: mocks.Conn{
				RemoteAddrFunc: func() net.Addr { return na },
			},
			WriterFunc: func() (r mid_netpoll.Writer) { return &MockNetpollWriter{} },
			ReaderFunc: func() (r mid_netpoll.Reader) { return &MockNetpollReader{} },
			IsActiveFunc: func() (r bool) {
				return true
			},
		}, nil
	}

	addr1, addr2 := mockAddr1, "127.0.0.1:8002"
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	count := make(map[net.Conn]int)
	for i := 0; i < 10; i++ {
		c, err := p.Get(context.TODO(), "tcp", addr1, opt)
		test.Assert(t, err == nil)
		count[c]++
	}
	test.Assert(t, len(count) == 1)

	count = make(map[net.Conn]int)
	for i := 0; i < 10; i++ {
		c, err := p.Get(context.TODO(), "tcp", addr1, opt)
		test.Assert(t, err == nil)
		err = p.Put(c)
		test.Assert(t, err == nil)
		count[c]++
	}
	test.Assert(t, len(count) == 1)

	count = make(map[net.Conn]int)
	for i := 0; i < 10; i++ {
		c, err := p.Get(context.TODO(), "tcp", addr1, opt)
		test.Assert(t, err == nil)
		err = p.Put(c)
		test.Assert(t, err == nil)
		count[c]++

		c, err = p.Get(context.TODO(), "tcp", addr2, opt)
		test.Assert(t, err == nil)
		err = p.Put(c)
		test.Assert(t, err == nil)
		count[c]++
	}
	test.Assert(t, len(count) == 2)
}

func TestMuxConnPoolDiscardClean(t *testing.T) {
	size := 4
	d := &dialer.SynthesizedDialer{}
	p := NewMuxConnPool(size)

	var closed int
	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return &MockNetpollConn{
			Conn: mocks.Conn{
				CloseFunc: func() (e error) {
					closed++
					return nil
				},
			},
			IsActiveFunc: func() (r bool) {
				return true
			},
		}, nil
	}

	network, address := "tcp", mockAddr0
	var conns []net.Conn
	for i := 0; i < 1024; i++ {
		conn, err := p.Get(context.TODO(), network, address, &dialer.ConnOption{Dialer: d})
		test.Assert(t, err == nil)
		conns = append(conns, conn)
	}
	for i := 0; i < 128; i++ {
		p.Discard(conns[i])
		test.Assert(t, closed == 0, closed)
	}
	p.Clean(network, address)
	test.Assert(t, closed == size, closed, size)
}

func TestMuxConnPoolClose(t *testing.T) {
	p := NewMuxConnPool(1)
	d := &dialer.SynthesizedDialer{}

	var closed int
	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return &MockNetpollConn{
			Conn: mocks.Conn{
				CloseFunc: func() (e error) {
					closed++
					return nil
				},
			},
			IsActiveFunc: func() (r bool) {
				return true
			},
		}, nil
	}

	network, address := "tcp", mockAddr0
	_, err := p.Get(context.TODO(), network, address, &dialer.ConnOption{Dialer: d})
	test.Assert(t, err == nil)

	network, address = "tcp", mockAddr1
	_, err = p.Get(context.TODO(), network, address, &dialer.ConnOption{Dialer: d})
	test.Assert(t, err == nil)

	connCount := 0
	p.connMap.Range(func(key, value interface{}) bool {
		connCount++
		return true
	})
	test.Assert(t, connCount == 2)

	p.Close()
	test.Assert(t, closed == 2, closed)

	connCount = 0
	p.connMap.Range(func(key, value interface{}) bool {
		connCount++
		return true
	})
	test.Assert(t, connCount == 0)
}

func BenchmarkMuxPoolGetOne(b *testing.B) {
	d := &dialer.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			na := utils.NewNetAddr(network, address)
			return &MockNetpollConn{
				Conn: mocks.Conn{
					RemoteAddrFunc: func() net.Addr { return na },
				},
				WriterFunc: func() (r mid_netpoll.Writer) { return &MockNetpollWriter{} },
				ReaderFunc: func() (r mid_netpoll.Reader) { return &MockNetpollReader{} },
			}, nil
		},
	}
	p := NewMuxConnPool(1)
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Get(context.TODO(), "tcp", mockAddr1, opt)
		}
	})
}

func BenchmarkMuxPoolGetRand2000(b *testing.B) {
	d := &dialer.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			na := utils.NewNetAddr(network, address)
			return &MockNetpollConn{
				Conn: mocks.Conn{
					RemoteAddrFunc: func() net.Addr { return na },
				},
				WriterFunc: func() (r mid_netpoll.Writer) { return &MockNetpollWriter{} },
				ReaderFunc: func() (r mid_netpoll.Reader) { return &MockNetpollReader{} },
			}, nil
		},
	}
	p := NewMuxConnPool(1)
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	var addrs []string
	for i := 0; i < 2000; i++ {
		addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", 8000+rand.Intn(10000)))
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Get(context.TODO(), "tcp", addrs[rand.Intn(2000)], opt)
		}
	})
}

func BenchmarkMuxPoolGetRand2000Mesh(b *testing.B) {
	d := &dialer.SynthesizedDialer{
		DialFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			na := utils.NewNetAddr(network, address)
			return &MockNetpollConn{
				Conn: mocks.Conn{
					RemoteAddrFunc: func() net.Addr { return na },
				},
				WriterFunc: func() (r mid_netpoll.Writer) { return &MockNetpollWriter{} },
				ReaderFunc: func() (r mid_netpoll.Reader) { return &MockNetpollReader{} },
			}, nil
		},
	}
	p := NewMuxConnPool(1)
	opt := &dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second}

	var addrs []string
	for i := 0; i < 2000; i++ {
		addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", 8000+rand.Intn(10000)))
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Get(context.TODO(), "tcp", addrs[rand.Intn(2000)], opt)
		}
	})
}
