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
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

var _ Object = (*testObject)(nil)

type testObject struct {
	closeCallback func()
}

func (o *testObject) Close(exception error) error {
	if o.closeCallback != nil {
		o.closeCallback()
	}
	return nil
}

func TestObjectPool(t *testing.T) {
	op := NewObjectPool(time.Microsecond * 10)
	count := 10000
	key := "test"
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		o := new(testObject)
		o.closeCallback = func() {
			wg.Done()
		}
		wg.Add(1)
		op.Push(key, o)
	}
	wg.Wait()
	test.Assert(t, op.objects[key].Size() == 0)
	op.Close()

	op = NewObjectPool(time.Second)
	var deleted int32
	for i := 0; i < count; i++ {
		o := new(testObject)
		o.closeCallback = func() {
			atomic.AddInt32(&deleted, 1)
		}
		op.Push(key, o)
	}
	test.Assert(t, atomic.LoadInt32(&deleted) == 0)
	test.Assert(t, op.objects[key].Size() == count)
	op.Close()
}

func TestObjectPool_cleaningLazyInit(t *testing.T) {
	// wait for all cleaning goroutines created by other tests finished
	for cleaningGoroutineExist() {
		time.Sleep(10 * time.Millisecond)
	}
	op := NewObjectPool(10 * time.Microsecond)
	defer op.Close()
	if cleaningGoroutineExist() {
		t.Fatal("cleaning goroutine should not be started when ObjectPool.Push is not invoked")
	}
	op.Push("test", new(testObject))
	select {
	case <-func() chan struct{} {
		c := make(chan struct{})
		go func() {
			if !cleaningGoroutineExist() {
				time.Sleep(10 * time.Millisecond)
			}
			close(c)
		}()
		return c
	}():
	case <-time.After(time.Second):
		t.Fatal("cleaning goroutine should be started when ObjectPool.Push is invoked")
	}
}

func cleaningGoroutineExist() bool {
	buf := make([]byte, 2<<20)
	buf = buf[:runtime.Stack(buf, true)]
	for _, g := range strings.Split(string(buf), "\n\n") {
		if strings.Contains(g, "(*ObjectPool).cleaning") {
			return true
		}
	}
	return false
}
