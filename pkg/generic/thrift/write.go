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
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/tidwall/gjson"
)

type writerOption struct {
	requestBase *Base // request base from metahandler
	// decoding Base64 to binary
	binaryWithBase64 bool
}

type writer func(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error

func typeOf(sample interface{}, t *descriptor.TypeDescriptor, opt *writerOption) (descriptor.Type, writer, error) {
	tt := t.Type
	switch sample.(type) {
	case bool:
		return descriptor.BOOL, writeBool, nil
	case int8, byte:
		return descriptor.I08, writeInt8, nil
	case int16:
		return descriptor.I16, writeInt16, nil
	case int32:
		return descriptor.I32, writeInt32, nil
	case int64:
		return descriptor.I64, writeInt64, nil
	case float64:
		// maybe come from json decode
		switch tt {
		case descriptor.I08, descriptor.I16, descriptor.I32, descriptor.I64, descriptor.DOUBLE:
			return tt, writeJSONFloat64, nil
		}
	case json.Number:
		switch tt {
		case descriptor.I08, descriptor.I16, descriptor.I32, descriptor.I64, descriptor.DOUBLE:
			return tt, writeJSONNumber, nil
		}
	case string:
		// maybe a base64 string encoded from binary
		if t.Name == "binary" && opt.binaryWithBase64 {
			return descriptor.STRING, writeBase64Binary, nil
		}
		// maybe a json number string
		return descriptor.STRING, writeString, nil
	case []byte:
		return descriptor.STRING, writeBinary, nil
	case []interface{}:
		return descriptor.LIST, writeList, nil
	case map[interface{}]interface{}:
		return descriptor.MAP, writeInterfaceMap, nil
	case map[string]interface{}:
		//  4: optional map<i64, ReqItem> req_items (api.body='req_items')
		// need parse string into int64
		switch tt {
		case descriptor.STRUCT:
			return descriptor.STRUCT, writeStruct, nil
		case descriptor.MAP:
			return descriptor.MAP, writeStringMap, nil
		}
	case *descriptor.HTTPRequest:
		return descriptor.STRUCT, writeHTTPRequest, nil
	case *gjson.Result:
		return descriptor.STRUCT, writeJSON, nil
	case nil, descriptor.Void: // nil and Void
		return descriptor.VOID, writeVoid, nil
	}
	return 0, nil, fmt.Errorf("unsupported type:%T, expected type:%s", sample, tt)
}

func typeJSONOf(data *gjson.Result, t *descriptor.TypeDescriptor, opt *writerOption) (v interface{}, w writer, err error) {
	tt := t.Type
	defer func() {
		if r := recover(); r != nil {
			err = perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("json convert error:%#+v", r))
		}
	}()
	switch tt {
	case descriptor.BOOL:
		v = data.Bool()
		w = writeBool
		return
	case descriptor.I08:
		v = int8(data.Int())
		w = writeInt8
		return
	case descriptor.I16:
		v = int16(data.Int())
		w = writeInt16
		return
	case descriptor.I32:
		v = int32(data.Int())
		w = writeInt32
		return
	case descriptor.I64:
		v = data.Int()
		w = writeInt64
		return
	case descriptor.DOUBLE:
		v = data.Float()
		w = writeJSONFloat64
		return
	case descriptor.STRING:
		v = data.String()
		if t.Name == "binary" && opt.binaryWithBase64 {
			w = writeBase64Binary
		} else {
			w = writeString
		}
		return
	// case descriptor.BINARY:
	//	return writeBinary, nil
	case descriptor.SET, descriptor.LIST:
		v = data.Array()
		w = writeJSONList
		return
	case descriptor.MAP:
		v = data.Map()
		w = writeStringJSONMap
		return
	case descriptor.STRUCT:
		v = data
		w = writeJSON
		return
	case descriptor.VOID: // nil and Void
		v = data
		w = writeVoid
		return
	}
	return 0, nil, fmt.Errorf("data:%#v, expected type:%s, err:%#v", data, tt, err)
}

func nextWriter(sample interface{}, t *descriptor.TypeDescriptor, opt *writerOption) (writer, error) {
	tt, fn, err := typeOf(sample, t, opt)
	if err != nil {
		return nil, err
	}
	if t.Type == thrift.SET && tt == thrift.LIST {
		tt = thrift.SET
	}
	return fn, assertType(t.Type, tt)
}

func nextJSONWriter(data *gjson.Result, t *descriptor.TypeDescriptor, opt *writerOption) (interface{}, writer, error) {
	v, fn, err := typeJSONOf(data, t, opt)
	if err != nil {
		return nil, nil, err
	}
	return v, fn, nil
}

func wrapStructWriter(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	if err := out.WriteStructBegin(t.Struct.Name); err != nil {
		return err
	}
	for name, field := range t.Struct.FieldsByName {
		if field.IsException {
			// generic server ignore the exception, because no description for exception
			// generic handler just return error
			continue
		}
		if err := out.WriteFieldBegin(field.Name, field.Type.Type.ToThriftTType(), int16(field.ID)); err != nil {
			return err
		}
		writer, err := nextWriter(val, field.Type, opt)
		if err != nil {
			return fmt.Errorf("nextWriter of field[%s] error %w", name, err)
		}
		if err := writer(ctx, val, out, field.Type, opt); err != nil {
			return err
		}
		if err := out.WriteFieldEnd(); err != nil {
			return err
		}
	}
	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	return out.WriteStructEnd()
}

func wrapJSONWriter(ctx context.Context, val *gjson.Result, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	if err := out.WriteStructBegin(t.Struct.Name); err != nil {
		return err
	}
	for name, field := range t.Struct.FieldsByName {
		if field.IsException {
			// generic server ignore the exception, because no description for exception
			// generic handler just return error
			continue
		}
		if err := out.WriteFieldBegin(field.Name, field.Type.Type.ToThriftTType(), int16(field.ID)); err != nil {
			return err
		}
		v, writer, err := nextJSONWriter(val, field.Type, opt)
		if err != nil {
			return fmt.Errorf("nextWriter of field[%s] error %w", name, err)
		}
		if err := writer(ctx, v, out, field.Type, opt); err != nil {
			return err
		}
		if err := out.WriteFieldEnd(); err != nil {
			return err
		}
	}
	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	return out.WriteStructEnd()
}

func writeVoid(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	return writeStruct(ctx, map[string]interface{}{}, out, t, opt)
}

func writeBool(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	return out.WriteBool(val.(bool))
}

func writeInt8(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	return out.WriteByte(val.(int8))
}

func writeJSONNumber(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	jn := val.(json.Number)
	switch t.Type {
	case thrift.I08:
		i, err := jn.Int64()
		if err != nil {
			return err
		}
		return writeInt8(ctx, int8(i), out, t, opt)
	case thrift.I16:
		i, err := jn.Int64()
		if err != nil {
			return err
		}
		return writeInt16(ctx, int16(i), out, t, opt)
	case thrift.I32:
		i, err := jn.Int64()
		if err != nil {
			return err
		}
		return writeInt32(ctx, int32(i), out, t, opt)
	case thrift.I64:
		i, err := jn.Int64()
		if err != nil {
			return err
		}
		return writeInt64(ctx, i, out, t, opt)
	case thrift.DOUBLE:
		i, err := jn.Float64()
		if err != nil {
			return err
		}
		return writeFloat64(ctx, i, out, t, opt)
	}
	return nil
}

func writeJSONFloat64(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	i := val.(float64)
	switch t.Type {
	case thrift.I08:
		return writeInt8(ctx, int8(i), out, t, opt)
	case thrift.I16:
		return writeInt16(ctx, int16(i), out, t, opt)
	case thrift.I32:
		return writeInt32(ctx, int32(i), out, t, opt)
	case thrift.I64:
		return writeInt64(ctx, int64(i), out, t, opt)
	case thrift.DOUBLE:
		return writeFloat64(ctx, i, out, t, opt)
	}
	return fmt.Errorf("need number type, but got: %s", t.Type)
}

func writeInt16(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	return out.WriteI16(val.(int16))
}

func writeInt32(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	return out.WriteI32(val.(int32))
}

func writeInt64(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	return out.WriteI64(val.(int64))
}

func writeFloat64(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	return out.WriteDouble(val.(float64))
}

func writeString(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	return out.WriteString(val.(string))
}

func writeBase64Binary(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	bytes, err := base64.StdEncoding.DecodeString(val.(string))
	if err != nil {
		return err
	}
	return out.WriteBinary(bytes)
}

func writeBinary(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	return out.WriteBinary(val.([]byte))
}

func writeList(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	l := val.([]interface{})
	length := len(l)
	if err := out.WriteListBegin(t.Elem.Type.ToThriftTType(), length); err != nil {
		return err
	}
	if length == 0 {
		return out.WriteListEnd()
	}
	writer, err := nextWriter(l[0], t.Elem, opt)
	if err != nil {
		return err
	}
	for _, elem := range l {
		if err := writer(ctx, elem, out, t.Elem, opt); err != nil {
			return err
		}
	}
	return out.WriteListEnd()
}

func writeJSONList(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	l := val.([]gjson.Result)
	length := len(l)
	if err := out.WriteListBegin(t.Elem.Type.ToThriftTType(), length); err != nil {
		return err
	}
	if length == 0 {
		return out.WriteListEnd()
	}
	for _, elem := range l {
		v, writer, err := nextJSONWriter(&elem, t.Elem, opt)
		if err != nil {
			return err
		}
		if err := writer(ctx, v, out, t.Elem, opt); err != nil {
			return err
		}
	}
	return out.WriteListEnd()
}

func takeSampleFromMap(sample map[interface{}]interface{}) (interface{}, interface{}) {
	for key, elem := range sample {
		return key, elem
	}
	panic("unreachable")
}

func writeInterfaceMap(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	m := val.(map[interface{}]interface{})
	length := len(m)
	if err := out.WriteMapBegin(t.Key.Type.ToThriftTType(), t.Elem.Type.ToThriftTType(), length); err != nil {
		return err
	}
	if length == 0 {
		return out.WriteMapEnd()
	}
	keySample, elemSample := takeSampleFromMap(m)
	keyWriter, err := nextWriter(keySample, t.Key, opt)
	if err != nil {
		return err
	}
	elemWriter, err := nextWriter(elemSample, t.Elem, opt)
	if err != nil {
		return err
	}
	for key, elem := range m {
		if err := keyWriter(ctx, key, out, t.Key, opt); err != nil {
			return err
		}
		if err := elemWriter(ctx, elem, out, t.Elem, opt); err != nil {
			return err
		}
	}
	return out.WriteMapEnd()
}

func writeStringMap(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	m := val.(map[string]interface{})
	length := len(m)
	if err := out.WriteMapBegin(t.Key.Type.ToThriftTType(), t.Elem.Type.ToThriftTType(), length); err != nil {
		return err
	}
	if length == 0 {
		return out.WriteMapEnd()
	}

	var (
		keyWriter  writer
		elemWriter writer
	)
	for key, elem := range m {
		_key, err := buildinTypeFromString(key, t.Key)
		if err != nil {
			return err
		}
		if keyWriter == nil {
			if keyWriter, err = nextWriter(_key, t.Key, opt); err != nil {
				return err
			}
		}
		if elemWriter == nil {
			if elemWriter, err = nextWriter(elem, t.Elem, opt); err != nil {
				return err
			}
		}
		if err := keyWriter(ctx, _key, out, t.Key, opt); err != nil {
			return err
		}

		if err := elemWriter(ctx, elem, out, t.Elem, opt); err != nil {
			return err
		}
	}
	return out.WriteMapEnd()
}

func writeStringJSONMap(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	m := val.(map[string]gjson.Result)
	length := len(m)
	if err := out.WriteMapBegin(t.Key.Type.ToThriftTType(), t.Elem.Type.ToThriftTType(), length); err != nil {
		return err
	}
	if length == 0 {
		return out.WriteMapEnd()
	}

	var (
		keyWriter  writer
		elemWriter writer
		v          interface{}
	)
	for key, elem := range m {
		_key, err := buildinTypeFromString(key, t.Key)
		if err != nil {
			return err
		}
		if keyWriter == nil {
			if keyWriter, err = nextWriter(_key, t.Key, opt); err != nil {
				return err
			}
		}
		if v, elemWriter, err = nextJSONWriter(&elem, t.Elem, opt); err != nil {
			return err
		}
		if err := keyWriter(ctx, _key, out, t.Key, opt); err != nil {
			return err
		}

		if err := elemWriter(ctx, v, out, t.Elem, opt); err != nil {
			return err
		}
	}
	return out.WriteMapEnd()
}

func writeRequestBase(ctx context.Context, val interface{}, out thrift.TProtocol, field *descriptor.FieldDescriptor, opt *writerOption) error {
	if st, ok := val.(map[string]interface{}); ok {
		// copy from user's Extra
		if ext, ok := st["Extra"]; ok {
			switch v := ext.(type) {
			case map[string]interface{}:
				// from http json
				for key, value := range v {
					if _, ok := opt.requestBase.Extra[key]; !ok {
						if vStr, ok := value.(string); ok {
							if opt.requestBase.Extra == nil {
								opt.requestBase.Extra = map[string]string{}
							}
							opt.requestBase.Extra[key] = vStr
						}
					}
				}
			case map[interface{}]interface{}:
				// from struct map
				for key, value := range v {
					if kStr, ok := key.(string); ok {
						if _, ok := opt.requestBase.Extra[kStr]; !ok {
							if vStr, ok := value.(string); ok {
								if opt.requestBase.Extra == nil {
									opt.requestBase.Extra = map[string]string{}
								}
								opt.requestBase.Extra[kStr] = vStr
							}
						}
					}
				}
			}
		}
	}
	if err := out.WriteFieldBegin(field.Name, field.Type.Type.ToThriftTType(), int16(field.ID)); err != nil {
		return err
	}
	if err := opt.requestBase.Write(out); err != nil {
		return err
	}
	return out.WriteFieldEnd()
}

// writeStruct iter with Descriptor, can check the field's required and others
func writeStruct(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	st := val.(map[string]interface{})
	err := out.WriteStructBegin(t.Struct.Name)
	if err != nil {
		return err
	}
	for name, field := range t.Struct.FieldsByName {
		elem, ok := st[name]
		if field.Type.IsRequestBase && opt.requestBase != nil {
			if err := writeRequestBase(ctx, elem, out, field, opt); err != nil {
				return err
			}
			continue
		}
		if !ok || elem == nil {
			if field.Required {
				return fmt.Errorf("required field (%d/%s) missing", field.ID, name)
			}
			continue
		}
		if field.ValueMapping != nil {
			elem, err = field.ValueMapping.Request(ctx, elem, field)
			if err != nil {
				return err
			}
		}
		writer, err := nextWriter(elem, field.Type, opt)
		if err != nil {
			return fmt.Errorf("nextWriter of field[%s] error %w", name, err)
		}
		if err := out.WriteFieldBegin(field.Name, field.Type.Type.ToThriftTType(), int16(field.ID)); err != nil {
			return err
		}
		if err := writer(ctx, elem, out, field.Type, opt); err != nil {
			return err
		}
		if err := out.WriteFieldEnd(); err != nil {
			return err
		}
	}
	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	return out.WriteStructEnd()
}

func writeHTTPRequest(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	req := val.(*descriptor.HTTPRequest)
	defer func() {
		if req.Params != nil {
			req.Params.Recycle()
		}
	}()
	if err := out.WriteStructBegin(t.Struct.Name); err != nil {
		return err
	}
	for name, field := range t.Struct.FieldsByName {
		v, err := requestMappingValue(ctx, req, field)
		if err != nil {
			return err
		}
		if field.Type.IsRequestBase && opt.requestBase != nil {
			if err := writeRequestBase(ctx, v, out, field, opt); err != nil {
				return err
			}
			continue
		}
		if v == nil {
			if field.Required {
				return fmt.Errorf("required field (%d/%s) missing", field.ID, name)
			}
			continue
		}
		if field.ValueMapping != nil {
			if v, err = field.ValueMapping.Request(ctx, v, field); err != nil {
				return err
			}
		}
		writer, err := nextWriter(v, field.Type, opt)
		if err != nil {
			return fmt.Errorf("nextWriter of field[%s] error %w", name, err)
		}
		if err := out.WriteFieldBegin(field.Name, field.Type.Type.ToThriftTType(), int16(field.ID)); err != nil {
			return err
		}
		if err := writer(ctx, v, out, field.Type, opt); err != nil {
			return err
		}
		if err := out.WriteFieldEnd(); err != nil {
			return err
		}
	}
	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	return out.WriteStructEnd()
}

func writeJSON(ctx context.Context, val interface{}, out thrift.TProtocol, t *descriptor.TypeDescriptor, opt *writerOption) error {
	data := val.(*gjson.Result)
	err := out.WriteStructBegin(t.Struct.Name)
	if err != nil {
		return err
	}
	for name, field := range t.Struct.FieldsByName {
		elem := data.Get(name)
		if field.Type.IsRequestBase && opt.requestBase != nil {
			elemI := elem.Value()
			if err := writeRequestBase(ctx, elemI, out, field, opt); err != nil {
				return err
			}
			continue
		}

		if elem.Type == gjson.Null {
			if field.Required {
				return perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("required field (%d/%s) missing", field.ID, name))
			}
			continue
		}

		v, writer, err := nextJSONWriter(&elem, field.Type, opt)
		if err != nil {
			return fmt.Errorf("nextWriter of field[%s] error %w", name, err)
		}
		if err := out.WriteFieldBegin(field.Name, field.Type.Type.ToThriftTType(), int16(field.ID)); err != nil {
			return err
		}
		if err := writer(ctx, v, out, field.Type, opt); err != nil {
			return err
		}
		if err := out.WriteFieldEnd(); err != nil {
			return err
		}

	}
	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	return out.WriteStructEnd()
}
