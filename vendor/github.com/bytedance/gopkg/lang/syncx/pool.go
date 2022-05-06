// Copyright 2021 ByteDance Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !race
// +build !race

package syncx

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/bytedance/gopkg/internal/runtimex"
)

type Pool struct {
	noCopy noCopy

	local     unsafe.Pointer // local fixed-size per-P pool, actual type is [P]poolLocal
	localSize uintptr        // size of the local array

	newSize int32 // mark every time New is executed
	gcSize  int32 // recommended number of gc

	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	New func() interface{}
	// NoGC any objects in this Pool.
	NoGC bool
}

// noCopy may be embedded into structs which must not be copied
// after the first use.
//
// See https://golang.org/issues/8005#issuecomment-190753527
// for details.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

const blockSize = 256

type block [blockSize]interface{}

const shared = 0
const unused = 1

// Local per-P Pool appendix.
type poolLocalInternal struct {
	pidx    int    // idx of private
	private *block // Can be used only by the respective P.
	// Local P can pushHead/popHead; any P can popTail.
	// 1 is shared, 2 is unused
	shared [2]poolChain
}

type poolLocal struct {
	poolLocalInternal
	// Prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	pad [128 - unsafe.Sizeof(poolLocalInternal{})%128]byte
}

// Put adds x to the pool.
func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	l, pid := p.pin()
	if l.pidx >= blockSize {
		l.shared[shared].pushHead(l.private)
		l.pidx, l.private = 0, nil
	}
	if l.private == nil {
		l.private, _ = l.shared[unused].popHead()
		if l.private == nil {
			l.private = p.getSlow(pid, unused)
		}
		if l.private == nil {
			l.private = &block{}
		}
	}
	l.private[l.pidx] = x
	l.pidx++
	x = nil
	runtimex.Unpin()
}

// Get selects an arbitrary item from the Pool, removes it from the
// Pool, and returns it to the caller.
// Get may choose to ignore the pool and treat it as empty.
// Callers should not assume any relation between values passed to Put and
// the values returned by Get.
//
// If Get would otherwise return nil and p.New is non-nil, Get returns
// the result of calling p.New.
func (p *Pool) Get() (x interface{}) {
	l, pid := p.pin()
	if l.pidx > 0 {
		l.pidx--
		x = l.private[l.pidx]
		l.private[l.pidx] = nil
	}
	if x == nil {
		// Try to pop the head of the local shard. We prefer
		// the head over the tail for temporal locality of
		// reuse.
		var b, _ = l.shared[shared].popHead()
		if b == nil {
			b = p.getSlow(pid, shared)
		}
		if b != nil {
			if l.private != nil {
				l.shared[unused].pushHead(l.private)
			}
			l.private = b
			l.pidx = blockSize - 1
			x = l.private[l.pidx]
			l.private[l.pidx] = nil
		}
	}
	runtimex.Unpin()
	if x == nil && p.New != nil {
		atomic.AddInt32(&p.newSize, 1)
		x = p.New()
	}
	return x
}

func (p *Pool) getSlow(pid int, idx int) *block {
	// See the comment in pin regarding ordering of the loads.
	size := atomic.LoadUintptr(&p.localSize) // load-acquire
	locals := p.local                        // load-consume
	// Try to steal one element from other procs.
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i+1)%int(size))
		if x, _ := l.shared[idx].popTail(); x != nil {
			return x
		}
	}
	return nil
}

// pin pins the current goroutine to P, disables preemption and
// returns poolLocal pool for the P and the P's id.
// Caller must call runtime_procUnpin() when done with the pool.
func (p *Pool) pin() (*poolLocal, int) {
	pid := runtimex.Pin()
	// In pinSlow we store to local and then to localSize, here we load in opposite order.
	// Since we've disabled preemption, GC cannot happen in between.
	// Thus here we must observe local at least as large localSize.
	// We can observe a newSize/larger local, it is fine (we must observe its zero-initialized-ness).
	s := atomic.LoadUintptr(&p.localSize) // load-acquire
	l := p.local                          // load-consume
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	return p.pinSlow()
}

func (p *Pool) pinSlow() (*poolLocal, int) {
	// Retry under the mutex.
	// Can not lock the mutex while pinned.
	runtimex.Unpin()
	allPoolsMu.Lock()
	defer allPoolsMu.Unlock()
	pid := runtimex.Pin()
	// poolCleanup won't be called while we are pinned.
	s := p.localSize
	l := p.local
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	if p.local == nil {
		allPools = append(allPools, p)
	}
	// If GOMAXPROCS changes between GCs, we re-allocate the array and lose the old one.
	size := runtime.GOMAXPROCS(0)
	local := make([]poolLocal, size)
	atomic.StorePointer(&p.local, unsafe.Pointer(&local[0])) // store-release
	atomic.StoreUintptr(&p.localSize, uintptr(size))         // store-release
	return &local[pid], pid
}

// GC will follow these rules：
// 1. Mark the tag `newSize`, if `newSize` exists this time, skip GC.
// 2. Calculate the current size, mark as `newSize`.
// 3. if `newSize`  < `oldSize`, skip GC.
// 4. GC size is oldSize/2, the real GC is to throw away a few poolLocals directly.
func (p *Pool) gc() {
	if p.NoGC {
		return
	}
	// 1. check newSize
	if p.newSize > 0 {
		p.newSize = 0
		return
	}
	var newSize int32
	for i := 0; i < int(p.localSize); i++ {
		l := indexLocal(p.local, i)
		newSize += l.shared[shared].size
	}
	// 2. if new < old; old = new
	if newSize < p.gcSize {
		p.gcSize = newSize
		return
	}
	// 3. if new < procs; return
	if newSize <= int32(p.localSize) {
		p.gcSize = newSize
		return
	}
	// 4. gc old/2
	var gcSize int32
	for i := 0; i < int(p.localSize) && gcSize < p.gcSize/2; i++ {
		l := indexLocal(p.local, i)
		gcSize += l.shared[shared].size
		l.shared[shared].size, l.shared[shared].head, l.shared[shared].tail = 0, nil, nil
		l.shared[unused].size, l.shared[unused].head, l.shared[unused].tail = 0, nil, nil
	}
	p.gcSize = newSize - gcSize
}

var (
	allPoolsMu sync.Mutex
	period     int
	// allPools is the set of pools that have non-empty primary
	// caches. Protected by either 1) allPoolsMu and pinning or 2)
	// STW.
	allPools []*Pool
)

// GC will be executed every 4 cycles
func poolCleanup() {
	runtime_poolCleanup()
	period++
	if period&0x4 == 0 {
		return
	}
	// This function is called with the world stopped, at the beginning of a garbage collection.
	// It must not allocate and probably should not call any runtime functions.

	// Because the world is stopped, no pool user can be in a
	// pinned section (in effect, this has all Ps pinned).

	// Move primary cache to victim cache.
	for _, p := range allPools {
		p.gc()
	}
}

func init() {
	// FIXME: The linkname here is risky.
	//  If Go renames these func officially, we need to synchronize here, otherwise it may cause OOM.
	runtime_registerPoolCleanup(poolCleanup)
}

func indexLocal(l unsafe.Pointer, i int) *poolLocal {
	lp := unsafe.Pointer(uintptr(l) + uintptr(i)*unsafe.Sizeof(poolLocal{}))
	return (*poolLocal)(lp)
}
