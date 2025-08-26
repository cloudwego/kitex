/*
 * Copyright 2025 CloudWeGo Authors
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

package context_watcher

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/klog"
)

// Errors
var (
	errAlreadyDone         = errors.New("context already done; callback executed immediately")
	errRegisterNilCtx      = errors.New("register nil ctx")
	errRegisterNilCallback = errors.New("register nil callback")

	errShardFull     = errors.New("shard has reached the maximum number of ctx it can listen to")
	errWatcherClosed = errors.New("watcher is closed")
)

var (
	defInitialShards  = runtime.NumCPU()
	defMaxCtxPerShard = 65536 - 1 - 512 // 65536(max supported listening num) 1(default op chan) 512(reserve some space)

	registryEntryPool = sync.Pool{
		New: func() interface{} {
			return &registryEntry{}
		},
	}
)

// Watcher is used to monitor a large number of ctx.Done() calls and synchronously execute the ctx callback after ctx.Done() is triggered.
// When Kitex's minimum Go version compatibility reaches v1.21, context.AfterFunc should be used instead.
type Watcher struct {
	shards   []*shard
	shardsMu sync.RWMutex

	registry   map[uintptr]*registryEntry
	registryMu sync.Mutex // the probability of concurrent registration with the same ctx is very low, using write Lock directly

	removals       chan uintptr
	maxCtxPerShard int
	closed         int32
	wg             sync.WaitGroup

	removerFinished chan struct{}
	removerExit     chan struct{}
}

type registryEntry struct {
	// the shard allocated to
	shard  *shard
	status registryStatus
}

func (entry *registryEntry) Recycle() {
	entry.shard = nil
	registryEntryPool.Put(entry)
}

func newRegisteringEntry() *registryEntry {
	entry := registryEntryPool.Get().(*registryEntry)
	entry.status = statusRegistering
	return entry
}

type registryStatus int8

const (
	statusRegistering registryStatus = iota
	statusRegistered
)

func NewWatcher() *Watcher {
	return newWatcher(defInitialShards, defMaxCtxPerShard)
}

func newWatcher(initialShards, maxCtxPerShard int) *Watcher {
	w := &Watcher{
		shards:          make([]*shard, 0, initialShards),
		registry:        make(map[uintptr]*registryEntry),
		removals:        make(chan uintptr, 1024),
		maxCtxPerShard:  maxCtxPerShard,
		removerFinished: make(chan struct{}),
		removerExit:     make(chan struct{}),
	}

	for i := 0; i < initialShards; i++ {
		w.createAndStartShard()
	}
	w.startRemovalProcess()

	return w
}

func (w *Watcher) Close() error {
	if !atomic.CompareAndSwapInt32(&w.closed, 0, 1) {
		return nil
	}

	// wait for all shards finished
	w.shardsMu.RLock()
	for _, s := range w.shards {
		s.close()
	}
	w.shardsMu.RUnlock()
	w.wg.Wait()

	// wait for remover process finished
	w.signalRemovalProcessExit()
	w.waitRemovalProcessFinished()

	return nil
}

func (w *Watcher) Register(ctx context.Context, callback func(context.Context)) error {
	if atomic.LoadInt32(&w.closed) == 1 {
		return errWatcherClosed
	}

	if ctx == nil {
		return errRegisterNilCtx
	}
	if callback == nil {
		return errRegisterNilCallback
	}
	done := ctx.Done()
	if done == nil {
		return nil
	}
	// check if context is already done
	if isDone(ctx) {
		safeSyncCallback(callback, ctx)
		return nil
	}

	donePtr := reflect.ValueOf(done).Pointer()

	if !w.tryReserveRegistration(donePtr) {
		// already registered
		return nil
	}

registerProcess:
	s := w.findOrCreateShard(donePtr)
	// attempt registration
	if err := s.register(ctx, callback); err != nil {
		if err == errAlreadyDone {
			w.cleanupReservation(donePtr)
			return nil
		}
		// shard has reached the maximum number of ctx it can listen to due to high concurrency
		// repeat the registerProcess again
		goto registerProcess
	}

	// mark as successfully registered, then this ctx could be visible
	w.markAsRegistered(donePtr, s)
	return nil
}

func (w *Watcher) Deregister(ctx context.Context) {
	if atomic.LoadInt32(&w.closed) == 1 {
		return
	}

	if ctx == nil {
		return
	}
	done := ctx.Done()
	if done == nil {
		return
	}

	donePtr := reflect.ValueOf(done).Pointer()

	var shardDeregister bool
	w.registryMu.Lock()
	entry, exists := w.registry[donePtr]
	if exists && entry.status == statusRegistered {
		delete(w.registry, donePtr)
		shardDeregister = true
	}
	w.registryMu.Unlock()

	if shardDeregister && entry.shard != nil {
		entry.shard.deregister(ctx)
		entry.Recycle()
	}
}

func (w *Watcher) tryReserveRegistration(donePtr uintptr) bool {
	w.registryMu.Lock()
	defer w.registryMu.Unlock()
	if _, exists := w.registry[donePtr]; exists {
		return false
	}

	w.registry[donePtr] = newRegisteringEntry()
	return true
}

func (w *Watcher) markAsRegistered(donePtr uintptr, shard *shard) {
	w.registryMu.Lock()
	if entry, exists := w.registry[donePtr]; exists {
		entry.shard = shard
		entry.status = statusRegistered
	}
	w.registryMu.Unlock()
}

func (w *Watcher) cleanupReservation(donePtr uintptr) {
	w.registryMu.Lock()
	delete(w.registry, donePtr)
	w.registryMu.Unlock()
}

func (w *Watcher) findOrCreateShard(donePtr uintptr) *shard {
	w.shardsMu.RLock()
	if s := w.findAvailableShard(donePtr); s != nil {
		w.shardsMu.RUnlock()
		return s
	}
	w.shardsMu.RUnlock()

	w.shardsMu.Lock()
	defer w.shardsMu.Unlock()

	// avoid constructing multiple shards for concurrent requests
	if s := w.findAvailableShard(donePtr); s != nil {
		return s
	}

	// create and start new shard
	return w.createAndStartShard()
}

func (w *Watcher) findAvailableShard(donePtr uintptr) *shard {
	if len(w.shards) == 0 {
		return nil
	}

	length := len(w.shards)
	start := int(donePtr) % length
	for i := 0; i < length; i++ {
		idx := (start + i) % length
		s := w.shards[idx]
		if s.getCtxCount() < int32(w.maxCtxPerShard) {
			return s
		}
	}
	return nil
}

func (w *Watcher) createAndStartShard() *shard {
	s := newShard(int32(w.maxCtxPerShard), w.removals)
	w.shards = append(w.shards, s)

	w.wg.Add(1)
	gofunc.GoFunc(context.Background(), func() {
		defer w.wg.Done()
		s.run()
	})

	return s
}

func (w *Watcher) startRemovalProcess() {
	gofunc.GoFunc(context.Background(), func() {
		defer close(w.removerFinished)
		for {
			select {
			case donePtr := <-w.removals:
				w.registryMu.Lock()
				entry, ok := w.registry[donePtr]
				if ok {
					delete(w.registry, donePtr)
					entry.Recycle()
				}
				w.registryMu.Unlock()
			case <-w.removerExit:
				return
			}
		}
	})
}

func (w *Watcher) signalRemovalProcessExit() {
	close(w.removerExit)
}

func (w *Watcher) waitRemovalProcessFinished() {
	<-w.removerFinished
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func safeSyncCallback(callback func(context.Context), ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			klog.Warnf("[KITEX] context watcher invoked callback panic, err: %v, stack: %s", r, debug.Stack())
		}
	}()
	callback(ctx)
}
