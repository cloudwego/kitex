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

package event

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestQueueInvalidCapacity(t *testing.T) {
	defer func() {
		e := recover()
		test.Assert(t, e != nil)
	}()
	NewQueue(-1)
}

func TestQueue(t *testing.T) {
	size := 10
	q := NewQueue(size)

	for i := 0; i < size; i++ {
		n := "E." + strconv.Itoa(i)
		q.Push(&Event{
			Name: n,
		})
		es := q.Dump().([]*Event)
		test.Assert(t, len(es) == i+1)
		test.Assert(t, es[0].Name == n)
	}

	for i := 0; i < size; i++ {
		q.Push(&Event{
			Name: strconv.Itoa(i),
		})
		es := q.Dump().([]*Event)
		test.Assert(t, len(es) == size)
	}
}

var _ lazyExtra = (*mockLazyExtra)(nil)

// mockLazyExtra implements lazyExtra interface
type mockLazyExtra struct {
	data string
}

func (m *mockLazyExtra) KitexDumpLazyExtra() interface{} {
	return map[string]interface{}{"data": m.data}
}

func TestQueueLazyExtra(t *testing.T) {
	q := NewQueue(5)

	q.Push(&Event{Name: "plain", Extra: "hello"})
	q.Push(&Event{Name: "lazy", Extra: &mockLazyExtra{data: "world"}})

	es := q.Dump().([]*Event)
	test.Assert(t, len(es) == 2)

	test.Assert(t, es[0].Name == "lazy")
	m, ok := es[0].Extra.(map[string]interface{})
	test.Assert(t, ok, es[0].Extra)
	test.Assert(t, m["data"] == "world", m)

	test.Assert(t, es[1].Name == "plain")
	test.Assert(t, es[1].Extra == "hello")
}

func TestQueueLazyExtraConcurrent(t *testing.T) {
	t.Run("PushAndDump", func(t *testing.T) {
		q := NewQueue(50)
		var failures atomic.Int64
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				q.Push(&Event{Name: "lazy", Extra: &mockLazyExtra{data: strconv.Itoa(i)}})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				es := q.Dump().([]*Event)
				for _, e := range es {
					if _, ok := e.Extra.(map[string]interface{}); !ok {
						failures.Add(1)
					}
				}
			}
		}()
		wg.Wait()
		test.Assert(t, failures.Load() == 0, failures.Load())
	})
	t.Run("PushMixed", func(t *testing.T) {
		q := NewQueue(50)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				q.Push(&Event{Name: "lazy", Extra: &mockLazyExtra{data: strconv.Itoa(i)}})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				q.Push(&Event{Name: "plain", Extra: "hello"})
			}
		}()
		wg.Wait()

		es := q.Dump().([]*Event)
		test.Assert(t, len(es) == 50)
		for _, e := range es {
			switch e.Name {
			case "lazy":
				_, ok := e.Extra.(map[string]interface{})
				test.Assert(t, ok, e.Extra)
			case "plain":
				test.Assert(t, e.Extra == "hello")
			default:
				t.Fatalf("unexpected event name: %s", e.Name)
			}
		}
	})
	t.Run("DumpDoesNotMutateRing", func(t *testing.T) {
		q := NewQueue(50)
		for i := 0; i < 50; i++ {
			q.Push(&Event{Name: "lazy", Extra: &mockLazyExtra{data: strconv.Itoa(i)}})
		}
		var failures atomic.Int64
		var wg sync.WaitGroup
		wg.Add(3)
		for g := 0; g < 3; g++ {
			go func() {
				defer wg.Done()
				for i := 0; i < 500; i++ {
					es := q.Dump().([]*Event)
					if len(es) != 50 {
						failures.Add(1)
					}
				}
			}()
		}
		wg.Wait()
		test.Assert(t, failures.Load() == 0, failures.Load())
	})
}

func Test_DefaultEventNum(t *testing.T) {
	initNum := GetDefaultEventNum()
	defer SetDefaultEventNum(initNum)

	test.Assert(t, initNum == MaxEventNum, initNum)
	// negative num does not take effect
	SetDefaultEventNum(-1)
	curNum := GetDefaultEventNum()
	test.Assert(t, curNum == initNum, curNum)
	// 0 does not take effect
	SetDefaultEventNum(0)
	curNum = GetDefaultEventNum()
	test.Assert(t, curNum == initNum, curNum)

	SetDefaultEventNum(100)
	curNum = GetDefaultEventNum()
	test.Assert(t, curNum == 100, curNum)
}

func BenchmarkQueue(b *testing.B) {
	q := NewQueue(100)
	e := &Event{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(e)
	}
}

func BenchmarkQueueConcurrent(b *testing.B) {
	q := NewQueue(100)
	e := &Event{}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			q.Push(e)
		}
	})
	b.StopTimer()
}
