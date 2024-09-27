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
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/internal"
	"github.com/cloudwego/kitex/pkg/stats"
)

var (
	_ RPCStats          = (*rpcStats)(nil)
	_ MutableRPCStats   = (*rpcStats)(nil)
	_ internal.Reusable = (*rpcStats)(nil)
	_ internal.Reusable = (*event)(nil)

	rpcStatsPool = sync.Pool{New: func() interface{} { return newRPCStats() }}
	eventPool    = sync.Pool{New: func() interface{} { return &event{} }}

	once        sync.Once
	maxEventNum int
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

type atomicErr struct {
	err error
}

type atomicPanicErr struct {
	panicErr interface{}
}

type rpcStats struct {
	level stats.Level

	eventStatus []uint32 // see comments of eventXXX consts below
	eventMap    []event

	sendSize     uint64
	lastSendSize uint64 // for Streaming APIs, record the size of the last sent message
	recvSize     uint64
	lastRecvSize uint64 // for Streaming APIs, record the size of the last received message

	err      atomic.Value
	panicErr atomic.Value

	// true if rpcStats is from CopyForRetry
	copied bool
}

const ( // for (*rpcStats).eventStatus
	eventUnset    uint32 = 0b0000 // unset
	eventUpdating uint32 = 0b0001 // updating, it's considered to be unset
	eventRecorded uint32 = 0b0010 // updated, GetEvent will return the event

	// eventStale is only set by CopyForRetry,
	// it represents data is recorded and copied from last rpcstat.
	//
	// FIXME:
	// it may be overwritten later and would cause an issue below:
	// - before retry, A, B are recorded, and user may use (B - A) for measuring latency.
	// - after retry, only A is recorded, then (B - A) will be a negative number.
	// - this is NOT a new issue introduced by eventStatus but caused by CopyForRetry.
	// it may also cause data race issue if Record and GetEvent at the same time.
	eventStale uint32 = 0b0100 | eventRecorded
)

func newRPCStats() *rpcStats {
	return &rpcStats{
		eventStatus: make([]uint32, maxEventNum),
		eventMap:    make([]event, maxEventNum),
	}
}

// NewRPCStats creates a new RPCStats.
func NewRPCStats() RPCStats {
	once.Do(func() {
		stats.FinishInitialization()
		maxEventNum = stats.MaxEventNum()
	})
	return rpcStatsPool.Get().(*rpcStats)
}

// Record implements the RPCStats interface.
// It only record once for each event, it will ignore follow events with the same Index()
func (r *rpcStats) Record(ctx context.Context, e stats.Event, status stats.Status, info string) {
	if e.Level() > r.level {
		return
	}
	idx := e.Index()
	p := &r.eventStatus[idx]
	if atomic.CompareAndSwapUint32(p, eventUnset, eventUpdating) ||
		(r.copied && atomic.CompareAndSwapUint32(p, eventStale, eventUpdating)) {
		r.eventMap[idx] = event{event: e, status: status, info: info, time: time.Now()}
		atomic.StoreUint32(p, eventRecorded) // done, make it visible to GetEvent
	} else {
		// eventRecorded? panic?
	}
}

// NewEvent creates a new Event based on the given event, status and info.
//
// It's only used by ReportStreamEvent
func NewEvent(statsEvent stats.Event, status stats.Status, info string) Event {
	eve := eventPool.Get().(*event)
	eve.event = statsEvent
	eve.status = status
	eve.info = info
	eve.time = time.Now()
	return eve
}

// SendSize implements the RPCStats interface.
func (r *rpcStats) SendSize() uint64 {
	return atomic.LoadUint64(&r.sendSize)
}

// LastSendSize implements the RPCStats interface.
func (r *rpcStats) LastSendSize() uint64 {
	return atomic.LoadUint64(&r.lastSendSize)
}

// RecvSize implements the RPCStats interface.
func (r *rpcStats) RecvSize() uint64 {
	return atomic.LoadUint64(&r.recvSize)
}

// LastRecvSize implements the RPCStats interface.
func (r *rpcStats) LastRecvSize() uint64 {
	return atomic.LoadUint64(&r.lastRecvSize)
}

// Error implements the RPCStats interface.
func (r *rpcStats) Error() error {
	ae, _ := r.err.Load().(atomicErr)
	return ae.err
}

// Panicked implements the RPCStats interface.
func (r *rpcStats) Panicked() (bool, interface{}) {
	ape, _ := r.panicErr.Load().(atomicPanicErr)
	return ape.panicErr != nil, ape.panicErr
}

// GetEvent implements the RPCStats interface.
func (r *rpcStats) GetEvent(e stats.Event) Event {
	idx := e.Index()
	if atomic.LoadUint32(&r.eventStatus[idx])&eventRecorded != 0 {
		// no need to check (*event).IsNil() ... it's useless
		return &r.eventMap[idx]
	}
	return nil
}

// Level implements the RPCStats interface.
func (r *rpcStats) Level() stats.Level {
	return r.level
}

// CopyForRetry implements the RPCStats interface, it copies a RPCStats from the origin one
// to pass through info of the first request to retrying requests.
func (r *rpcStats) CopyForRetry() RPCStats {
	// Copied rpc stats is for request retrying and cannot be reused, so no need to get from pool.
	nr := newRPCStats()
	nr.level = r.level // it will be written before retry by client.Options
	nr.copied = true   // for GetEvent when status=eventStale

	// RPCStart is the only internal event we need to keep
	startIdx := int(stats.RPCStart.Index())
	// user-defined events start index
	userIdx := stats.PredefinedEventNum()
	for i := 0; i < len(nr.eventMap); i++ {
		// Ignore none RPCStart events to avoid incorrect tracing.
		if i == startIdx || i >= userIdx {
			if atomic.LoadUint32(&r.eventStatus[i]) == eventRecorded {
				nr.eventMap[i] = r.eventMap[i]
				nr.eventStatus[i] = eventStale
			}
		}
	}
	return nr
}

// SetSendSize sets send size.
// This should be called by Ping-Pong APIs which only send once.
func (r *rpcStats) SetSendSize(size uint64) {
	atomic.StoreUint64(&r.sendSize, size)
}

// IncrSendSize increments send size.
// This should be called by Streaming APIs which may send multiple times.
func (r *rpcStats) IncrSendSize(size uint64) {
	atomic.AddUint64(&r.sendSize, size)
	atomic.StoreUint64(&r.lastSendSize, size)
}

// SetRecvSize sets recv size.
// This should be called by Ping-Pong APIs which only recv once.
func (r *rpcStats) SetRecvSize(size uint64) {
	atomic.StoreUint64(&r.recvSize, size)
}

// IncrRecvSize increments recv size.
// This should be called by Streaming APIs which may recv multiple times.
func (r *rpcStats) IncrRecvSize(size uint64) {
	atomic.AddUint64(&r.recvSize, size)
	atomic.StoreUint64(&r.lastRecvSize, size)
}

// SetError sets error.
func (r *rpcStats) SetError(err error) {
	r.err.Store(atomicErr{err: err})
}

// SetPanicked sets if panicked.
func (r *rpcStats) SetPanicked(x interface{}) {
	r.panicErr.Store(atomicPanicErr{panicErr: x})
}

// SetLevel sets the level.
func (r *rpcStats) SetLevel(level stats.Level) {
	r.level = level
}

// Reset resets the stats.
func (r *rpcStats) Reset() {
	r.level = 0
	if ae, _ := r.err.Load().(atomicErr); ae.err != nil {
		r.err.Store(atomicErr{})
	}
	if ape, _ := r.panicErr.Load().(atomicPanicErr); ape.panicErr != nil {
		r.panicErr.Store(atomicPanicErr{})
	}
	atomic.StoreUint64(&r.recvSize, 0)
	atomic.StoreUint64(&r.sendSize, 0)
	for i := range r.eventMap {
		r.eventStatus[i] = eventUnset // no need atomic.StoreUint32?
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
