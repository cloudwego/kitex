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

package utils

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestNewRing(t *testing.T) {
	r := NewRing(10)
	test.Assert(t, r != nil)

	r = NewRing(0)
	test.Assert(t, r != nil)

	r = NewRing(-1)
	test.Assert(t, r != nil)
}

func TestRing_Push(t *testing.T) {
	var err error
	// size > 0
	r := NewRing(10)
	for i := 0; i < 20; i++ {
		err = r.Push(i)

		if i < 10 {
			test.Assert(t, err == nil)
		} else {
			test.Assert(t, err != nil)
		}
	}

	// size == 0
	r = NewRing(0)
	err = r.Push(1)
	test.Assert(t, err == ErrRingFull)

	// size < 0
	r = NewRing(-1)
	err = r.Push(1)
	test.Assert(t, err == ErrRingFull)
}

func TestRing_Pop(t *testing.T) {
	// size > 0
	r := NewRing(10)
	for i := 0; i < 10; i++ {
		err := r.Push(i)
		test.Assert(t, err == nil)
	}

	for i := 0; i < 20; i++ {
		if i < 10 {
			elem := r.Pop()
			test.Assert(t, elem != nil)
		} else {
			elem := r.Pop()
			test.Assert(t, elem == nil)
		}
	}

	// size == 0
	r = NewRing(0)
	test.Assert(t, r.Pop() == nil)

	// size < 0
	r = NewRing(-1)
	test.Assert(t, r.Pop() == nil)
}

func TestRing_Parallel(t *testing.T) {
	r := NewRing(10)
	var wg sync.WaitGroup

	flag := make([]bool, 100)
	var errCount int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			if err := r.Push(v); err != nil {
				atomic.AddInt64(&errCount, 1)
			} else {
				flag[v] = true
			}
		}(i)
	}
	wg.Wait()
	test.Assert(t, errCount == 90)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if x := r.Pop(); x != nil {
				j, ok := x.(int)
				test.Assert(t, ok)
				test.Assert(t, flag[j])
			}
		}()
	}
	wg.Wait()
}

type counter struct{ value int }

func (c *counter) count()      { c.value++ }
func (c *counter) result() int { return c.value }

func TestRing_Race(t *testing.T) {
	size, p, times := 10, 100, 10000
	r := NewRing(size)
	var wg sync.WaitGroup

	for i := 0; i < size; i++ {
		err := r.Push(new(counter))
		test.Assert(t, err == nil, i, err)
	}
	for i := 0; i < p; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tmp := 0
			for j := 0; j < times; j++ {
				for {
					c, ok := r.Pop().(*counter)
					if ok && c != nil {
						tmp += c.result()
						c.count()
						err := r.Push(c)
						test.Assert(t, err == nil)
						break
					}
					runtime.Gosched()
				}
			}
		}()
	}
	wg.Wait()

	sum := 0
	for i := 0; i < size; i++ {
		c, ok := r.Pop().(*counter)
		test.Assert(t, ok, i)
		sum += c.result()
	}
	test.Assert(t, sum == p*times, sum)
}
