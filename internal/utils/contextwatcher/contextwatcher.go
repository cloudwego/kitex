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

package contextwatcher

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/cloudwego/kitex/pkg/gofunc"
)

// contextWatcherReflect uses reflect.Select to monitor multiple contexts efficiently.
// Instead of creating one goroutine per context, it uses a fixed number of shards,
// each running a single goroutine that monitors multiple contexts using reflect.Select.
type contextWatcherReflect struct {
	shardCapacity  int
	signalCapacity int

	shards         atomic.Pointer[[]*reflectShard]
	expandMutex    sync.Mutex
	ctxShards      sync.Map // map[context.Context]int - tracks which shard each context belongs to
	activeContexts atomic.Int64

	fallbackCtxs sync.Map // map[context.Context]chan struct{}
}

var (
	// global is the package-level context watcher instance
	// It is initialized lazily on first use to avoid unnecessary resource allocation
	global     *contextWatcherReflect
	globalOnce sync.Once
)

const (
	// Default configuration for the global watcher
	defaultShardCapacity  = 64  // Initial capacity for each shard
	defaultSignalCapacity = 100 // Buffer size for signal channel
)

// getGlobalWatcher returns the global context watcher instance, initializing it if necessary.
// This function is thread-safe and ensures the watcher is created only once.
func getGlobalWatcher() *contextWatcherReflect {
	globalOnce.Do(func() {
		global = newContextWatcherReflect(defaultShardNum, defaultShardCapacity, defaultSignalCapacity)
	})
	return global
}

// RegisterContext registers a context with a callback function that will be invoked
// when the context is done (cancelled or timed out).
//
// Key behaviors:
// - If ctx.Done() is nil, the function returns immediately without doing anything
// - If ctx is already done, the callback is invoked immediately in a separate goroutine
// - Duplicate registrations of the same context are ignored (only the first registration takes effect)
// - The global watcher is initialized lazily on first call
//
// The callback will be executed in a separate goroutine to avoid blocking.
// After the callback completes, the context is automatically cleaned up from the watcher.
func RegisterContext(ctx context.Context, callback func(context.Context)) {
	getGlobalWatcher().RegisterContext(ctx, callback)
}

// DeregisterContext removes a context from the watcher, preventing its callback from being invoked.
//
// Key behaviors:
// - If the context was not registered, this is a no-op
// - If the context's callback has already been invoked, this is a no-op
// - This function is safe to call multiple times with the same context
// - After deregistration, if the context is cancelled, the callback will NOT be invoked
//
// This is useful when you want to cancel monitoring a context before it completes.
func DeregisterContext(ctx context.Context) {
	// Only deregister if global watcher has been initialized
	// If it hasn't been initialized, there's nothing to deregister
	if global != nil {
		global.DeregisterContext(ctx)
	}
}

// newContextWatcherReflect creates a new contextWatcherReflect with default number of shards.
func newContextWatcherReflect(shardNum, shardCapacity, signalCapacity int) *contextWatcherReflect {
	w := &contextWatcherReflect{
		shardCapacity:  shardCapacity,
		signalCapacity: signalCapacity,
	}

	shards := make([]*reflectShard, shardNum)
	for i := 0; i < shardNum; i++ {
		shards[i] = newReflectShard(w, shardCapacity, signalCapacity)
		go shards[i].run()
	}
	w.shards.Store(&shards)

	return w
}

func (w *contextWatcherReflect) getShard(ctx context.Context) (shard *reflectShard, exist, fallback bool) {
	shards := *w.shards.Load()
	n := len(shards)
	if val, ok := w.ctxShards.Load(ctx); ok {
		idx := val.(int)
		return shards[idx], true, false
	}

	// check two candidate shards (power of two choices) and select the one with lower load
	idx1, idx2 := getRandomIndexes(ctx, n)
	selected := idx1
	load1 := shards[idx1].load.Load()
	load2 := shards[idx2].load.Load()
	finalLoad := load1
	if load2 < load1 {
		selected = idx2
		finalLoad = load2
	}
	if finalLoad > maxCasesPerShard {
		// fallback
		return nil, false, true
	}

	actual, loaded := w.ctxShards.LoadOrStore(ctx, selected)
	if loaded {
		idx := actual.(int)
		return shards[idx], true, false
	}

	w.activeContexts.Add(1)
	return shards[selected], false, false
}

func (w *contextWatcherReflect) removeActiveContext(ctx context.Context) {
	w.ctxShards.Delete(ctx)
	w.activeContexts.Add(-1)
}

func (w *contextWatcherReflect) RegisterContext(ctx context.Context, callback func(context.Context)) {
	if ctx.Done() == nil {
		return
	}
	select {
	case <-ctx.Done():
		gofunc.GoFunc(ctx, func() {
			callback(ctx)
		})
		return
	default:
	}

	shard, exist, fallback := w.getShard(ctx)
	if fallback {
		w.fallbackRegister(ctx, callback)
		w.checkAndExpand()
		return
	}
	if exist {
		return
	}

	shard.registerContext(ctx, callback)
	// Check if we need to expand shards
	w.checkAndExpand()
}

func (w *contextWatcherReflect) fallbackRegister(ctx context.Context, callback func(context.Context)) {
	finCh := make(chan struct{})
	_, loaded := w.fallbackCtxs.LoadOrStore(ctx, finCh)
	if loaded {
		return
	}

	gopool.Go(func() {
		select {
		case <-ctx.Done():
			callback(ctx)
			// clean up the map entry after callback execution
			w.fallbackCtxs.Delete(ctx)
		case <-finCh:
			return
		}
	})
}

func (w *contextWatcherReflect) DeregisterContext(ctx context.Context) {
	// fallback ctx
	rawVal, loaded := w.fallbackCtxs.LoadAndDelete(ctx)
	if loaded {
		// make goroutine exited
		close(rawVal.(chan struct{}))
		return
	}

	val, ok := w.ctxShards.Load(ctx)
	if !ok {
		// ctx not registered or already cleaned up
		return
	}

	idx := val.(int)
	shards := *w.shards.Load()
	shards[idx].deregisterContext(ctx)
}

// checkAndExpand checks if shard expansion is needed based on current load
// and triggers expansion if necessary. This is called automatically after
// each RegisterContext call.
func (w *contextWatcherReflect) checkAndExpand() {
	activeCount := w.activeContexts.Load()
	shards := *w.shards.Load()
	shardsNum := len(shards)

	// Don't expand if:
	// 1. Already at maximum shard count
	// 2. Total contexts below minimum threshold
	// 3. Average load per shard is acceptable
	if activeCount < minContextsForExpand {
		return
	}
	if shardsNum >= maxShardsNum {
		return
	}
	avgLoad := activeCount / int64(shardsNum)
	if avgLoad <= shardLoadThreshold {
		return
	}

	// Trigger expansion asynchronously to avoid blocking registration
	if w.expandMutex.TryLock() {
		go func() {
			w.expandShardsLocked()
			w.expandMutex.Unlock()
		}()
	}
}

// expandShardsLocked is the internal expansion implementation
// Caller must hold expandMutex
func (w *contextWatcherReflect) expandShardsLocked() {
	oldShards := *w.shards.Load()
	oldNum := len(oldShards)
	newNum := oldNum * 2
	if newNum > maxShardsNum {
		newNum = maxShardsNum
	}
	if newNum == oldNum {
		return
	}

	newShards := make([]*reflectShard, newNum)

	// existing contexts stay in their original shards
	copy(newShards, oldShards)

	for i := oldNum; i < newNum; i++ {
		newShards[i] = newReflectShard(w, w.shardCapacity, w.signalCapacity)
		go newShards[i].run()
	}

	w.shards.Store(&newShards)
}

func getRandomIndexes(ctx context.Context, n int) (int, int) {
	type iface struct {
		typ  uintptr
		data uintptr
	}
	ptr := (*iface)(unsafe.Pointer(&ctx)).data
	idx1 := int(ptr % uintptr(n))
	idx2 := int((ptr >> 8) % uintptr(n))
	return idx1, idx2
}

var (
	defaultShardNum = runtime.GOMAXPROCS(0) * 3 / 2
)

const (
	shardLoadThreshold   = 64  // trigger expansion when average load per shard exceeds
	minContextsForExpand = 512 // Minimum total contexts before considering expansion (reduced from 1024)
	maxCasesPerShard     = 128 // Hard limit: max cases per shard to maintain reflect.Select performance
	maxShardsNum         = 1024
)

type operationType uint8

const (
	createType operationType = iota + 1
	deleteType
)

type reflectItem struct {
	opType   operationType
	ctx      context.Context
	callback func(context.Context)
}

// reflectShard monitors multiple contexts using a single goroutine with reflect.Select.
type reflectShard struct {
	watcher    *contextWatcherReflect
	cases      []reflect.SelectCase    // cases[0] is the signal channel for control messages
	items      []reflectItem           // items[0] is not used
	ctxIndexes map[context.Context]int // ctx => index in cases or items
	signal     chan reflectItem
	load       atomic.Int32 // current number of active contexts in this shard
}

func newReflectShard(watcher *contextWatcherReflect, shardCapacity, signalCapacity int) *reflectShard {
	signal := make(chan reflectItem, signalCapacity)
	cases := make([]reflect.SelectCase, 1, shardCapacity+1)
	cases[0] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(signal),
	}
	items := make([]reflectItem, 1, shardCapacity+1)
	return &reflectShard{
		watcher:    watcher,
		cases:      cases,
		items:      items,
		ctxIndexes: make(map[context.Context]int),
		signal:     signal,
	}
}

func (s *reflectShard) registerContext(ctx context.Context, callback func(context.Context)) {
	s.signal <- reflectItem{
		opType:   createType,
		ctx:      ctx,
		callback: callback,
	}
}

func (s *reflectShard) deregisterContext(ctx context.Context) {
	s.signal <- reflectItem{
		opType: deleteType,
		ctx:    ctx,
	}
}

func (s *reflectShard) run() {
	for {
		chosen, recvVal, _ := reflect.Select(s.cases)
		if chosen == 0 {
			// signal
			item := recvVal.Interface().(reflectItem)
			switch item.opType {
			case createType:
				if _, ok := s.ctxIndexes[item.ctx]; ok {
					continue
				}

				// Check if context is already done - no need to register
				select {
				case <-item.ctx.Done():
					gofunc.GoFunc(item.ctx, func() {
						item.callback(item.ctx)
					})
					s.watcher.removeActiveContext(item.ctx)
					continue
				default:
				}

				s.cases = append(s.cases, reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(item.ctx.Done()),
				})
				s.items = append(s.items, item)
				s.ctxIndexes[item.ctx] = len(s.cases) - 1
				s.load.Add(1)
			case deleteType:
				idx, ok := s.ctxIndexes[item.ctx]
				if !ok {
					continue
				}
				s.removeItemAtIndex(idx)
				delete(s.ctxIndexes, item.ctx)
				s.load.Add(-1) // Decrement shard load
				s.watcher.removeActiveContext(item.ctx)
			}
			continue
		}
		item := s.items[chosen]
		gofunc.GoFunc(item.ctx, func() {
			item.callback(item.ctx)
		})
		s.removeItemAtIndex(chosen)
		delete(s.ctxIndexes, item.ctx)
		s.load.Add(-1) // Decrement shard load when context is done
		s.watcher.removeActiveContext(item.ctx)
	}
}

func (s *reflectShard) removeItemAtIndex(idx int) {
	lastIdx := len(s.cases) - 1
	if idx != lastIdx {
		s.cases[idx] = s.cases[lastIdx]
		s.items[idx] = s.items[lastIdx]

		moved := s.items[idx].ctx
		s.ctxIndexes[moved] = idx
	}
	s.cases[lastIdx] = reflect.SelectCase{}
	s.items[lastIdx] = reflectItem{}

	s.cases = s.cases[:lastIdx]
	s.items = s.items[:lastIdx]
}
