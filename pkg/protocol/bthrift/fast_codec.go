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
	id           int16
	ttype        thrift.TType
	name         string
	required     bool
	fastReadFunc fastFieldFunc
	isset        bool
	// setter       setterFunc
}

type Fields map[int16]*Field

func NewFields(len int) Fields {
	return make(map[int16]*Field, len)
}

func (fs Fields) Add(ID int16, TType thrift.TType, name string, required bool, fastReadFunc fastFieldFunc) Fields {
	f := &Field{
		id:           ID,
		ttype:        TType,
		name:         name,
		required:     required,
		fastReadFunc: fastReadFunc,
	}
	fs[ID] = f
	return fs
}

type setterFunc func(buf []byte) (int, error)

type fastFieldFunc func(buf []byte) (int, error)

// should be the same with struct_tpl.go - StructLikeFastRead
// todo 入参扩展性
// todo fields 字段拷贝开销

func FastRead(buf []byte, p interface{}, fields Fields, KeepUnknownFields bool) (int, error) {
	var err error
	var offset int
	var l int
	var fieldTypeId thrift.TType
	var fieldId int16
	var fieldName string

	var _unknownFields []byte

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

		if len(fields) == 0 {
			isKnownField = false
		} else {
			field, isKnownField = fields[fieldId]
		}

		if isKnownField {
			fieldName = field.name
			if fieldTypeId == field.ttype {
				l, err = field.fastReadFunc(buf[offset:])
				// l, err = FastReadField(buf[offset:], field)
				offset += l
				if err != nil {
					goto ReadFieldError
				}
				field.isset = true
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
		if f.required && !f.isset {
			fieldId = f.id
			goto RequiredFieldNotSetError
		}
		// ???todo
		f.isset = false
	}
	return offset, nil
ReadFieldBeginError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d begin error: ", p, fieldId), err)
ReadFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T read field %d '%s' error: ", p, fieldId, fieldName), err)
SkipFieldError:
	return offset, thrift.PrependError(fmt.Sprintf("%T field %d skip type %d error: ", p, fieldId, fieldTypeId), err)
RequiredFieldNotSetError:
	return offset, thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("required field %s is not set", fieldName))
}

//
//// todo 各种逻辑，还有 field mask 补全
//func FastReadField(buf []byte, f Field) (int, error) {
//	offset := 0
//	switch f.ttype {
//	case thrift.BYTE:
//
//	case thrift.I16:
//
//	case thrift.I32:
//
//	case thrift.I64:
//		var _field int64
//		if v, l, err := Binary.ReadI64(buf[offset:]); err != nil {
//			return offset, err
//		} else {
//			offset += l
//			_field = v
//		}
//	    f.setter(_field)
//		return offset, nil
//	case thrift.STRING:
//
//	case thrift.BOOL:
//
//	case thrift.DOUBLE:
//
//	case thrift.STRUCT:
//
//	case thrift.MAP:
//
//	case thrift.LIST:
//
//	case thrift.SET:
//
//		// todo more type
//	}
//	var _field int64
//	if v, l, err := Binary.ReadI64(buf[offset:]); err != nil {
//		return offset, err
//	} else {
//		offset += l
//		_field = v
//	}
//	// f.setter(_field)
//	return offset, nil
//}
