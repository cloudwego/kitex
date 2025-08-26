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
	"reflect"
	"sync/atomic"
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
	opClose
)

type shard struct {
	maxCount   int32
	opChan     chan operation
	removals   chan<- uintptr
	count      int32
	closedChan chan struct{}

	// Select case management
	cases     []reflect.SelectCase
	contexts  []context.Context
	callbacks []func(context.Context)
	indexMap  map[uintptr]int
}

func newShard(maxCount int32, removals chan<- uintptr) *shard {
	s := &shard{
		maxCount:   maxCount,
		opChan:     make(chan operation, 64),
		removals:   removals,
		closedChan: make(chan struct{}),
		contexts:   make([]context.Context, 0, maxCount),
		callbacks:  make([]func(context.Context), 0, maxCount),
		indexMap:   make(map[uintptr]int, maxCount),
	}

	// default operation chan
	s.cases = make([]reflect.SelectCase, 1, maxCount+1)
	s.cases[0] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(s.opChan),
	}

	return s
}

func (s *shard) isClosed() bool {
	select {
	case <-s.closedChan:
		return true
	default:
		return false
	}
}

func (s *shard) register(ctx context.Context, callback func(context.Context)) error {
	if s.isClosed() {
		return nil
	}

	done := make(chan error, 1)
	op := operation{
		typ:      opRegister,
		ctx:      ctx,
		callback: callback,
		done:     done,
	}
	select {
	case s.opChan <- op:
		return <-done
	case <-s.closedChan:
		return nil
	}
}

func (s *shard) deregister(ctx context.Context) {
	if s.isClosed() {
		return
	}

	op := operation{
		typ: opDeregister,
		ctx: ctx,
	}
	select {
	case s.opChan <- op:
	case <-s.closedChan:
	}
}

func (s *shard) close() {
	if s.isClosed() {
		return
	}

	op := operation{
		typ: opClose,
	}
	select {
	case s.opChan <- op:
	case <-s.closedChan:
	}
}

func (s *shard) run() {
	for {
		chosen, recv, _ := reflect.Select(s.cases)
		// operation chan
		if chosen == 0 {
			op := recv.Interface().(operation)
			if s.handleOperation(op) {
				return
			}
			continue
		}
		// ctx.Done triggered
		s.handleContextDone(chosen - 1)
	}
}

func (s *shard) handleOperation(op operation) (shouldExited bool) {
	switch op.typ {
	case opRegister:
		err := s.doRegister(op.ctx, op.callback)
		if op.done != nil {
			op.done <- err
		}
	case opDeregister:
		s.doDeregister(op.ctx)
	case opClose:
		s.doClose()
		shouldExited = true
	}
	return shouldExited
}

func (s *shard) doRegister(ctx context.Context, callback func(context.Context)) error {
	// already done
	if isDone(ctx) {
		safeSyncCallback(callback, ctx)
		return errAlreadyDone
	}

	donePtr := reflect.ValueOf(ctx.Done()).Pointer()
	// already exist
	if _, exists := s.indexMap[donePtr]; exists {
		return nil
	}

	if s.getCtxCount() >= s.maxCount {
		return errShardFull
	}

	// add to watch list
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

func (s *shard) doClose() {
	close(s.closedChan)
}

func (s *shard) handleContextDone(index int) {
	if index < 0 || index >= len(s.contexts) {
		return
	}

	ctx := s.contexts[index]
	callback := s.callbacks[index]
	donePtr := reflect.ValueOf(ctx.Done()).Pointer()

	// execute callback synchronously
	safeSyncCallback(callback, ctx)
	// remove from watch list
	s.removeAtIndex(index, donePtr)
	s.notifyWatcherCleanupCtx(donePtr)
}

// removeAtIndex remove the element at index in cases, contexts and callbacks,
// and remove the index mapping of key donePtr
func (s *shard) removeAtIndex(index int, donePtr uintptr) {
	lastIndex := len(s.contexts) - 1
	// element in the middle
	// move last element to index position
	if index < lastIndex {
		// the 0-index case is opChan, so +1 here
		s.cases[index+1] = s.cases[lastIndex+1]
		s.contexts[index] = s.contexts[lastIndex]
		s.callbacks[index] = s.callbacks[lastIndex]

		// update moved element's index mapping
		movedPtr := reflect.ValueOf(s.contexts[index].Done()).Pointer()
		s.indexMap[movedPtr] = index
	}

	// remove last element
	s.cases = s.cases[:lastIndex+1]
	s.contexts = s.contexts[:lastIndex]
	s.callbacks = s.callbacks[:lastIndex]

	delete(s.indexMap, donePtr)
	atomic.AddInt32(&s.count, -1)
}

func (s *shard) getCtxCount() int32 {
	return atomic.LoadInt32(&s.count)
}

func (s *shard) notifyWatcherCleanupCtx(donePtr uintptr) {
	s.removals <- donePtr
}
