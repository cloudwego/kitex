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

package utils

import (
	"sync"
	"time"
)

type RefreshTask interface {
	Refresh()
}

func NewSharedTicker(refreshInterval time.Duration) *SharedTicker {
	return &SharedTicker{
		Interval: refreshInterval,
		tasks:    map[RefreshTask]struct{}{},
		stopChan: make(chan struct{}, 1),
	}
}

type SharedTicker struct {
	sync.Mutex
	started  bool
	Interval time.Duration
	tasks    map[RefreshTask]struct{}
	stopChan chan struct{}
}

func (t *SharedTicker) Add(b RefreshTask) {
	t.Lock()
	defer t.Unlock()
	// Add task
	t.tasks[b] = struct{}{}
	if !t.started {
		t.started = true
		go t.Tick(t.Interval)
	}
}

func (t *SharedTicker) Delete(b RefreshTask) {
	t.Lock()
	defer t.Unlock()
	// Delete from tasks
	delete(t.tasks, b)
	// no tasks remaining then stop the Tick
	if len(t.tasks) == 0 {
		// unblocked when multi Delete call
		select {
		case t.stopChan <- struct{}{}:
			t.started = false
		default:
		}
	}
}

func (t *SharedTicker) Closed() bool {
	t.Lock()
	defer t.Unlock()
	return !t.started
}

func (t *SharedTicker) Tick(interval time.Duration) {
	var wg sync.WaitGroup
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.Lock()
			for b := range t.tasks {
				wg.Add(1)
				go func(b RefreshTask) {
					defer wg.Done()
					b.Refresh()
				}(b)
			}
			wg.Wait()
			t.Unlock()
		case <-t.stopChan:
			return
		}
	}
}
