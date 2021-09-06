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

package rpcinfo

import (
	"context"
	"sync"
	"time"

	"github.com/jackedelic/kitex/internal"
	"github.com/jackedelic/kitex/pkg/stats"
)

var (
	_            RPCStats = &rpcStats{}
	rpcStatsPool sync.Pool
	eventPool    sync.Pool
	once         sync.Once
	maxEventNum  int
)

type event struct {
	event  stats.Event
	status stats.Status
	info   string
	time   time.Time
}

// Event implements the Event interface.
func (e *event) Event() stats.Event {
	return e.event
}

// Status implements the Event interface.
func (e *event) Status() stats.Status {
	return e.status
}

// Info implements the Event interface.
func (e *event) Info() string {
	return e.info
}

// Time implements the Event interface.
func (e *event) Time() time.Time {
	return e.time
}

// IsNil implements the Event interface.
func (e *event) IsNil() bool {
	return e == nil
}

func newEvent() interface{} {
	return &event{}
}

func (e *event) zero() {
	e.event = nil
	e.status = 0
	e.info = ""
	e.time = time.Time{}
}

// Recycle reuses the event.
func (e *event) Recycle() {
	e.zero()
	eventPool.Put(e)
}

type rpcStats struct {
	sync.RWMutex
	level stats.Level

	eventMap []Event

	sendSize uint64
	recvSize uint64

	err      error
	panicErr interface{}
}

func init() {
	rpcStatsPool.New = newRPCStats
	eventPool.New = newEvent
}

func newRPCStats() interface{} {
	return &rpcStats{
		eventMap: make([]Event, maxEventNum),
	}
}

// Record implements the RPCStats interface.
func (r *rpcStats) Record(ctx context.Context, e stats.Event, status stats.Status, info string) {
	if e.Level() > r.level {
		return
	}
	eve := eventPool.Get().(*event)
	eve.event = e
	eve.status = status
	eve.info = info
	eve.time = time.Now()

	idx := e.Index()
	r.Lock()
	r.eventMap[idx] = eve
	r.Unlock()
}

// SendSize implements the RPCStats interface.
func (r *rpcStats) SendSize() uint64 {
	return r.sendSize
}

// RecvSize implements the RPCStats interface.
func (r *rpcStats) RecvSize() uint64 {
	return r.recvSize
}

// Error implements the RPCStats interface.
func (r *rpcStats) Error() error {
	return r.err
}

// Panicked implements the RPCStats interface.
func (r *rpcStats) Panicked() (bool, interface{}) {
	return r.panicErr != nil, r.panicErr
}

// GetEvent implements the RPCStats interface.
func (r *rpcStats) GetEvent(e stats.Event) Event {
	idx := e.Index()
	r.RLock()
	evt := r.eventMap[idx]
	r.RUnlock()
	if evt == nil || evt.IsNil() {
		return nil
	}
	return evt
}

// Level implements the RPCStats interface.
func (r *rpcStats) Level() stats.Level {
	return r.level
}

// SetSendSize sets send size.
func (r *rpcStats) SetSendSize(size uint64) {
	r.sendSize = size
}

// SetRecvSize sets recv size.
func (r *rpcStats) SetRecvSize(size uint64) {
	r.recvSize = size
}

// SetError sets error.
func (r *rpcStats) SetError(err error) {
	r.err = err
}

// SetPanicked sets if panicked.
func (r *rpcStats) SetPanicked(x interface{}) {
	r.panicErr = x
}

// SetLevel sets the level.
func (r *rpcStats) SetLevel(level stats.Level) {
	r.level = level
}

// Reset resets the stats.
func (r *rpcStats) Reset() {
	r.level = 0
	r.err = nil
	r.panicErr = nil
	r.recvSize = 0
	r.sendSize = 0
	for i := range r.eventMap {
		if t, ok := r.eventMap[i].(internal.Reusable); ok {
			t.Recycle()
		}
		r.eventMap[i] = nil
	}
}

// ImmutableView restricts the rpcStats into a read-only rpcinfo.RPCStats.
func (r *rpcStats) ImmutableView() RPCStats {
	return r
}

// Recycle reuses the rpcStats.
func (r *rpcStats) Recycle() {
	r.Reset()
	rpcStatsPool.Put(r)
}

// NewRPCStats creates a new RPCStats.
func NewRPCStats() RPCStats {
	once.Do(func() {
		stats.FinishInitialization()
		maxEventNum = stats.MaxEventNum()
	})
	return rpcStatsPool.Get().(*rpcStats)
}
