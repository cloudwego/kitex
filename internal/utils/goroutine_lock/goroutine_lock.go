/*
 * Copyright 2024 CloudWeGo Authors
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

// Package goroutinelock implements goroutine locks.
package goroutinelock

import (
	"sync"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/klog"
)

// Wg is used to implement goroutine locks.
var Wg = &WaitGroup{}

// WaitGroup defines a wait group with counter and status.
type WaitGroup struct {
	sync.WaitGroup
	count           int64
	shutdownStarted uint32
}

// Add adds delta and bumps counter.
func (wg *WaitGroup) Add(delta int) {
	atomic.AddInt64(&wg.count, int64(delta))
	wg.WaitGroup.Add(delta)
	if atomic.LoadUint32(&wg.shutdownStarted) > 0 {
		klog.Warn("KITEX: shutdown started but a new goroutine lock is added")
	}
}

// Done decrease wait group counter by 1.
func (wg *WaitGroup) Done() {
	atomic.AddInt64(&wg.count, -1)
	wg.WaitGroup.Done()
	if atomic.LoadUint32(&wg.shutdownStarted) > 0 {
		count := wg.GetCount()
		if count > 0 {
			klog.Infof("KITEX: waiting for goroutine locks to be released, remaining %d...", count)
		} else {
			klog.Info("KITEX: all goroutine locks have been released")
		}
	}
}

// GetCount gets wait group counter.
func (wg *WaitGroup) GetCount() int {
	return int(atomic.LoadInt64(&wg.count))
}

// StartShutdown sets the shutdown status to true.
func (wg *WaitGroup) StartShutdown() {
	atomic.AddUint32(&wg.shutdownStarted, 1)
}
