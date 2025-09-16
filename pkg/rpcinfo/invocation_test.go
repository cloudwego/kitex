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

package rpcinfo

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestInvocation(t *testing.T) {
	t.Run("with package", func(t *testing.T) {
		inv := NewInvocation("a.a.a", "foo", "bar")
		test.Assert(t, inv.PackageName() == "bar")
	})
	t.Run("without package", func(t *testing.T) {
		inv := NewInvocation("a.a.a", "foo")
		test.Assert(t, inv.PackageName() == "")
	})

	inv := NewInvocation("a.a.a", "aaa")
	test.Assert(t, inv.SeqID() == globalSeqID)

	var set InvocationSetter = inv

	set.SetSeqID(111)
	set.SetPackageName("pkg")
	set.SetServiceName("svc")
	set.SetMethodName("mtd")
	test.Assert(t, inv.SeqID() == 111)
	test.Assert(t, inv.PackageName() == "pkg")
	test.Assert(t, inv.ServiceName() == "svc")
	test.Assert(t, inv.MethodName() == "mtd")

	globalSeqID = -1
	inv = NewInvocation("a.a.a", "aaa")
	test.Assert(t, inv.SeqID() == 1, inv.SeqID())

	sIvk := NewServerInvocation()
	test.Assert(t, sIvk.SeqID() == 0)
}

func TestInvocation_Extra(t *testing.T) {
	keyNonAtomic1 := RegisterInvocationExtraKey("non_atomic_1", false)
	keyAtomic1 := RegisterInvocationExtraKey("atomic_1", true)
	keyNonAtomic2 := RegisterInvocationExtraKey("non_atomic_2", false)
	keyAtomic2 := RegisterInvocationExtraKey("atomic_2", true)
	keyConcurrent := RegisterInvocationExtraKey("concurrent_key", true)

	inv := invocationPool.Get().(*invocation)
	defer invocationPool.Put(inv)
	inv.Reset()

	valStr := "hello world"
	inv.SetExtraInfo(keyNonAtomic1, valStr)
	res1 := inv.ExtraInfo(keyNonAtomic1)
	test.Assert(t, res1 == valStr)

	valInt := 12345
	inv.SetExtraInfo(keyAtomic1, valInt)
	res2 := inv.ExtraInfo(keyAtomic1)
	test.Assert(t, res2 == valInt)

	res3 := inv.ExtraInfo(keyNonAtomic2)
	test.Assert(t, res3 == nil)
	res4 := inv.ExtraInfo(keyAtomic2)
	test.Assert(t, res4 == nil)

	newValStr := "new hello"
	inv.SetExtraInfo(keyNonAtomic1, newValStr)
	res5 := inv.ExtraInfo(keyNonAtomic1)
	test.Assert(t, res5 == newValStr)
	newValInt := 54321
	inv.SetExtraInfo(keyAtomic1, newValInt)
	res6 := inv.ExtraInfo(keyAtomic1)
	test.Assert(t, res6 == newValInt)

	inv.Reset()
	res7 := inv.ExtraInfo(keyNonAtomic1)
	test.Assert(t, res7 == nil)
	res8 := inv.ExtraInfo(keyAtomic1)
	test.Assert(t, res8 == nil)

	var wg sync.WaitGroup
	numGoroutines := 20

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(v int) {
			defer wg.Done()
			inv.SetExtraInfo(keyConcurrent, fmt.Sprintf("value-%d", v))
		}(i)
	}
	wg.Wait()

	finalValue, ok := inv.ExtraInfo(keyConcurrent).(string)
	test.Assert(t, ok)
	test.Assert(t, strings.HasPrefix(finalValue, "value-"))

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			readVal := inv.ExtraInfo(keyConcurrent)
			test.Assert(t, readVal == finalValue)
		}()
	}
	wg.Wait()
}
