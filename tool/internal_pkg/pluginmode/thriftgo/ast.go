// Copyright 2023 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package thriftgo

import (
	"github.com/cloudwego/thriftgo/generator/golang"
	"github.com/cloudwego/thriftgo/parser"
)

func offsetTPL(assign string, offset string) string {
	return offset + " += " + assign + "\n"
}

func ZeroWriter(t *parser.Type, oprot string, buf string, offset string) string {
	switch t.GetCategory() {
	case parser.Category_Bool:
		return offsetTPL(oprot+".WriteBool("+buf+", false)", offset)
	case parser.Category_Byte:
		return offsetTPL(oprot+".WriteByte("+buf+", 0)", offset)
	case parser.Category_I16:
		return offsetTPL(oprot+".WriteI16("+buf+", 0)", offset)
	case parser.Category_Enum, parser.Category_I32:
		return offsetTPL(oprot+".WriteI32("+buf+", 0)", offset)
	case parser.Category_I64:
		return offsetTPL(oprot+".WriteI64("+buf+", 0)", offset)
	case parser.Category_Double:
		return offsetTPL(oprot+".WriteDouble("+buf+", 0)", offset)
	case parser.Category_String:
		return offsetTPL(oprot+".WriteString("+buf+", \"\")", offset)
	case parser.Category_Binary:
		return offsetTPL(oprot+".WriteBinary("+buf+", []byte{})", offset)
	case parser.Category_Map:
		return offsetTPL(oprot+".WriteMapBegin("+buf+", thrift."+golang.GetTypeIDConstant(t.GetKeyType())+
			",thrift."+golang.GetTypeIDConstant(t.GetValueType())+",0)", offset) + offsetTPL(oprot+".WriteMapEnd("+buf+")", offset)
	case parser.Category_List:
		return offsetTPL(oprot+".WriteListBegin("+buf+", thrift."+golang.GetTypeIDConstant(t.GetValueType())+
			",0)", offset) + offsetTPL(oprot+".WriteListEnd("+buf+")", offset)
	case parser.Category_Set:
		return offsetTPL(oprot+".WriteSetBegin("+buf+", thrift."+golang.GetTypeIDConstant(t.GetValueType())+
			",0)", offset) + offsetTPL(oprot+".WriteSetEnd("+buf+")", offset)
	case parser.Category_Struct:
		return offsetTPL(oprot+".WriteStructBegin("+buf+", \"\")", offset) + offsetTPL(oprot+".WriteFieldStop("+buf+")", offset) +
			offsetTPL(oprot+".WriteStructEnd("+buf+")", offset)
	default:
		panic("unsuported type zero writer for" + t.Name)
	}
}

func ZeroBLength(t *parser.Type, oprot string, offset string) string {
	switch t.GetCategory() {
	case parser.Category_Bool:
		return offsetTPL(oprot+".BoolLength(false)", offset)
	case parser.Category_Byte:
		return offsetTPL(oprot+".ByteLength(0)", offset)
	case parser.Category_I16:
		return offsetTPL(oprot+".I16Length(0)", offset)
	case parser.Category_Enum, parser.Category_I32:
		return offsetTPL(oprot+".I32Length(0)", offset)
	case parser.Category_I64:
		return offsetTPL(oprot+".I64Length(0)", offset)
	case parser.Category_Double:
		return offsetTPL(oprot+".DoubleLength(0)", offset)
	case parser.Category_String:
		return offsetTPL(oprot+".StringLength(\"\")", offset)
	case parser.Category_Binary:
		return offsetTPL(oprot+".BinaryLength([]byte{})", offset)
	case parser.Category_Map:
		return offsetTPL(oprot+".MapBeginLength(thrift."+golang.GetTypeIDConstant(t.GetKeyType())+
			",thrift."+golang.GetTypeIDConstant(t.GetValueType())+", 0)", offset) + offsetTPL(oprot+".MapEndLength()", offset)
	case parser.Category_List:
		return offsetTPL(oprot+".ListBeginLength(thrift."+golang.GetTypeIDConstant(t.GetValueType())+
			",0)", offset) + offsetTPL(oprot+".ListEndLength()", offset)
	case parser.Category_Set:
		return offsetTPL(oprot+".SetBeginLength(thrift."+golang.GetTypeIDConstant(t.GetValueType())+
			",0)", offset) + offsetTPL(oprot+".SetEndLength()", offset)
	case parser.Category_Struct:
		return offsetTPL(oprot+".StructBeginLength(\"\")", offset) + offsetTPL(oprot+".FieldStopLength()", offset) +
			offsetTPL(oprot+".StructEndLength()", offset)
	default:
		panic("unsuported type zero writer for" + t.Name)
	}
}
