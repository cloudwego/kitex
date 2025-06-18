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
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/pkg/warmup"
)

var (
	_ net.Conn            = &longConn{}
	_ remote.LongConnPool = &LongPool{}

	// global shared tickers for different LongPool
	sharedTickers sync.Map
)

const (
	DefaultProactiveConnCheckInterval = 3 * time.Second
	idleConfigDumpKey                 = "idle_config"
)

func getSharedTicker(p *LongPool, refreshInterval time.Duration) *utils.SharedTicker {
	sti, ok := sharedTickers.Load(refreshInterval)
	if ok {
		st := sti.(*utils.SharedTicker)
		st.Add(p)
		return st
	}
	sti, _ = sharedTickers.LoadOrStore(refreshInterval, utils.NewSharedTicker(refreshInterval))
	st := sti.(*utils.SharedTicker)
	st.Add(p)
	return st
}

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
	pooledAt time.Time
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
		return conn.IsActive()
	} else {
		return time.Now().Before(c.deadline)
	}
}

// Expired checks the deadline of the connection.
func (c *longConn) Expired() bool {
	return time.Now().After(c.deadline)
}

type PoolDump struct {
	IdleNum       int         `json:"idle_num"`
	ConnsDeadline []time.Time `json:"conns_deadline"`
}

func newPool(config connpool.IdleConfig, proactiveCheckCfg ProactiveCheckConfig) *pool {
	p := &pool{
		idleList:             make([]*longConn, 0, config.MaxIdlePerAddress),
		minIdle:              config.MinIdlePerAddress,
		maxIdle:              config.MaxIdlePerAddress,
		maxIdleTimeout:       config.MaxIdleTimeout,
		ProactiveCheckConfig: proactiveCheckCfg,
	}
	if p.ProactiveCheckConfig.Enable {
		if p.maxIdleTimeout < p.ProactiveCheckConfig.Interval {
			p.ProactiveCheckConfig.Interval = p.maxIdleTimeout
		}
	}
	return p
}

// pool implements a pool of long connections.
type pool struct {
	idleList []*longConn // idleList Get/Put by FILO(stack) but Evict by FIFO(queue)
	mu       sync.RWMutex
	// config
	minIdle        int
	maxIdle        int           // currIdle <= maxIdle.
	maxIdleTimeout time.Duration // the idle connection will be cleaned if the idle time exceeds maxIdleTimeout.
	ProactiveCheckConfig
}

// Get gets the first active connection from the idleList. Return the number of connections decreased during the Get.
func (p *pool) Get() (*longConn, bool, int) {
	p.mu.Lock()
	// Get the first active one
	n := len(p.idleList)
	selected := n - 1
	for ; selected >= 0; selected-- {
		o := p.idleList[selected]
		// reset slice element to nil, active conn object only could be hold reference by user function
		p.idleList[selected] = nil
		if o.IsActive() {
			p.idleList = p.idleList[:selected]
			p.mu.Unlock()
			return o, true, n - selected
		}
		// inactive object
		o.Close()
	}
	// in case all objects are inactive
	if selected < 0 {
		selected = 0
	}
	p.idleList = p.idleList[:selected]
	p.mu.Unlock()
	return nil, false, n - selected
}

// Put puts back a connection to the pool.
func (p *pool) Put(o *longConn) bool {
	p.mu.Lock()
	var recycled bool
	if len(p.idleList) < p.maxIdle {
		now := time.Now()
		o.deadline = now.Add(p.maxIdleTimeout)
		o.pooledAt = now
		p.idleList = append(p.idleList, o)
		recycled = true
	}
	p.mu.Unlock()
	return recycled
}

// Evict cleanups the expired connections.
// Evict returns how many connections has been evicted.
func (p *pool) Evict() (evicted int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ProactiveCheckConfig.Enable {
		// connection state check, this will set the state to the closed connections
		if err := p.checkConnState(); err != nil {
			klog.Errorf("KITEX: connpool health check failed: %v", err)
		}
	}

	nonIdle := len(p.idleList) - p.minIdle
	// clear non idle connections
	for ; evicted < nonIdle; evicted++ {
		if !p.idleList[evicted].Expired() {
			break
		}
		// close the inactive object
		p.idleList[evicted].Close()
		// reset slice element to nil, otherwise it will cause the connections will never be destroyed
		// TODO: use clear(unused_slice) when we no need to care about < go1.18
		p.idleList[evicted] = nil
	}
	p.idleList = p.idleList[evicted:]
	return evicted
}

// checkConnState checks and sets the state of connections that have been idle for more than connCheckInterval.
func (p *pool) checkConnState() error {
	if connCheckFunc := p.ProactiveCheckConfig.CheckFunc; connCheckFunc != nil {
		var toCheck []net.Conn
		now := time.Now()
		for _, conn := range p.idleList {
			if !now.After(conn.pooledAt.Add(p.ProactiveCheckConfig.Interval)) {
				// only check if now >= pooledAt+interval
				break
			}
			toCheck = append(toCheck, conn.RawConn())
		}
		return connCheckFunc(toCheck...)
	}
	return nil
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

// Dump dumps the info of all the objects in the pool.
func (p *pool) Dump() PoolDump {
	p.mu.RLock()
	idleNum := len(p.idleList)
	connsDeadline := make([]time.Time, idleNum)
	for i := 0; i < idleNum; i++ {
		connsDeadline[i] = p.idleList[i].deadline
	}
	s := PoolDump{
		IdleNum:       idleNum,
		ConnsDeadline: connsDeadline,
	}
	p.mu.RUnlock()
	return s
}

func newPeer(
	serviceName string,
	addr net.Addr,
	idleCfg connpool.IdleConfig,
	proactiveCheckCfg ProactiveCheckConfig,
	globalIdle *utils.MaxCounter,
) *peer {
	return &peer{
		serviceName: serviceName,
		addr:        addr,
		globalIdle:  globalIdle,
		pool:        newPool(idleCfg, proactiveCheckCfg),
	}
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

// Get gets a connection with dialer and timeout. Dial a new connection if no idle connection in pool is available.
func (p *peer) Get(d remote.Dialer, timeout time.Duration, reporter Reporter, addr string) (net.Conn, error) {
	var c net.Conn
	c, reused, decNum := p.pool.Get()
	p.globalIdle.DecN(int64(decNum))
	if reused {
		reporter.ReuseSucceed(Long, p.serviceName, p.addr)
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

// Put puts a connection back to the peer.
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
	p.globalIdle.DecN(int64(n))
}

// Close closes the peer and all the connections in the ring.
func (p *peer) Close() {
	n := p.pool.Close()
	p.globalIdle.DecN(int64(n))
}

type LongPoolConfig struct {
	ServiceName string
	connpool.IdleConfig
	ProactiveCheckConfig
}

// ProactiveCheckConfig is the config of proactive connection detection logic.
// only for go net now to avoid using closed connections.
type ProactiveCheckConfig struct {
	Enable    bool // if true, the connection pool will check the aliveness of conn.
	Interval  time.Duration
	CheckFunc func(conn ...net.Conn) error // CheckFunc is used to detect the connection state
}

// NewLongPoolWithConfig creates a long pool using the given LongPoolConfig.
func NewLongPoolWithConfig(cfg LongPoolConfig) *LongPool {
	idleConfig := cfg.IdleConfig
	limit := utils.NewMaxCounter(idleConfig.MaxIdleGlobal)
	lp := &LongPool{
		reporter:   &DummyReporter{},
		globalIdle: limit,
		newPeer: func(addr net.Addr) *peer {
			return newPeer(
				cfg.ServiceName,
				addr,
				idleConfig,
				cfg.ProactiveCheckConfig,
				limit)
		},
		config: cfg,
	}

	evictInterval := idleConfig.MaxIdleTimeout
	if cfg.ProactiveCheckConfig.Enable {
		if interval := cfg.ProactiveCheckConfig.Interval; interval < evictInterval {
			evictInterval = interval
		}
	}

	// add this long pool into the sharedTicker
	lp.sharedTicker = getSharedTicker(lp, evictInterval)
	return lp
}

// NewLongPool creates a long pool using the given IdleConfig.
func NewLongPool(serviceName string, idlConfig connpool.IdleConfig) *LongPool {
	limit := utils.NewMaxCounter(idlConfig.MaxIdleGlobal)
	pcfg := ProactiveCheckConfig{}
	lp := &LongPool{
		reporter:   &DummyReporter{},
		globalIdle: limit,
		newPeer: func(addr net.Addr) *peer {
			return newPeer(
				serviceName,
				addr,
				idlConfig,
				pcfg,
				limit)
		},
		config: LongPoolConfig{
			serviceName,
			idlConfig,
			pcfg,
		},
	}
	// add this long pool into the sharedTicker
	lp.sharedTicker = getSharedTicker(lp, idlConfig.MaxIdleTimeout)
	return lp
}

// LongPool manages a pool of long connections.
type LongPool struct {
	reporter     Reporter
	peerMap      sync.Map
	newPeer      func(net.Addr) *peer
	globalIdle   *utils.MaxCounter
	config       LongPoolConfig
	sharedTicker *utils.SharedTicker
	closed       int32 // active: 0, closed: 1
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
	m[idleConfigDumpKey] = lp.config.IdleConfig
	lp.peerMap.Range(func(key, value interface{}) bool {
		t := value.(*peer).pool.Dump()
		m[key.(netAddr).String()] = t
		return true
	})
	return m
}

// Config returns the config of the long pool
func (lp *LongPool) Config() LongPoolConfig {
	return lp.config
}

// Close releases all peers in the pool, it is executed when client is closed.
func (lp *LongPool) Close() error {
	if !atomic.CompareAndSwapInt32(&lp.closed, 0, 1) {
		return fmt.Errorf("long pool is already closed")
	}
	// close all peers
	lp.peerMap.Range(func(addr, value interface{}) bool {
		lp.peerMap.Delete(addr)
		v := value.(*peer)
		v.Close()
		return true
	})
	// remove from the shared ticker
	lp.sharedTicker.Delete(lp)
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
func (lp *LongPool) Evict() {
	if atomic.LoadInt32(&lp.closed) == 0 {
		// Evict idle connections
		lp.peerMap.Range(func(key, value interface{}) bool {
			p := value.(*peer)
			p.Evict()
			return true
		})
	}
}

// Tick implements the interface utils.TickerTask.
func (lp *LongPool) Tick() {
	lp.Evict()
}

// getPeer gets a peer from the pool based on the addr, or create a new one if not exist.
func (lp *LongPool) getPeer(addr netAddr) *peer {
	p, ok := lp.peerMap.Load(addr)
	if ok {
		return p.(*peer)
	}
	p, _ = lp.peerMap.LoadOrStore(addr, lp.newPeer(addr))
	return p.(*peer)
}
