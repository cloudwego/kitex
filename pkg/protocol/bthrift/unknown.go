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

package bthrift

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/thriftgo/generator/golang/extension/unknown"
)

// UnknownField is used to describe an unknown field.
type UnknownField struct {
	Name    string
	ID      int16
	Type    int
	KeyType int
	ValType int
	Value   interface{}
}

// GetUnknownFields deserialize unknownFields stored in v to a list of *UnknownFields.
func GetUnknownFields(v interface{}) (fields []UnknownField, err error) {
	var buf []byte
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%T is not a struct type", v)
	}
	if unknownField := rv.FieldByName("_unknownFields"); !unknownField.IsValid() {
		return nil, fmt.Errorf("%T has no field named '_unknownFields'", v)
	} else {
		buf = unknownField.Bytes()
	}
	return ConvertUnknownFields(buf)
}

// ConvertUnknownFields converts buf to deserialized unknown fields.
func ConvertUnknownFields(buf unknown.Fields) (fields []UnknownField, err error) {
	if len(buf) == 0 {
		return nil, errors.New("_unknownFields is empty")
	}
	var offset int
	var l int
	var name string
	var fieldTypeId thrift.TType
	var fieldId int16
	var f UnknownField
	for {
		if offset == len(buf) {
			return
		}
		name, fieldTypeId, fieldId, l, err = Binary.ReadFieldBegin(buf[offset:])
		offset += l
		if err != nil {
			return nil, fmt.Errorf("read field %d begin error: %v", fieldId, err)
		}
		l, err = readUnknownField(&f, buf[offset:], name, fieldTypeId, fieldId)
		offset += l
		if err != nil {
			return nil, fmt.Errorf("read unknown field %d error: %v", fieldId, err)
		}
		fields = append(fields, f)
	}
}

func readUnknownField(f *UnknownField, buf []byte, name string, fieldType thrift.TType, id int16) (length int, err error) {
	var size int
	var l int
	f.Name = name
	f.ID = id
	f.Type = int(fieldType)
	switch fieldType {
	case thrift.BOOL:
		f.Value, l, err = Binary.ReadBool(buf[length:])
		length += l
	case thrift.BYTE:
		f.Value, l, err = Binary.ReadByte(buf[length:])
		length += l
	case thrift.I16:
		f.Value, l, err = Binary.ReadI16(buf[length:])
		length += l
	case thrift.I32:
		f.Value, l, err = Binary.ReadI32(buf[length:])
		length += l
	case thrift.I64:
		f.Value, l, err = Binary.ReadI64(buf[length:])
		length += l
	case thrift.DOUBLE:
		f.Value, l, err = Binary.ReadDouble(buf[length:])
		length += l
	case thrift.STRING:
		f.Value, l, err = Binary.ReadString(buf[length:])
		length += l
	case thrift.SET:
		var ttype thrift.TType
		ttype, size, l, err = Binary.ReadSetBegin(buf[length:])
		length += l
		if err != nil {
			return length, fmt.Errorf("read set begin error: %w", err)
		}
		f.ValType = int(ttype)
		set := make([]UnknownField, size)
		for i := 0; i < size; i++ {
			l, err2 := readUnknownField(&set[i], buf[length:], "", thrift.TType(f.ValType), int16(i))
			length += l
			if err2 != nil {
				return length, fmt.Errorf("read set elem error: %w", err2)
			}
		}
		l, err = Binary.ReadSetEnd(buf[length:])
		length += l
		if err != nil {
			return length, fmt.Errorf("read set end error: %w", err)
		}
		f.Value = set
	case thrift.LIST:
		var ttype thrift.TType
		ttype, size, l, err = Binary.ReadListBegin(buf[length:])
		length += l
		if err != nil {
			return length, fmt.Errorf("read list begin error: %w", err)
		}
		f.ValType = int(ttype)
		list := make([]UnknownField, size)
		for i := 0; i < size; i++ {
			l, err2 := readUnknownField(&list[i], buf[length:], "", thrift.TType(f.ValType), int16(i))
			length += l
			if err2 != nil {
				return length, fmt.Errorf("read list elem error: %w", err2)
			}
		}
		l, err = Binary.ReadListEnd(buf[length:])
		length += l
		if err != nil {
			return length, fmt.Errorf("read list end error: %w", err)
		}
		f.Value = list
	case thrift.MAP:
		var kttype, vttype thrift.TType
		kttype, vttype, size, l, err = Binary.ReadMapBegin(buf[length:])
		length += l
		if err != nil {
			return length, fmt.Errorf("read map begin error: %w", err)
		}
		f.KeyType = int(kttype)
		f.ValType = int(vttype)
		flatMap := make([]UnknownField, size*2)
		for i := 0; i < size; i++ {
			l, err2 := readUnknownField(&flatMap[2*i], buf[length:], "", thrift.TType(f.KeyType), int16(i))
			length += l
			if err2 != nil {
				return length, fmt.Errorf("read map key error: %w", err2)
			}
			l, err2 = readUnknownField(&flatMap[2*i+1], buf[length:], "", thrift.TType(f.ValType), int16(i))
			length += l
			if err2 != nil {
				return length, fmt.Errorf("read map value error: %w", err2)
			}
		}
		l, err = Binary.ReadMapEnd(buf[length:])
		length += l
		if err != nil {
			return length, fmt.Errorf("read map end error: %w", err)
		}
		f.Value = flatMap
	case thrift.STRUCT:
		_, l, err = Binary.ReadStructBegin(buf[length:])
		length += l
		if err != nil {
			return length, fmt.Errorf("read struct begin error: %w", err)
		}
		var field UnknownField
		var fields []UnknownField
		for {
			name, fieldTypeID, fieldID, l, err := Binary.ReadFieldBegin(buf[length:])
			length += l
			if err != nil {
				return length, fmt.Errorf("read field begin error: %w", err)
			}
			if fieldTypeID == thrift.STOP {
				break
			}
			l, err = readUnknownField(&field, buf[length:], name, fieldTypeID, fieldID)
			length += l
			if err != nil {
				return length, fmt.Errorf("read struct field error: %w", err)
			}
			l, err = Binary.ReadFieldEnd(buf[length:])
			length += l
			if err != nil {
				return length, fmt.Errorf("read field end error: %w", err)
			}
			fields = append(fields, field)
		}
		l, err = Binary.ReadStructEnd(buf[length:])
		length += l
		if err != nil {
			return length, fmt.Errorf("read struct end error: %w", err)
		}
		f.Value = fields
	default:
		return length, fmt.Errorf("unknown data type %d", f.Type)
	}
	if err != nil {
		return length, err
	}
	return
}

// UnknownFieldsLength returns the length of fs.
func UnknownFieldsLength(fs []UnknownField) (int, error) {
	l := 0
	for _, f := range fs {
		l += Binary.FieldBeginLength(f.Name, thrift.TType(f.Type), f.ID)
		ll, err := unknownFieldLength(&f)
		l += ll
		if err != nil {
			return l, err
		}
		l += Binary.FieldEndLength()
	}
	return l, nil
}

func unknownFieldLength(f *UnknownField) (length int, err error) {
	// use constants to avoid some type assert
	switch f.Type {
	case unknown.TBool:
		length += Binary.BoolLength(false)
	case unknown.TByte:
		length += Binary.ByteLength(0)
	case unknown.TDouble:
		length += Binary.DoubleLength(0)
	case unknown.TI16:
		length += Binary.I16Length(0)
	case unknown.TI32:
		length += Binary.I32Length(0)
	case unknown.TI64:
		length += Binary.I64Length(0)
	case unknown.TString:
		length += Binary.StringLength(f.Value.(string))
	case unknown.TSet:
		vs := f.Value.([]UnknownField)
		length += Binary.SetBeginLength(thrift.TType(f.ValType), len(vs))
		for _, v := range vs {
			l, err := unknownFieldLength(&v)
			length += l
			if err != nil {
				return length, err
			}
		}
		length += Binary.SetEndLength()
	case unknown.TList:
		vs := f.Value.([]UnknownField)
		length += Binary.ListBeginLength(thrift.TType(f.ValType), len(vs))
		for _, v := range vs {
			l, err := unknownFieldLength(&v)
			length += l
			if err != nil {
				return length, err
			}
		}
		length += Binary.ListEndLength()
	case unknown.TMap:
		kvs := f.Value.([]UnknownField)
		length += Binary.MapBeginLength(thrift.TType(f.KeyType), thrift.TType(f.ValType), len(kvs)/2)
		for i := 0; i < len(kvs); i += 2 {
			l, err := unknownFieldLength(&kvs[i])
			length += l
			if err != nil {
				return length, err
			}
			l, err = unknownFieldLength(&kvs[i+1])
			length += l
			if err != nil {
				return length, err
			}
		}
		length += Binary.MapEndLength()
	case unknown.TStruct:
		fs := f.Value.([]UnknownField)
		length += Binary.StructBeginLength(f.Name)
		l, err := UnknownFieldsLength(fs)
		length += l
		if err != nil {
			return length, err
		}
		length += Binary.FieldStopLength()
		length += Binary.StructEndLength()
	default:
		return length, fmt.Errorf("unknown data type %d", f.Type)
	}
	return
}

// WriteUnknownFields writes fs into buf, and return written offset of the buf.
func WriteUnknownFields(buf []byte, fs []UnknownField) (offset int, err error) {
	for _, f := range fs {
		offset += Binary.WriteFieldBegin(buf[offset:], f.Name, thrift.TType(f.Type), f.ID)
		l, err := writeUnknownField(buf[offset:], &f)
		offset += l
		if err != nil {
			return offset, err
		}
		offset += Binary.WriteFieldEnd(buf[offset:])
	}
	return offset, nil
}

func writeUnknownField(buf []byte, f *UnknownField) (offset int, err error) {
	switch f.Type {
	case unknown.TBool:
		offset += Binary.WriteBool(buf, f.Value.(bool))
	case unknown.TByte:
		offset += Binary.WriteByte(buf, f.Value.(int8))
	case unknown.TDouble:
		offset += Binary.WriteDouble(buf, f.Value.(float64))
	case unknown.TI16:
		offset += Binary.WriteI16(buf, f.Value.(int16))
	case unknown.TI32:
		offset += Binary.WriteI32(buf, f.Value.(int32))
	case unknown.TI64:
		offset += Binary.WriteI64(buf, f.Value.(int64))
	case unknown.TString:
		offset += Binary.WriteString(buf, f.Value.(string))
	case unknown.TSet:
		vs := f.Value.([]UnknownField)
		offset += Binary.WriteSetBegin(buf, thrift.TType(f.ValType), len(vs))
		for _, v := range vs {
			l, err := writeUnknownField(buf[offset:], &v)
			offset += l
			if err != nil {
				return offset, err
			}
		}
		offset += Binary.WriteSetEnd(buf[offset:])
	case unknown.TList:
		vs := f.Value.([]UnknownField)
		offset += Binary.WriteListBegin(buf, thrift.TType(f.ValType), len(vs))
		for _, v := range vs {
			l, err := writeUnknownField(buf[offset:], &v)
			offset += l
			if err != nil {
				return offset, err
			}
		}
		offset += Binary.WriteListEnd(buf[offset:])
	case unknown.TMap:
		kvs := f.Value.([]UnknownField)
		offset += Binary.WriteMapBegin(buf, thrift.TType(f.KeyType), thrift.TType(f.ValType), len(kvs)/2)
		for i := 0; i < len(kvs); i += 2 {
			l, err := writeUnknownField(buf[offset:], &kvs[i])
			offset += l
			if err != nil {
				return offset, err
			}
			l, err = writeUnknownField(buf[offset:], &kvs[i+1])
			offset += l
			if err != nil {
				return offset, err
			}
		}
		offset += Binary.WriteMapEnd(buf[offset:])
	case unknown.TStruct:
		fs := f.Value.([]UnknownField)
		offset += Binary.WriteStructBegin(buf, f.Name)
		l, err := WriteUnknownFields(buf[offset:], fs)
		offset += l
		if err != nil {
			return offset, err
		}
		offset += Binary.WriteFieldStop(buf[offset:])
		offset += Binary.WriteStructEnd(buf[offset:])
	default:
		return offset, fmt.Errorf("unknown data type %d", f.Type)
	}
	return
}
