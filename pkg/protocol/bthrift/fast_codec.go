/*
 * Copyright 2024 CloudWeGo Authors
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
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
)

type Field struct {
	ID       int16
	TType    thrift.TType
	Required bool

	Field_string *string
	Field_double *float64
	Field_i8     *int8
	Field_i16    *int16
	Field_i32    *int32
	Field_i64    *int64
	Field_bool   *bool
	// FieldBinary ?

	//FieldSetter func(v interface{})
	//NewValues   func(size int) []interface{}
	//NewFields_  func(size int) []interface{}

	FastReadField func(buf []byte) (int, error)
}

// should be the same with struct_tpl.go - StructLikeFastRead
// todo 入参扩展性
// todo fields 字段拷贝开销
// todo 相比之前，switch-case 变成了 map，会有一些劣化
// 各种边界场景代码生成正确性测试

func FastRead(buf []byte, p interface{}, fields map[int16]*Field, KeepUnknownFields bool) (int, error) {
	var err error
	var offset int
	var l int
	var fieldTypeId thrift.TType
	var fieldId int16

	// todo
	var _unknownFields []byte

	var hasFields = len(fields) != 0

	for {
		var beginOff = offset

		_, fieldTypeId, fieldId, l, err = Binary.ReadFieldBegin(buf[offset:])
		offset += l
		if err != nil {
			goto ReadFieldBeginError
		}
		if fieldTypeId == thrift.STOP {
			break
		}

		var field *Field
		var isKnownField bool

		if hasFields {
			field, isKnownField = fields[fieldId]
		}

		if isKnownField {
			if fieldTypeId == field.TType {
				l, err = FastReadField(buf[offset:], field)
				offset += l
				if err != nil {
					goto ReadFieldError
				}
				// required 字段默认为 true，被填充过后变成 false，最后检查所有 fields 的 required 字段都是 false 就可以了
				field.Required = false
			} else {
				l, err = Binary.Skip(buf[offset:], fieldTypeId)
				offset += l
				if err != nil {
					goto SkipFieldError
				}
			}
		} else {
			l, err = Binary.Skip(buf[offset:], fieldTypeId)
			offset += l
			if err != nil {
				goto SkipFieldError
			}
			if KeepUnknownFields {
				_unknownFields = append(_unknownFields, buf[beginOff:offset]...)
			}
		}
	}

	for _, f := range fields {
		if f.Required {
			fieldId = f.ID
			goto RequiredFieldNotSetError
		}
	}
	return offset, nil
ReadFieldBeginError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	// todo 之前是 name，为了性能先去掉了
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d error: ", p, fieldId), err)
SkipFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)
RequiredFieldNotSetError:
	// todo 之前是 name，为了性能先去掉了
	return offset, thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("required field %d is not set", fieldId))
}

type FastReadStruct interface {
	InitDefault()
	FastRead(buf []byte) (int, error)
}

func FastReadField(buf []byte, field *Field) (int, error) {

	switch field.TType {
	case thrift.STRING:
		if v, l, err := Binary.ReadString(buf); err != nil {
			return 0, err
		} else {
			field.Field_string = &v
			return l, nil
		}
	case thrift.DOUBLE:
		if v, l, err := Binary.ReadDouble(buf); err != nil {
			return 0, err
		} else {
			field.Field_double = &v
			return l, nil
		}
	case thrift.BYTE:
		if v, l, err := Binary.ReadByte(buf); err != nil {
			return 0, err
		} else {
			field.Field_i8 = &v
			return l, nil
		}
	case thrift.BOOL:
		if v, l, err := Binary.ReadBool(buf); err != nil {
			return 0, err
		} else {
			field.Field_bool = &v
			return l, nil
		}
	case thrift.I32:
		if v, l, err := Binary.ReadI32(buf); err != nil {
			return 0, err
		} else {
			field.Field_i32 = &v
			return l, nil
		}
	case thrift.I16:
		if v, l, err := Binary.ReadI16(buf); err != nil {
			return 0, err
		} else {
			field.Field_i16 = &v
			return l, nil
		}
	case thrift.I64:
		if v, l, err := Binary.ReadI64(buf); err != nil {
			return 0, err
		} else {
			field.Field_i64 = &v
			return l, nil
		}
	//case thrift.LIST:
	//
	//	setField := field.FieldSetter
	//	if setField == nil {
	//		return 0, fmt.Errorf("no field setter")
	//	}
	//
	//	_, size, l, err := Binary.ReadListBegin(buf)
	//	if err != nil {
	//		return l, err
	//	}
	//
	//	_field := field.NewFields_(size) //make([]*FastReadStruct, 0, size)
	//	values := field.NewValues(size)  //make([]FastReadStruct, size)
	//	offset := 0
	//	for i := 0; i < size; i++ {
	//		// check 这里改了指针类型
	//		_elem := values[i].(FastReadStruct)
	//		_elem.InitDefault()
	//		if l, err := _elem.FastRead(buf[offset:]); err != nil {
	//			return offset, err
	//		} else {
	//			offset += l
	//		}
	//		_field = append(_field, &_elem)
	//	}
	//	if l, err := Binary.ReadListEnd(buf[offset:]); err != nil {
	//		return offset, err
	//	} else {
	//		offset += l
	//	}
	//	setField(_field)
	//	return offset, nil

	default:
		fr := field.FastReadField
		if fr != nil {
			return fr(buf)
		}
		return 0, fmt.Errorf("no fast read")
	}
}
