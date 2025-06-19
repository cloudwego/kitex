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

package retry

import (
	"context"
	"reflect"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func mockRPCInfo(retryTag string) rpcinfo.RPCInfo {
	var tags map[string]string
	if retryTag != "" {
		tags = map[string]string{
			rpcinfo.RetryTag: retryTag,
		}
	}
	to := rpcinfo.NewEndpointInfo("service", "method", nil, tags)
	return rpcinfo.NewRPCInfo(nil, to, nil, nil, nil)
}

func mockContext(retryTag string) context.Context {
	return rpcinfo.NewCtxWithRPCInfo(context.TODO(), mockRPCInfo(retryTag))
}

func TestIsLocalRetryRequest(t *testing.T) {
	t.Run("no-retry-tag", func(t *testing.T) {
		test.Assertf(t, !IsLocalRetryRequest(mockContext("")), "no-retry-tag")
	})
	t.Run("retry-tag=0", func(t *testing.T) {
		test.Assertf(t, !IsLocalRetryRequest(mockContext("0")), "retry-tag=0")
	})
	t.Run("retry-tag=1", func(t *testing.T) {
		test.Assertf(t, IsLocalRetryRequest(mockContext("1")), "retry-tag=1")
	})
}

func TestIsRemoteRetryRequest(t *testing.T) {
	t.Run("no-retry", func(t *testing.T) {
		test.Assertf(t, !IsRemoteRetryRequest(context.Background()), "should be not retry")
	})
	t.Run("retry", func(t *testing.T) {
		ctx := metainfo.WithPersistentValue(context.Background(), TransitKey, "2")
		test.Assertf(t, IsRemoteRetryRequest(ctx), "should be retry")
	})
}

type XXXResult struct {
	str string
}

func TestNewStructPointer(t *testing.T) {
	type MyStruct struct{ Val int }

	t.Run("normal struct pointer", func(t *testing.T) {
		original := &MyStruct{Val: 10}
		copied := NewStructPointer(original)

		if reflect.TypeOf(copied) != reflect.TypeOf(original) {
			t.Errorf("Type mismatch, got %T, want %T", copied, original)
		}
		if copied == original {
			t.Error("Should create new instance, but got same pointer")
		}
	})

	t.Run("nil input", func(t *testing.T) {
		if got := NewStructPointer(nil); got != nil {
			t.Errorf("Expected nil, got %v", got)
		}
	})

	t.Run("non-pointer input panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic but didn't get one")
			}
		}()

		NewStructPointer(MyStruct{Val: 10}) // Should panic
	})
}

func Test_ShallowCopyStructPointer(t *testing.T) {
	type SampleStruct struct {
		Field1     string
		Field2     int
		innerField int
	}

	t.Run("src is nil", func(t *testing.T) {
		dst := &SampleStruct{Field1: "test", Field2: 123}
		ShallowCopyStructPointer(nil, dst)
		if dst == nil || dst.Field1 != "test" || dst.Field2 != 123 {
			t.Errorf("Expected dst to remain unchanged, got %v", dst)
		}
	})

	t.Run("inner field is copied", func(t *testing.T) {
		src := &SampleStruct{Field1: "source", Field2: 456, innerField: 789}
		dst := &SampleStruct{Field1: "test", Field2: 123}
		ShallowCopyStructPointer(src, dst)
		if dst.Field1 != "source" || dst.Field2 != 456 || dst.innerField != 789 {
			t.Errorf("Expected dst to be updated to src values, got %v", dst)
		}
	})

	t.Run("src and dst are of different types", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic but didn't get one")
			}
		}()
		src := &SampleStruct{Field1: "source", Field2: 456}
		var dst = &struct {
			Field1 string
			Field2 int
			Field3 bool
		}{Field1: "test", Field2: 123, Field3: true}
		ShallowCopyStructPointer(src, dst)
		if dst.Field1 != "source" || dst.Field2 != 456 || !dst.Field3 {
			t.Errorf("Expected dst to be partially updated to src values, got %v", dst)
		}
	})
}
