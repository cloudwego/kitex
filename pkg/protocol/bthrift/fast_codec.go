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
	ID           int16
	TType        thrift.TType
	Required     bool
	FastReadFunc fastFieldFunc
}

type fastFieldFunc func(buf []byte) (int, error)

// should be the same with struct_tpl.go - StructLikeFastRead
// todo 入参扩展性
// todo fields 字段拷贝开销
// todo 相比之前，switch-case 变成了 map，会有一些劣化

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
				l, err = field.FastReadFunc(buf[offset:])
				// l, err = FastReadField(buf[offset:], field)
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
