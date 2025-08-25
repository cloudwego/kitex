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
	"sync"
	"sync/atomic"
)

// Errors
var (
	errAlreadyDone   = errors.New("context already done; callback executed immediately")
	errShardFull     = errors.New("shard has reached the maximum number of ctx it can listen to")
	errWatcherClosed = errors.New("watcher is closed")
)

type Watcher struct {
	shards   []*shard
	shardsMu sync.RWMutex

	registry   map[uintptr]*registryEntry
	registryMu sync.RWMutex

	removals       chan uintptr
	maxCtxPerShard int
	closed         atomic.Bool
	wg             sync.WaitGroup
}

type registryEntry struct {
	// the shard allocated to
	shard  *shard
	status registryStatus
}

type registryStatus int8

const (
	statusRegistering registryStatus = iota
	statusRegistered
)

type operation struct {
	typ      operationType
	ctx      context.Context
	callback func(context.Context)
	done     chan error
}

type operationType int8

const (
	opRegister operationType = iota
	opDeregister
)

type shard struct {
	maxCount int32
	opChan   chan operation
	removals chan<- uintptr
	count    int32

	// Select case management
	cases     []reflect.SelectCase
	contexts  []context.Context
	callbacks []func(context.Context)
	indexMap  map[uintptr]int
}

func NewWatcher() *Watcher {
	initialShards := runtime.NumCPU()
	w := &Watcher{
		shards:         make([]*shard, 0, initialShards),
		registry:       make(map[uintptr]*registryEntry),
		removals:       make(chan uintptr, 4096),
		maxCtxPerShard: 65023, // 65536 - 1 (opCh) - 512 (margin)
	}

	// Create initial shards
	for i := 0; i < initialShards; i++ {
		w.createAndStartShard()
	}

	// Start removal processor
	w.startRemovalProcessor()

	return w
}

func (w *Watcher) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}

	// Stop all shards
	w.shardsMu.RLock()
	for _, s := range w.shards {
		close(s.opChan)
	}
	w.shardsMu.RUnlock()

	// Stop removal processor
	close(w.removals)
	w.wg.Wait()

	return nil
}

func (w *Watcher) Register(ctx context.Context, callback func(context.Context)) error {
	if w.closed.Load() {
		return errWatcherClosed
	}

	if ctx == nil || callback == nil {
		return nil
	}
	done := ctx.Done()
	if done == nil {
		return nil
	}
	// Check if context is already done
	if isDone(ctx) {
		safeCallback(callback, ctx)
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

	// Mark as successfully registered
	w.markAsRegistered(donePtr, s)
	return nil
}

func (w *Watcher) Deregister(ctx context.Context) {
	if ctx == nil {
		return
	}

	done := ctx.Done()
	if done == nil {
		return
	}

	donePtr := reflect.ValueOf(done).Pointer()

	w.registryMu.Lock()
	entry, exists := w.registry[donePtr]
	if exists {
		delete(w.registry, donePtr)
	}
	w.registryMu.Unlock()

	if exists && entry.shard != nil {
		entry.shard.deregister(ctx)
	}
}

func (w *Watcher) tryReserveRegistration(donePtr uintptr) bool {
	w.registryMu.Lock()
	defer w.registryMu.Unlock()

	if _, exists := w.registry[donePtr]; exists {
		return false
	}

	w.registry[donePtr] = &registryEntry{status: statusRegistering}
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

	// Create and start new shard
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
	go func() {
		defer w.wg.Done()
		s.run()
	}()

	return s
}

func (w *Watcher) startRemovalProcessor() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for donePtr := range w.removals {
			w.registryMu.Lock()
			delete(w.registry, donePtr)
			w.registryMu.Unlock()
		}
	}()
}

func newShard(maxCount int32, removals chan<- uintptr) *shard {
	s := &shard{
		maxCount:  maxCount,
		opChan:    make(chan operation, 64),
		removals:  removals,
		contexts:  make([]context.Context, 0, maxCount),
		callbacks: make([]func(context.Context), 0, maxCount),
		indexMap:  make(map[uintptr]int, maxCount),
	}

	// default operation chan
	s.cases = make([]reflect.SelectCase, 1, maxCount+1)
	s.cases[0] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.opChan),
	}

	return s
}

func (s *shard) register(ctx context.Context, callback func(context.Context)) error {
	done := make(chan error, 1)
	op := operation{
		typ:      opRegister,
		ctx:      ctx,
		callback: callback,
		done:     done,
	}
	s.opChan <- op
	return <-done
}

func (s *shard) deregister(ctx context.Context) {
	op := operation{
		typ: opDeregister,
		ctx: ctx,
	}

	s.opChan <- op
}

func (s *shard) run() {
	for {
		chosen, recv, ok := reflect.Select(s.cases)

		// Operation chan
		if chosen == 0 {
			if !ok {
				return // shard shutdown
			}
			op := recv.Interface().(operation)
			s.handleOperation(op)
			continue
		}

		// ctx.Done triggered
		s.handleContextDone(chosen - 1)
	}
}

func (s *shard) handleOperation(op operation) {
	switch op.typ {
	case opRegister:
		err := s.doRegister(op.ctx, op.callback)
		if op.done != nil {
			op.done <- err
		}

	case opDeregister:
		s.doDeregister(op.ctx)
	}
}

func (s *shard) doRegister(ctx context.Context, callback func(context.Context)) error {
	if isDone(ctx) {
		safeCallback(callback, ctx)
		return errAlreadyDone
	}

	donePtr := reflect.ValueOf(ctx.Done()).Pointer()
	if _, exists := s.indexMap[donePtr]; exists {
		return nil
	}

	if s.getCtxCount() >= s.maxCount {
		return errShardFull
	}

	// Add to watch list
	index := len(s.contexts)
	s.cases = append(s.cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	})
	s.contexts = append(s.contexts, ctx)
	s.callbacks = append(s.callbacks, callback)
	s.indexMap[donePtr] = index
	atomic.AddInt32(&s.count, 1)

	return nil
}

func (s *shard) doDeregister(ctx context.Context) {
	donePtr := reflect.ValueOf(ctx.Done()).Pointer()

	if index, exists := s.indexMap[donePtr]; exists {
		s.removeAtIndex(index, donePtr)
	}
}

func (s *shard) handleContextDone(index int) {
	if index < 0 || index >= len(s.contexts) {
		return
	}

	ctx := s.contexts[index]
	callback := s.callbacks[index]
	donePtr := reflect.ValueOf(ctx.Done()).Pointer()

	// Execute callback
	safeCallback(callback, ctx)

	// Remove from watch list
	s.removeAtIndex(index, donePtr)

	// Notify watcher for cleanup
	select {
	case s.removals <- donePtr:
	// 这里真的有必要么？
	default:
		// Non-blocking send to avoid deadlock
		go func() {
			s.removals <- donePtr
		}()
	}
}

func (s *shard) removeAtIndex(index int, donePtr uintptr) {
	lastIndex := len(s.contexts) - 1

	if index < lastIndex {
		// Move last element to index position
		s.cases[index+1] = s.cases[lastIndex+1]
		s.contexts[index] = s.contexts[lastIndex]
		s.callbacks[index] = s.callbacks[lastIndex]

		// Update moved element's index mapping
		movedPtr := reflect.ValueOf(s.contexts[index].Done()).Pointer()
		s.indexMap[movedPtr] = index
	}

	s.cases = s.cases[:lastIndex+1]
	s.contexts = s.contexts[:lastIndex]
	s.callbacks = s.callbacks[:lastIndex]

	delete(s.indexMap, donePtr)
	atomic.AddInt32(&s.count, -1)
}

func (s *shard) getCtxCount() int32 {
	return atomic.LoadInt32(&s.count)
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func safeCallback(callback func(context.Context), ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			_ = r
		}
	}()
	callback(ctx)
}
