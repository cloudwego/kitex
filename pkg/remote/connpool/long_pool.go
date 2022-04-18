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

// Package connpool provide short connection and long connection pool.
package connpool

import (
	"context"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
)

var (
	_ net.Conn            = &longConn{}
	_ remote.LongConnPool = &LongPool{}
)

// netAddr implements the net.Addr interface and comparability.
type netAddr struct {
	network string
	address string
}

// Network implements the net.Addr interface.
func (na netAddr) Network() string { return na.network }

// String implements the net.Addr interface.
func (na netAddr) String() string { return na.address }

// longConn implements the net.Conn interface.
type longConn struct {
	net.Conn
	sync.RWMutex
	deadline time.Time
	address  string
}

// Close implements the net.Conn interface.
func (c *longConn) Close() error {
	return nil
}

// RawConn returns the real underlying net.Conn.
func (c *longConn) RawConn() net.Conn {
	return c.Conn
}

// IsActive indicates whether the connection is active.
func (c *longConn) IsActive() bool {
	if conn, ok := c.Conn.(remote.IsActive); ok {
		if !conn.IsActive() {
			return false
		}
	}
	return time.Now().Before(c.deadline)
}

// Peer has one address, it manage all connections base on this address
type peer struct {
	serviceName    string
	addr           net.Addr
	ring           *utils.Ring
	globalIdle     *utils.MaxCounter
	maxIdleTimeout time.Duration
}

func newPeer(
	serviceName string,
	addr net.Addr,
	maxIdle int,
	maxIdleTimeout time.Duration,
	globalIdle *utils.MaxCounter,
) *peer {
	return &peer{
		serviceName:    serviceName,
		addr:           addr,
		ring:           utils.NewRing(maxIdle),
		globalIdle:     globalIdle,
		maxIdleTimeout: maxIdleTimeout,
	}
}

// Reset resets the peer to addr.
func (p *peer) Reset(addr net.Addr) {
	p.addr = addr
	p.Close()
}

// Get picks up connection from ring or dial a new one.
func (p *peer) Get(d remote.Dialer, timeout time.Duration, reporter Reporter, addr string) (net.Conn, error) {
	for {
		conn, _ := p.ring.Pop().(*longConn)
		if conn == nil {
			break
		}
		p.globalIdle.Dec()
		if conn.IsActive() {
			reporter.ReuseSucceed(Long, p.serviceName, p.addr)
			return conn, nil
		}
		_ = conn.Conn.Close()
	}
	conn, err := d.DialTimeout(p.addr.Network(), p.addr.String(), timeout)
	if err != nil {
		reporter.ConnFailed(Long, p.serviceName, p.addr)
		return nil, err
	}
	reporter.ConnSucceed(Long, p.serviceName, p.addr)
	return &longConn{
		Conn:     conn,
		deadline: time.Now().Add(p.maxIdleTimeout),
		address:  addr,
	}, nil
}

func (p *peer) put(c *longConn) error {
	if !p.globalIdle.Inc() {
		return c.Conn.Close()
	}
	c.deadline = time.Now().Add(p.maxIdleTimeout)
	err := p.ring.Push(c)
	if err != nil {
		p.globalIdle.Dec()
		return c.Conn.Close()
	}
	return nil
}

// Close closes the peer and all the connections in the ring.
func (p *peer) Close() {
	for {
		conn, _ := p.ring.Pop().(*longConn)
		if conn == nil {
			break
		}
		p.globalIdle.Dec()
		_ = conn.Conn.Close()
	}
}

// LongPool manages a pool of long connections.
type LongPool struct {
	reporter Reporter
	peerMap  sync.Map
	newPeer  func(net.Addr) *peer
}

func (lp *LongPool) getPeer(addr netAddr) *peer {
	p, ok := lp.peerMap.Load(addr)
	if ok {
		return p.(*peer)
	}
	p, _ = lp.peerMap.LoadOrStore(addr, lp.newPeer(addr))
	return p.(*peer)
}

// Get pick or generate a net.Conn and return
// The context is not used but leave it for now.
func (lp *LongPool) Get(ctx context.Context, network, address string, opt remote.ConnOption) (net.Conn, error) {
	addr := netAddr{network, address}
	p := lp.getPeer(addr)
	return p.Get(opt.Dialer, opt.ConnectTimeout, lp.reporter, address)
}

// Put implements the ConnPool interface.
func (lp *LongPool) Put(conn net.Conn) error {
	c, ok := conn.(*longConn)
	if !ok {
		return conn.Close()
	}

	addr := conn.RemoteAddr()
	na := netAddr{addr.Network(), c.address}
	p, ok := lp.peerMap.Load(na)
	if ok {
		p.(*peer).put(c)
		return nil
	}
	return c.Conn.Close()
}

// Discard implements the ConnPool interface.
func (lp *LongPool) Discard(conn net.Conn) error {
	c, ok := conn.(*longConn)
	if ok {
		return c.Conn.Close()
	}
	return conn.Close()
}

// Clean implements the LongConnPool interface.
func (lp *LongPool) Clean(network, address string) {
	na := netAddr{network, address}
	if p, ok := lp.peerMap.Load(na); ok {
		lp.peerMap.Delete(na)
		go p.(*peer).Close()
	}
}

// Dump is used to dump current long pool info when needed, like debug query.
func (lp *LongPool) Dump() interface{} {
	m := make(map[string]interface{})
	lp.peerMap.Range(func(key, value interface{}) bool {
		t := value.(*peer).ring.Dump()
		arr := reflect.ValueOf(t).FieldByName("Array").Interface().([]interface{})
		for i := range arr {
			arr[i] = arr[i].(*longConn).deadline
		}
		m[key.(netAddr).String()] = t
		return true
	})
	return m
}

// Close releases all peers in the pool, it is executed when client is closed.
func (lp *LongPool) Close() error {
	lp.peerMap.Range(func(addr, value interface{}) bool {
		lp.peerMap.Delete(addr)
		v := value.(*peer)
		v.Close()
		return true
	})
	return nil
}

// EnableReporter enable reporter for long connection pool.
func (lp *LongPool) EnableReporter() {
	lp.reporter = GetCommonReporter()
}

// NewLongPool creates a long pool using the given IdleConfig.
func NewLongPool(serviceName string, idlConfig connpool.IdleConfig) *LongPool {
	limit := utils.NewMaxCounter(idlConfig.MaxIdleGlobal)
	return &LongPool{
		reporter: &DummyReporter{},
		newPeer: func(addr net.Addr) *peer {
			return newPeer(
				serviceName,
				addr,
				idlConfig.MaxIdlePerAddress,
				idlConfig.MaxIdleTimeout,
				limit)
		},
	}
}
