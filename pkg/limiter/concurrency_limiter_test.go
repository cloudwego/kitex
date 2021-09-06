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

package limiter

import (
	"sync"
	"testing"
	"time"

	"github.com/jackedelic/kitex/internal/test"
)

func TestConLimiter(t *testing.T) {
	lim := NewConcurrencyLimiter(50)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			_, now := lim.Status()
			test.Assert(t, now <= 50)
			time.Sleep(time.Millisecond / 10)
		}
	}()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				if lim.Acquire() {
					time.Sleep(time.Millisecond)
					lim.Release()
				} else {
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
}
