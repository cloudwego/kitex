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

package protobuf

import (
	"reflect"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func Test_pbError_TypeID(t *testing.T) {
	newPbError := NewPbError(2, "find an error")
	id := newPbError.TypeID()
	test.Assert(t, id == 2)
}

// test NewPbError with reflect
func TestNewPbError(t *testing.T) {
	err := &ErrorProto{TypeID: 1, Message: "hello world"}
	proto := &pbError{errProto: err}
	newPbError := NewPbError(1, "hello world")
	test.Assert(t, reflect.DeepEqual(proto, newPbError))
}

func Test_pbError_Error(t *testing.T) {
	test1 := "test1"
	newPbError := NewPbError(3, test1)
	test.Assert(t, newPbError.Error() == test1)
}
