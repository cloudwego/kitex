/*
 * Copyright 2023 CloudWeGo Authors
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

package loadbalance

import (
	"runtime"
	"sync"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestRoughRoundOneThread(t *testing.T) {
	p := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(p)

	rr := newRoughRound()
	var first = rr.Next()
	var last uint64
	var curr uint64
	for i := 0; i < 1000; i++ {
		curr = rr.Next()
		if last > 0 && curr-last != 1 {
			t.Logf("May be goroutine change P? %d-%d=%d", curr, last, curr-last)
		}
		last = curr
	}
	test.Assert(t, last-first == 1000, first, last)
}

func TestRoughRoundMultiThread(t *testing.T) {
	rr := newRoughRound()
	var wg sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var last uint64
			var curr uint64
			for j := 0; j < 10000; j++ {
				curr = rr.Next()
				if last > 0 && curr-last != 1 {
					t.Logf("May be goroutine change P? %d-%d=%d", curr, last, curr-last)
				}
				last = curr
			}
		}()
	}
	wg.Wait()
}

func BenchmarkIterator(b *testing.B) {
	testcases := []struct {
		Name    string
		NewFunc func() iterator
	}{
		{Name: "Round", NewFunc: newRound},
		{Name: "RoughRound", NewFunc: newRoughRound},
	}
	for _, tc := range testcases {
		var r = tc.NewFunc()
		b.Run(tc.Name, func(b *testing.B) {
			var last uint64
			for i := 0; i < b.N; i++ {
				last = r.Next()
			}
			_ = last
		})
	}
}

func BenchmarkIteratorParallel(b *testing.B) {
	testcases := []struct {
		Name    string
		NewFunc func() iterator
	}{
		{Name: "Round", NewFunc: newRound},
		{Name: "RoughRound", NewFunc: newRoughRound},
	}
	for _, tc := range testcases {
		var r = tc.NewFunc()
		b.Run(tc.Name, func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				var last uint64
				for pb.Next() {
					last = r.Next()
				}
				_ = last
			})
		})
	}
}
