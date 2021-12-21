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

// connMeta stores the address and deadline of the connection
type connMeta struct {
	addr     netAddr
	deadline time.Time
}

func (p *connMeta) Less(i interface{}) bool {
	return p.deadline.Before(i.(*connMeta).deadline)
}

// watcher is responsible for the stale connection cleaning
type watcher struct {
	connMetaQueue *utils.PriorityQueue
	checkInterval time.Duration
	closeCh       chan struct{}
	startCh       chan struct{}
}

func (w *watcher) Close() {
	close(w.closeCh)
	close(w.startCh)
	w.connMetaQueue.Close()
}

// Pop gets the top of the queue and remove it
func (w *watcher) Pop() utils.PQItem {
	return w.connMetaQueue.Pop()
}

// Top gets the top of the queue, which is the connection with the earliest deadline
func (w *watcher) Top() utils.PQItem {
	return w.connMetaQueue.Top()
}

// Push pushes one connMeta into the queue of the watcher
func (w *watcher) Push(i utils.PQItem) {
	// start the watcher when the queue is not empty
	if w.connMetaQueue.Len() == 0 {
		w.startCh <- struct{}{}
	}
	w.connMetaQueue.Push(i)
}

func newWatcher(checkInterval time.Duration) *watcher {
	w := &watcher{
		checkInterval: checkInterval,
		connMetaQueue: utils.NewPriorityQueue(),
		startCh:       make(chan struct{}),
		closeCh:       make(chan struct{}),
	}
	return w
}

// LongPool manages a pool of long connections.
type LongPool struct {
	reporter Reporter
	peerMap  sync.Map
	newPeer  func(net.Addr) *peer
	watcher  *watcher
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
		// put the connMeta when the connection is put back
		lp.watcher.Push(
			&connMeta{
				addr:     na,
				deadline: c.deadline,
			})
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
	lp.watcher.Close()
	return nil
}

// EnableReporter enable reporter for long connection pool.
func (lp *LongPool) EnableReporter() {
	lp.reporter = getCommonReporter()
}

// NewLongPool creates a long pool using the given IdleConfig.
func NewLongPool(serviceName string, idlConfig connpool.IdleConfig) *LongPool {
	limit := utils.NewMaxCounter(idlConfig.MaxIdleGlobal)
	lp := &LongPool{
		reporter: &DummyReporter{},
		newPeer: func(addr net.Addr) *peer {
			return newPeer(
				serviceName,
				addr,
				idlConfig.MaxIdlePerAddress,
				idlConfig.MaxIdleTimeout,
				limit)
		},
		// TODO: how to set the check interval
		watcher: newWatcher(idlConfig.MaxIdleTimeout / 2),
	}
	go lp.watch()
	return lp
}

// watch starts the watcher
func (lp *LongPool) watch() {
	ticker := time.NewTicker(lp.watcher.checkInterval)
	for {
		select {
		case <-lp.watcher.startCh:
			lp.invokeWatcher(ticker)
		case <-lp.watcher.closeCh:
			return
		case <-ticker.C:
			lp.cleanStaleConn(ticker)
		}
	}
}

// invokeWatcher invokes the watcher to reset the timer for next clean
func (lp *LongPool) invokeWatcher(t *time.Ticker) {
	top, _ := lp.watcher.Top().(*connMeta)
	if top == nil {
		return
	}
	t.Reset(getTickInterval(top.deadline.Sub(time.Now()), lp.watcher.checkInterval))
	return
}

// cleanStaleConn cleans the stale conn and reset the timer
func (lp *LongPool) cleanStaleConn(t *time.Ticker) {
	pm, _ := lp.watcher.Pop().(*connMeta)
	if pm == nil {
		t.Stop()
		return
	}
	// get the peer of this addr and loop to clean the stale conn
	p := lp.getPeer(pm.addr)
	conn, _ := p.ring.Top().(*longConn)
	if conn != nil && !conn.IsActive() {
		_ = conn.Conn.Close()
		_ = p.ring.Pop()
	}
	// set next timer
	top, _ := lp.watcher.Top().(*connMeta)
	if top != nil {
		t.Reset(getTickInterval(top.deadline.Sub(time.Now()), lp.watcher.checkInterval))
	} else {
		t.Stop()
	}
}

func getTickInterval(beforeStale, defaultInterval time.Duration) time.Duration {
	if beforeStale > 0 {
		return beforeStale
	} else {
		// TODO: determine the next check time
		//return defaultInterval
		return 0
	}
}
