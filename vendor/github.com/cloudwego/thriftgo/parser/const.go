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

package parser

// TType constants in the Thrift protocol.
const (
	STOP   = 0
	VOID   = 1
	BOOL   = 2
	BYTE   = 3
	I08    = 3
	DOUBLE = 4
	I16    = 6
	I32    = 8
	I64    = 10
	STRING = 11
	UTF7   = 11
	STRUCT = 12
	MAP    = 13
	SET    = 14
	LIST   = 15
	UTF8   = 16
	UTF16  = 17
	BINARY = 18

	UNKNOWN = 255
)

var typename2TypeID = map[string]uint8{
	"bool":   BOOL,
	"byte":   BYTE,
	"i8":     I08,
	"i16":    I16,
	"i32":    I32,
	"i64":    I64,
	"double": DOUBLE,
	"string": STRING,
	"binary": BINARY,
	"map":    MAP,
	"set":    SET,
	"list":   LIST,
}

// Typename2TypeID converts a TypeID name to its value.
func Typename2TypeID(name string) uint8 {
	if id := typename2TypeID[name]; id != 0 {
		return id
	}
	return UNKNOWN
}
