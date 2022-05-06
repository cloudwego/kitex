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
	"github.com/cloudwego/thriftgo/parser"
)

var typeids = struct {
	Bool   string
	Byte   string
	I8     string
	I16    string
	I32    string
	I64    string
	Double string
	String string
	Binary string
	Set    string
	List   string
	Map    string
	Struct string
}{
	Bool:   "Bool",
	Byte:   "Byte",
	I8:     "Byte", // i8 is byte
	I16:    "I16",
	I32:    "I32",
	I64:    "I64",
	Double: "Double",
	String: "String",
	Binary: "Binary",
	Set:    "Set",
	List:   "List",
	Map:    "Map",
	Struct: "Struct",
}

var category2TypeID = map[parser.Category]string{
	parser.Category_Bool:      "Bool",
	parser.Category_Byte:      "Byte", // i8 is byte
	parser.Category_I16:       "I16",
	parser.Category_I32:       "I32",
	parser.Category_I64:       "I64",
	parser.Category_Double:    "Double",
	parser.Category_String:    "String",
	parser.Category_Binary:    "Binary",
	parser.Category_Map:       "Map",
	parser.Category_List:      "List",
	parser.Category_Set:       "Set",
	parser.Category_Enum:      "I32",
	parser.Category_Struct:    "Struct",
	parser.Category_Union:     "Struct",
	parser.Category_Exception: "Struct",
}

var baseTypes = map[string]string{
	"bool":   "bool",
	"byte":   "int8",
	"i8":     "int8",
	"i16":    "int16",
	"i32":    "int32",
	"i64":    "int64",
	"double": "float64",
	"string": "string",
	"binary": "[]byte",
}

var isContainerTypes = map[string]bool{"map": true, "set": true, "list": true}

var isKeywords = map[string]bool{
	"break":       true,
	"default":     true,
	"func":        true,
	"interface":   true,
	"select":      true,
	"case":        true,
	"defer":       true,
	"go":          true,
	"map":         true,
	"struct":      true,
	"chan":        true,
	"else":        true,
	"goto":        true,
	"package":     true,
	"switch":      true,
	"const":       true,
	"fallthrough": true,
	"if":          true,
	"range":       true,
	"type":        true,
	"continue":    true,
	"for":         true,
	"import":      true,
	"return":      true,
	"var":         true,
}
