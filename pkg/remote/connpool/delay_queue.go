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

package connpool

import (
	"net"
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
)

type ConnDiscard struct {
	pool remote.ConnPool
	conn net.Conn
}

func NewConnDiscard(pool remote.ConnPool, conn net.Conn) *ConnDiscard {
	return &ConnDiscard{
		pool: pool,
		conn: conn,
	}
}

func (c *ConnDiscard) Discard() error {
	if c.pool != nil {
		return c.pool.Discard(c.conn)
	}
	return c.conn.Close()
}

type DiscardQueue struct {
	// interrupt is the time to wait for the next clean in milliseconds.
	interrupt time.Duration
	// delay is the time to wait for discard after clean start in milliseconds.
	delay       time.Duration
	discards    []*ConnDiscard
	mutex       sync.Mutex
	closeSignal chan struct{}
	once        sync.Once
}

func NewDiscardQueue(interrupt, delay time.Duration) *DiscardQueue {
	if delay < 100*time.Millisecond {
		delay = 100 * time.Millisecond
	}
	if interrupt < 100*time.Millisecond {
		interrupt = 100 * time.Millisecond
	}
	dq := &DiscardQueue{
		interrupt:   interrupt,
		delay:       delay,
		discards:    make([]*ConnDiscard, 0, 100),
		closeSignal: make(chan struct{}),
	}
	return dq
}

func (d *DiscardQueue) discard() {
LOOP:
	for range time.NewTicker(d.interrupt).C {
		d.mutex.Lock()
		go func(discards []*ConnDiscard) {
			time.Sleep(d.delay)
			for _, d := range discards {
				if err := d.Discard(); err != nil {
					klog.Warnf("DiscardQueue close conn failed, conn: %+v conn pool: %+v, error: %v", d.pool, d.conn, err)
				}
			}
		}(d.discards)
		d.discards = make([]*ConnDiscard, 0, 100)
		select {
		case <-d.closeSignal:
			d.mutex.Unlock()
			break LOOP
		default:
			d.mutex.Unlock()
		}
	}
}

func (d *DiscardQueue) Discard(discard *ConnDiscard) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.once.Do(func() {
		go d.discard()
	})
	d.discards = append(d.discards, discard)
}

func (d *DiscardQueue) Close() {
	go func() {
		d.closeSignal <- struct{}{}
	}()
}

type SynthesizedConnPool struct {
	remote.ConnPool
	discardQueue *DiscardQueue
}

func (sp *SynthesizedConnPool) DelayDiscard(conn net.Conn) {
	sp.discardQueue.Discard(NewConnDiscard(sp.ConnPool, conn))
}

func (sp *SynthesizedConnPool) Close() error {
	defer sp.discardQueue.Close()
	if err := sp.ConnPool.Close(); err != nil {
		return err
	}
	return nil
}

func WrapConnPoolIntoAsync(connPool remote.ConnPool, cfg *remote.AsyncConnPoolConfig) remote.AsyncConnPool {
	if connPool == nil {
		return nil
	}
	dq := NewDiscardQueue(cfg.DelayDiscardInterrupt, cfg.DelayDiscardTime)
	return &SynthesizedConnPool{ConnPool: connPool, discardQueue: dq}
}
