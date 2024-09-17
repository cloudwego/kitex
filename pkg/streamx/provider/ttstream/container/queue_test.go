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

package container

import (
	"sync"
	"testing"
)

func TestQueue(t *testing.T) {
	locker := sync.RWMutex{}
	q := NewQueueWithLocker[int](&locker)
	round := 100000
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < round; i++ {
			q.Add(1)
		}
	}()
	for sum := 0; sum < round; {
		v, ok := q.Get()
		if ok {
			sum += v
		}
	}
}
