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
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/pkg/warmup"
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
	return c.Conn.Close()
}

// RawConn returns the real underlying net.Conn.
func (c *longConn) RawConn() net.Conn {
	return c.Conn
}

// IsActive indicates whether the connection is active.
func (c *longConn) IsActive() bool {
	if conn, ok := c.Conn.(remote.IsActive); ok {
		if conn.IsActive() {
			return true
		}
	}
	return time.Now().Before(c.deadline)
}

func (c *longConn) Expired() bool {
	return time.Now().After(c.deadline)
}

func (c *longConn) Dump() interface{} {
	return c.deadline
}

// pool implements a pool of long connections.
type pool struct {
	idleList []*longConn
	mu       sync.RWMutex
	// config
	minIdle        int
	maxIdle        int           // currIdle <= maxIdle.
	maxIdleTimeout time.Duration // the idle connection will be cleaned if the idle time exceeds maxIdleTimeout.
}

func newPool(minIdle, maxIdle int, maxIdleTimeout time.Duration) *pool {
	p := &pool{
		idleList:       make([]*longConn, 0, maxIdle),
		minIdle:        minIdle,
		maxIdle:        maxIdle,
		maxIdleTimeout: maxIdleTimeout,
	}
	return p
}

// Get gets the first active connection from the idleList.
func (p *pool) Get() (*longConn, bool) {
	p.mu.Lock()
	// Get the first active one
	i := len(p.idleList) - 1
	for ; i >= 0; i-- {
		o := p.idleList[i]
		if o.IsActive() {
			p.idleList = p.idleList[:i]
			p.mu.Unlock()
			return o, true
		}
		// inactive object
		o.Close()
	}
	// in case all objects are inactive
	if i < 0 {
		i = 0
	}
	p.idleList = p.idleList[:i]
	p.mu.Unlock()
	return nil, false
}

// Put puts back a connection to the pool.
func (p *pool) Put(o *longConn) bool {
	p.mu.Lock()
	var recycled bool
	if len(p.idleList) < p.maxIdle {
		o.deadline = time.Now().Add(p.maxIdleTimeout)
		p.idleList = append(p.idleList, o)
		recycled = true
	}
	p.mu.Unlock()
	return recycled
}

// Evict cleanups the expired connections.
func (p *pool) Evict() int {
	p.mu.Lock()
	i := 0
	for ; i < len(p.idleList)-p.minIdle; i++ {
		if !p.idleList[i].Expired() && p.idleList[i].IsActive() {
			break
		}
		// close the inactive object
		p.idleList[i].Close()
	}
	p.idleList = p.idleList[i:]
	p.mu.Unlock()
	return i
}

// Len returns the length of the pool.
func (p *pool) Len() int {
	p.mu.RLock()
	l := len(p.idleList)
	p.mu.RUnlock()
	return l
}

// Close closes the pool and all the objects in the pool.
func (p *pool) Close() int {
	p.mu.Lock()
	num := len(p.idleList)
	for i := 0; i < num; i++ {
		p.idleList[i].Close()
	}
	p.idleList = nil

	p.mu.Unlock()
	return num
}

// peer has one address, it manages all connections base on this address
type peer struct {
	// info
	serviceName string
	addr        net.Addr
	globalIdle  *utils.MaxCounter
	// pool
	pool *pool
}

func newPeer(
	serviceName string,
	addr net.Addr,
	minIdle int,
	maxIdle int,
	maxIdleTimeout time.Duration,
	globalIdle *utils.MaxCounter,
) *peer {
	return &peer{
		serviceName: serviceName,
		addr:        addr,
		globalIdle:  globalIdle,
		pool:        newPool(minIdle, maxIdle, maxIdleTimeout),
	}
}

// Reset resets the peer to addr.
func (p *peer) Reset(addr net.Addr) {
	p.addr = addr
	p.Close()
}

func (p *peer) Get(d remote.Dialer, timeout time.Duration, reporter Reporter, addr string) (net.Conn, error) {
	var c net.Conn
	c, reused := p.pool.Get()
	if reused {
		p.globalIdle.Dec()
		return c, nil
	}
	// dial a new connection
	c, err := d.DialTimeout(p.addr.Network(), p.addr.String(), timeout)
	if err != nil {
		reporter.ConnFailed(Long, p.serviceName, p.addr)
		return nil, err
	}
	reporter.ConnSucceed(Long, p.serviceName, p.addr)
	return &longConn{
		Conn:    c,
		address: addr,
	}, nil
}

type PoolDump struct {
	IdleNum  int           `json:"idle_num"`
	IdleList []interface{} `json:"idle_list"`
}

// Dump dumps the info of all the objects in the pool.
func (p *pool) Dump() PoolDump {
	p.mu.RLock()
	idleNum := len(p.idleList)
	idleList := make([]interface{}, idleNum)
	for i := 0; i < idleNum; i++ {
		idleList[i] = p.idleList[i].Dump()
	}
	s := PoolDump{
		IdleNum:  idleNum,
		IdleList: idleList,
	}
	p.mu.RUnlock()
	return s
}

func (p *peer) Put(c *longConn) error {
	if !p.globalIdle.Inc() {
		return c.Close()
	}
	if !p.pool.Put(c) {
		p.globalIdle.Dec()
		return c.Close()
	}
	return nil
}

func (p *peer) Len() int {
	return p.pool.Len()
}

func (p *peer) Evict() {
	n := p.pool.Evict()
	for i := 0; i < n; i++ {
		p.globalIdle.Dec()
	}
}

// Close closes the peer and all the connections in the ring.
func (p *peer) Close() {
	n := p.pool.Close()
	for i := 0; i < n; i++ {
		p.globalIdle.Dec()
	}
}

// LongPool manages a pool of long connections.
type LongPool struct {
	reporter   Reporter
	peerMap    sync.Map
	newPeer    func(net.Addr) *peer
	globalIdle *utils.MaxCounter
	closeCh    chan struct{}
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
		p.(*peer).Put(c)
		return nil
	}
	return c.Conn.Close()
}

// Discard implements the ConnPool interface.
func (lp *LongPool) Discard(conn net.Conn) error {
	c, ok := conn.(*longConn)
	if ok {
		return c.Close()
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
		t := value.(*peer).pool.Dump()
		m[key.(netAddr).String()] = t
		return true
	})
	return m
}

// Close releases all peers in the pool, it is executed when client is closed.
func (lp *LongPool) Close() error {
	select {
	case <-lp.closeCh:
	default:
		close(lp.closeCh)
	}
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

// WarmUp implements the warmup.Pool interface.
func (lp *LongPool) WarmUp(eh warmup.ErrorHandling, wuo *warmup.PoolOption, co remote.ConnOption) error {
	h := &warmup.PoolHelper{ErrorHandling: eh}
	return h.WarmUp(wuo, lp, co)
}

// Evict cleanups the idle connections in peers.
func (lp *LongPool) Evict(frequency time.Duration) {
	t := time.NewTicker(frequency)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			lp.peerMap.Range(func(key, value interface{}) bool {
				p := value.(*peer)
				p.Evict()
				return true
			})
		case <-lp.closeCh:
			return
		}
	}
}

// NewLongPool creates a long pool using the given IdleConfig.
func NewLongPool(serviceName string, idlConfig connpool.IdleConfig) *LongPool {
	limit := utils.NewMaxCounter(idlConfig.MaxIdleGlobal)
	lp := &LongPool{
		reporter:   &DummyReporter{},
		globalIdle: limit,
		newPeer: func(addr net.Addr) *peer {
			return newPeer(
				serviceName,
				addr,
				idlConfig.MinIdlePerAddress,
				idlConfig.MaxIdlePerAddress,
				idlConfig.MaxIdleTimeout,
				limit)
		},
		closeCh: make(chan struct{}),
	}

	go lp.Evict(idlConfig.MaxIdleTimeout)
	return lp
}
