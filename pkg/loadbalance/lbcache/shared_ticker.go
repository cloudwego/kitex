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

package lbcache

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var (
	// insert, not delete
	sharedTickers    sync.Map
	sharedTickersSfg singleflight.Group
)

// shared ticker
type sharedTicker struct {
	sync.Mutex
	started  bool
	interval time.Duration
	tasks    map[*Balancer]struct{}
	stopChan chan struct{}
}

func getSharedTicker(b *Balancer, refreshInterval time.Duration) *sharedTicker {
	sti, ok := sharedTickers.Load(refreshInterval)
	if ok {
		st := sti.(*sharedTicker)
		st.add(b)
		return st
	}
	v, _, _ := sharedTickersSfg.Do(refreshInterval.String(), func() (interface{}, error) {
		st := &sharedTicker{
			interval: refreshInterval,
			tasks:    map[*Balancer]struct{}{},
			stopChan: make(chan struct{}, 1),
		}
		sharedTickers.Store(refreshInterval, st)
		return st, nil
	})
	st := v.(*sharedTicker)
	// add without singleflight,
	// because we need all balancers those call this function to add themself to sharedTicker
	st.add(b)
	return st
}

func (t *sharedTicker) add(b *Balancer) {
	t.Lock()
	defer t.Unlock()
	// add task
	t.tasks[b] = struct{}{}
	if !t.started {
		t.started = true
		go t.tick(t.interval)
	}
}

func (t *sharedTicker) delete(b *Balancer) {
	t.Lock()
	defer t.Unlock()
	// delete from tasks
	delete(t.tasks, b)
	// no tasks remaining then stop the tick
	if len(t.tasks) == 0 {
		// unblocked when multi delete call
		select {
		case t.stopChan <- struct{}{}:
			t.started = false
		default:
		}
	}
}

func (t *sharedTicker) tick(interval time.Duration) {
	var wg sync.WaitGroup
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.Lock()
			for b := range t.tasks {
				wg.Add(1)
				go func(b *Balancer) {
					defer wg.Done()
					b.refresh()
				}(b)
			}
			wg.Wait()
			t.Unlock()
		case <-t.stopChan:
			return
		}
	}
}
