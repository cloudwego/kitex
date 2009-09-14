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
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

var _ PoolObject = &mockPoolObject{}

type mockPoolObject struct {
	active   bool
	deadline time.Time
}

func (o *mockPoolObject) SetDeadline(t time.Time) error {
	o.deadline = t
	return nil
}

func (o *mockPoolObject) IsActive() bool {
	return o.active
}

func (o *mockPoolObject) Close() error {
	return nil
}

func (o *mockPoolObject) Dump() interface{} {
	return o.deadline
}

func (o *mockPoolObject) Expired() bool {
	return time.Now().After(o.deadline)
}

func mockObjectNewer() (PoolObject, error) {
	return &mockPoolObject{active: true}, nil
}

func TestPool_GetPut(t *testing.T) {
	var (
		minIdle        = 0
		maxIdle        = 1
		maxIdleTimeout = time.Millisecond
	)

	pool := NewPool(minIdle, maxIdle, maxIdleTimeout)

	obj, reused, err := pool.Get(mockObjectNewer)
	test.Assert(t, obj != nil)
	test.Assert(t, reused == false)
	test.Assert(t, err == nil)

	recycled := pool.Put(obj)
	test.Assert(t, recycled == true)
	test.Assert(t, pool.Len() == 1)
}

func TestPool_Reuse(t *testing.T) {
	var (
		minIdle        = 0
		maxIdle        = 1
		maxIdleTimeout = time.Millisecond
	)

	pool := NewPool(minIdle, maxIdle, maxIdleTimeout)

	count := make(map[PoolObject]bool)
	obj, reused, err := pool.Get(mockObjectNewer)
	test.Assert(t, obj != nil)
	test.Assert(t, reused == false)
	test.Assert(t, err == nil)
	count[obj] = true

	recycled := pool.Put(obj)
	test.Assert(t, recycled == true)
	test.Assert(t, pool.Len() == 1)

	obj, reused, err = pool.Get(mockObjectNewer)
	test.Assert(t, obj != nil)
	test.Assert(t, reused == true)
	test.Assert(t, err == nil)
	count[obj] = true

	test.Assert(t, len(count) == 1)
}

func TestPool_MaxIdle(t *testing.T) {
	var (
		minIdle        = 0
		maxIdle        = 2
		maxIdleTimeout = time.Millisecond
	)

	pool := NewPool(minIdle, maxIdle, maxIdleTimeout)

	var objs []PoolObject
	for i := 0; i < maxIdle+1; i++ {
		obj, reused, err := pool.Get(mockObjectNewer)
		test.Assert(t, obj != nil)
		test.Assert(t, reused == false)
		test.Assert(t, err == nil)
		objs = append(objs, obj)
	}

	for i := 0; i < maxIdle+1; i++ {
		recycled := pool.Put(objs[i])
		if i < maxIdle {
			test.Assert(t, recycled == true)
		} else {
			test.Assert(t, recycled == false)
		}
	}

	test.Assert(t, pool.Len() == maxIdle)
}

func TestPool_MinIdle(t *testing.T) {
	var (
		minIdle        = 1
		maxIdle        = 10
		maxIdleTimeout = time.Millisecond
	)

	pool := NewPool(minIdle, maxIdle, maxIdleTimeout)

	var objs []PoolObject
	for i := 0; i < maxIdle; i++ {
		obj, reused, err := pool.Get(mockObjectNewer)
		test.Assert(t, obj != nil)
		test.Assert(t, reused == false)
		test.Assert(t, err == nil)
		objs = append(objs, obj)
	}
	for i := range objs {
		pool.Put(objs[i])
	}

	test.Assert(t, pool.Len() == maxIdle)
	time.Sleep(maxIdleTimeout)
	pool.Evict()
	test.Assert(t, pool.Len() == minIdle)
}

func TestPool_Dump(t *testing.T) {
	var (
		minIdle        = 0
		maxIdle        = 2
		maxIdleTimeout = time.Millisecond
	)

	pool := NewPool(minIdle, maxIdle, maxIdleTimeout)
	obj, _, _ := pool.Get(mockObjectNewer)
	pool.Put(obj)

	dump := pool.Dump()
	test.Assert(t, dump.IdleNum == 1)
	test.Assert(t, len(dump.IdleList) == 1)
}
