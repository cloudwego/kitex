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
	"context"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/jackedelic/kitex/internal/pkg/generic/descriptor"
)

type readerOption struct {
	// result will be encode to json, so map[interface{}]interface{} will not be valid
	// need use map[string]interface{} instead
	forJSON bool
	// return exception as error
	throwException bool
	// read http response
	http bool
}

type reader func(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error)

func nextReader(tt descriptor.Type, t *descriptor.TypeDescriptor) (reader, error) {
	if err := assertType(tt, t.Type); err != nil {
		return nil, err
	}
	switch tt {
	case descriptor.BOOL:
		return readBool, nil
	case descriptor.BYTE:
		return readByte, nil
	case descriptor.I16:
		return readInt16, nil
	case descriptor.I32:
		return readInt32, nil
	case descriptor.I64:
		return readInt64, nil
	case descriptor.STRING:
		return readString, nil
	case descriptor.DOUBLE:
		return readDouble, nil
	case descriptor.LIST:
		return readList, nil
	case descriptor.SET:
		return readList, nil
	case descriptor.MAP:
		return readMap, nil
	case descriptor.STRUCT:
		return readStruct, nil
	case descriptor.VOID:
		return readVoid, nil
	default:
		return nil, fmt.Errorf("unsupported type: %d", tt)
	}
}

func skipStructReader(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	structName, err := in.ReadStructBegin()
	if err != nil {
		return nil, err
	}
	var v interface{}
	for {
		fieldName, fieldType, fieldID, err := in.ReadFieldBegin()
		if err != nil {
			return nil, err
		}
		if fieldType == thrift.STOP {
			break
		}
		field, ok := t.Struct.FieldsByID[int32(fieldID)]
		if !ok {
			// just ignore the missing field, maybe server update its idls
			if err := in.Skip(fieldType); err != nil {
				return nil, err
			}
		} else {
			_fieldType := descriptor.FromThriftTType(fieldType)
			reader, err := nextReader(_fieldType, field.Type)
			if err != nil {
				return nil, fmt.Errorf("nextReader of %s/%s/%d error %w", structName, fieldName, fieldID, err)
			}
			if field.IsException && opt != nil && opt.throwException {
				if v, err = reader(ctx, in, field.Type, opt); err != nil {
					return nil, err
				}
				// return exception as error
				return nil, fmt.Errorf("%#v", v)
			}
			if opt != nil && opt.http {
				// use http response reader when http generic call
				// only support struct response method, return error when use base type response
				reader = readHTTPResponse
			}
			if v, err = reader(ctx, in, field.Type, opt); err != nil {
				return nil, err
			}
		}
		if err := in.ReadFieldEnd(); err != nil {
			return nil, err
		}
	}

	return v, in.ReadStructEnd()
}

func readVoid(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	_, err := readStruct(ctx, in, t, opt)
	return descriptor.Void{}, err
}

func readDouble(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	return in.ReadDouble()
}

func readBool(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	return in.ReadBool()
}

func readByte(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	return in.ReadByte()
}

func readInt16(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	return in.ReadI16()
}

func readInt32(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	return in.ReadI32()
}

func readInt64(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	return in.ReadI64()
}

func readString(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	return in.ReadString()
}

func readList(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	elemType, length, err := in.ReadListBegin()
	if err != nil {
		return nil, err
	}
	_elemType := descriptor.FromThriftTType(elemType)
	reader, err := nextReader(_elemType, t.Elem)
	if err != nil {
		return nil, err
	}
	l := make([]interface{}, 0, length)
	for i := 0; i < length; i++ {
		item, err := reader(ctx, in, t.Elem, opt)
		if err != nil {
			return nil, err
		}
		l = append(l, item)
	}
	return l, in.ReadListEnd()
}

func readMap(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	if opt != nil && opt.forJSON {
		return readStringMap(ctx, in, t, opt)
	}
	return readInterfaceMap(ctx, in, t, opt)
}

func readInterfaceMap(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	keyType, elemType, length, err := in.ReadMapBegin()
	if err != nil {
		return nil, err
	}
	m := make(map[interface{}]interface{}, length)
	if length == 0 {
		return m, nil
	}
	_keyType := descriptor.FromThriftTType(keyType)
	keyReader, err := nextReader(_keyType, t.Key)
	if err != nil {
		return nil, err
	}
	_elemType := descriptor.FromThriftTType(elemType)
	elemReader, err := nextReader(_elemType, t.Elem)
	if err != nil {
		return nil, err
	}
	for i := 0; i < length; i++ {
		key, err := keyReader(ctx, in, t.Key, opt)
		if err != nil {
			return nil, err
		}
		elem, err := elemReader(ctx, in, t.Elem, opt)
		if err != nil {
			return nil, err
		}
		m[key] = elem
	}
	return m, in.ReadMapEnd()
}

func readStringMap(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	keyType, elemType, length, err := in.ReadMapBegin()
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{}, length)
	if length == 0 {
		return m, nil
	}
	_keyType := descriptor.FromThriftTType(keyType)
	keyReader, err := nextReader(_keyType, t.Key)
	if err != nil {
		return nil, err
	}
	_elemType := descriptor.FromThriftTType(elemType)
	elemReader, err := nextReader(_elemType, t.Elem)
	if err != nil {
		return nil, err
	}
	for i := 0; i < length; i++ {
		key, err := keyReader(ctx, in, t.Key, opt)
		if err != nil {
			return nil, err
		}
		elem, err := elemReader(ctx, in, t.Elem, opt)
		if err != nil {
			return nil, err
		}
		m[buildinTypeIntoString(key)] = elem
	}
	return m, in.ReadMapEnd()
}

func readStruct(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	st := map[string]interface{}{}
	_, err := in.ReadStructBegin()
	if err != nil {
		return nil, err
	}
	readFields := map[int32]struct{}{}
	for {
		_, fieldType, fieldID, err := in.ReadFieldBegin()
		if err != nil {
			return nil, err
		}
		if fieldType == thrift.STOP {
			if err := in.ReadFieldEnd(); err != nil {
				return nil, err
			}
			// check required
			// void is nil struct
			if t.Struct != nil {
				if err := t.Struct.CheckRequired(readFields); err != nil {
					return nil, err
				}
			}
			return st, in.ReadStructEnd()
		}
		field, ok := t.Struct.FieldsByID[int32(fieldID)]
		if !ok {
			// just ignore the missing field, maybe server update its idls
			if err := in.Skip(fieldType); err != nil {
				return nil, err
			}
		} else {
			_fieldType := descriptor.FromThriftTType(fieldType)
			reader, err := nextReader(_fieldType, field.Type)
			if err != nil {
				return nil, fmt.Errorf("nextReader of %s/%s/%d error %w", t.Name, field.Name, fieldID, err)
			}
			val, err := reader(ctx, in, field.Type, opt)
			if err != nil {
				return nil, err
			}
			if field.ValueMapping != nil {
				if val, err = field.ValueMapping.Response(ctx, val, field); err != nil {
					return nil, err
				}
			}
			st[field.FieldName()] = val
		}
		if err := in.ReadFieldEnd(); err != nil {
			return nil, err
		}
		readFields[int32(fieldID)] = struct{}{}
	}
}

func readHTTPResponse(ctx context.Context, in thrift.TProtocol, t *descriptor.TypeDescriptor, opt *readerOption) (interface{}, error) {
	resp := descriptor.NewHTTPResponse()
	_, err := in.ReadStructBegin()
	if err != nil {
		return nil, err
	}
	readFields := map[int32]struct{}{}
	for {
		_, fieldType, fieldID, err := in.ReadFieldBegin()
		if err != nil {
			return nil, err
		}
		if fieldType == thrift.STOP {
			if err := in.ReadFieldEnd(); err != nil {
				return nil, err
			}
			// check required
			if err := t.Struct.CheckRequired(readFields); err != nil {
				return nil, err
			}
			return resp, in.ReadStructEnd()
		}
		field, ok := t.Struct.FieldsByID[int32(fieldID)]
		if !ok {
			// just ignore the missing field, maybe server update its idls
			if err := in.Skip(fieldType); err != nil {
				return nil, err
			}
		} else {
			// check required
			_fieldType := descriptor.FromThriftTType(fieldType)
			reader, err := nextReader(_fieldType, field.Type)
			if err != nil {
				return nil, fmt.Errorf("nextReader of %s/%s/%d error %w", t.Name, field.Name, fieldID, err)
			}
			val, err := reader(ctx, in, field.Type, opt)
			if err != nil {
				return nil, err
			}
			if field.ValueMapping != nil {
				if val, err = field.ValueMapping.Response(ctx, val, field); err != nil {
					return nil, err
				}
			}
			if err = field.HTTPMapping.Response(ctx, resp, field, val); err != nil {
				return nil, err
			}
		}
		if err := in.ReadFieldEnd(); err != nil {
			return nil, err
		}
		readFields[int32(fieldID)] = struct{}{}
	}
}
