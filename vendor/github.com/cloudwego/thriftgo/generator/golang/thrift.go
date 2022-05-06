// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package golang

import (
	"fmt"
	"strings"

	"github.com/cloudwego/thriftgo/parser"
)

// GetTypeIDConstant returns the thrift type ID literal for the given type which
// is suitable to concate with "thrift." to produce a valid type ID constant.
func GetTypeIDConstant(t *parser.Type) string {
	tid := GetTypeID(t)
	tid = strings.ToUpper(tid)
	if tid == "BINARY" {
		tid = "STRING"
	}
	return tid
}

// GetTypeID returns the thrift type ID literal for the given type which is suitable
// to concate with "Read" or "Write" to produce a valid method name in the TProtocol
// interface. Note that enum types results in I32.
func GetTypeID(t *parser.Type) string {
	// Bool|Byte|I16|I32|I64|Double|String|Binary|Set|List|Map|Struct
	return category2TypeID[t.Category]
}

// IsBaseType determines whether the given type is a base type.
func IsBaseType(t *parser.Type) bool {
	if t.Category.IsBaseType() || t.Category == parser.Category_Enum {
		return true
	}
	if t.Category == parser.Category_Typedef {
		panic(fmt.Sprintf("unexpected typedef category: %+v", t))
	}
	return false
}

// NeedRedirect deterimines whether the given field should result in a pointer type.
// Condition: struct-like || (optional non-binary base type without default vlaue).
func NeedRedirect(f *parser.Field) bool {
	if f.Type.Category.IsStructLike() {
		return true
	}

	if f.Requiredness.IsOptional() && !f.IsSetDefault() {
		if f.Type.Category.IsBinary() {
			// binary types produce slice types
			return false
		}
		return IsBaseType(f.Type)
	}

	return false
}

// IsConstantInGo tells whether a constant in thrift IDL results in a constant in go.
func IsConstantInGo(v *parser.Constant) bool {
	c := v.Type.Category
	if c.IsBaseType() && c != parser.Category_Binary {
		return true
	}
	return c == parser.Category_Enum
}

// IsFixedLengthType determines whether the given type is a fixed length type.
func IsFixedLengthType(t *parser.Type) bool {
	return parser.Category_Bool <= t.Category && t.Category <= parser.Category_Double
}

// SupportIsSet determines whether a field supports IsSet query.
func SupportIsSet(f *parser.Field) bool {
	return f.Type.Category.IsStructLike() || f.Requiredness.IsOptional()
}
