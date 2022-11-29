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

package thrift

import (
	"reflect"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
)

func TestSplitType(t *testing.T) {
	pkg, name := splitType(".A")
	test.Assert(t, pkg == "")
	test.Assert(t, name == "A")

	pkg, name = splitType("foo.bar.A")
	test.Assert(t, pkg == "foo.bar")
	test.Assert(t, name == "A")

	pkg, name = splitType("A")
	test.Assert(t, pkg == "")
	test.Assert(t, name == "A")

	pkg, name = splitType("")
	test.Assert(t, pkg == "")
	test.Assert(t, name == "")
}

func TestBuildinTypeFromString(t *testing.T) {
	val, err := buildinTypeFromString("", &descriptor.TypeDescriptor{Type: descriptor.BOOL})
	test.Assert(t, err == nil)
	test.Assert(t, val == false)
	val, err = buildinTypeFromString("", &descriptor.TypeDescriptor{Type: descriptor.I08})
	test.Assert(t, err == nil)
	test.Assert(t, val == int8(0))
	val, err = buildinTypeFromString("", &descriptor.TypeDescriptor{Type: descriptor.I16})
	test.Assert(t, err == nil)
	test.Assert(t, val == int16(0))
	val, err = buildinTypeFromString("", &descriptor.TypeDescriptor{Type: descriptor.I32})
	test.Assert(t, err == nil)
	test.Assert(t, val == int32(0))
	val, err = buildinTypeFromString("", &descriptor.TypeDescriptor{Type: descriptor.I64})
	test.Assert(t, err == nil)
	test.Assert(t, val == int64(0))
	val, err = buildinTypeFromString("", &descriptor.TypeDescriptor{Type: descriptor.DOUBLE})
	test.Assert(t, err == nil)
	test.Assert(t, val == float64(0))
	val, err = buildinTypeFromString("", &descriptor.TypeDescriptor{Type: descriptor.STRING})
	test.Assert(t, err == nil)
	test.Assert(t, val == "")
	val, err = buildinTypeFromString("", &descriptor.TypeDescriptor{Type: descriptor.LIST, Elem: &descriptor.TypeDescriptor{Type: descriptor.I64}})
	test.Assert(t, err == nil)
	test.Assert(t, reflect.DeepEqual(val, []interface{}{int64(0)}))
}
