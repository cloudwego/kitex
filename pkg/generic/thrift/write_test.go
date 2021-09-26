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
	"encoding/json"
	"reflect"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/tidwall/gjson"
)

func Test_writeVoid(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteStructBeginFunc: func(name string) error {
			test.Assert(t, name == "")
			return nil
		},
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeVoid",
			args{
				val: 1,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.VOID,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeVoid(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeVoid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeBool(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteBoolFunc: func(val bool) error {
			test.Assert(t, val)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeBool",
			args{
				val: true,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.BOOL,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeBool(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeBool() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeInt8(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteByteFunc: func(val int8) error {
			test.Assert(t, val == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeInt8",
			args{
				val: int8(1),
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeInt8(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeInt8() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeJSONNumber(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteByteFunc: func(val int8) error {
			test.Assert(t, val == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeJSONNumber",
			args{
				val: json.Number("1"),
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeJSONNumber(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeJSONNumber() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeJSONFloat64(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteByteFunc: func(val int8) error {
			test.Assert(t, val == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeJSONFloat64",
			args{
				val: 1.0,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeJSONFloat64(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeJSONFloat64() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeInt16(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteI16Func: func(val int16) error {
			test.Assert(t, val == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeInt16",
			args{
				val: int16(1),
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeInt16(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeInt16() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeInt32(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteI32Func: func(val int32) error {
			test.Assert(t, val == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeInt32",
			args{
				val: int32(1),
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeInt32(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeInt32() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeInt64(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteI64Func: func(val int64) error {
			test.Assert(t, val == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeInt64",
			args{
				val: int64(1),
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeInt64(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeInt64() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeFloat64(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteDoubleFunc: func(val float64) error {
			test.Assert(t, val == 1.0)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeFloat64",
			args{
				val: 1.0,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.DOUBLE,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeFloat64(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeFloat64() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeString(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteStringFunc: func(val string) error {
			test.Assert(t, val == stringInput)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeString",
			args{
				val: stringInput,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.STRING,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeString(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeBinary(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteBinaryFunc: func(val []byte) error {
			test.Assert(t, reflect.DeepEqual(val, []byte(stringInput)))
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeBinary",
			args{
				val: []byte(stringInput),
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.STRING,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeBinary(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeList(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteListBeginFunc: func(elemType thrift.TType, size int) error {
			test.Assert(t, elemType == thrift.STRING)
			test.Assert(t, size == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeList",
			args{
				val: []interface{}{stringInput},
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeList(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeList() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeInterfaceMap(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
			test.Assert(t, keyType == thrift.STRING)
			test.Assert(t, valueType == thrift.STRING)
			test.Assert(t, size == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeInterfaceMap",
			args{
				val: map[interface{}]interface{}{"hello": "world"},
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeInterfaceMap(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeInterfaceMap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeStringMap(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
			test.Assert(t, keyType == thrift.STRING)
			test.Assert(t, valueType == thrift.STRING)
			test.Assert(t, size == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeStringMap",
			args{
				val: map[string]interface{}{"hello": "world"},
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeStringMap(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeStringMap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeStruct(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteStructBeginFunc: func(name string) error {
			test.Assert(t, name == "Demo")
			return nil
		},
		WriteFieldBeginFunc: func(name string, typeID thrift.TType, id int16) error {
			test.Assert(t, name == "hello")
			test.Assert(t, typeID == thrift.STRING)
			test.Assert(t, id == 1)
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeStruct",
			args{
				val: map[string]interface{}{"hello": "world"},
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type: descriptor.STRUCT,
					Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{
						Name: "Demo",
						FieldsByName: map[string]*descriptor.FieldDescriptor{
							"hello": {Name: "hello", ID: 1, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
						RequiredFields: map[int32]*descriptor.FieldDescriptor{
							1: {Name: "hello", ID: 1, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
					},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeStruct(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeStruct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeHTTPRequest(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteStructBeginFunc: func(name string) error {
			test.Assert(t, name == "Demo")
			return nil
		},
		WriteFieldBeginFunc: func(name string, typeID thrift.TType, id int16) error {
			test.Assert(t, name == "hello")
			test.Assert(t, typeID == thrift.STRING)
			test.Assert(t, id == 1)
			return nil
		},
	}
	req := &descriptor.HTTPRequest{
		Body: map[string]interface{}{"hello": "world"},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeStruct",
			args{
				val: req,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type: descriptor.STRUCT,
					Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{
						Name: "Demo",
						FieldsByName: map[string]*descriptor.FieldDescriptor{
							"hello": {
								Name:        "hello",
								ID:          1,
								Type:        &descriptor.TypeDescriptor{Type: descriptor.STRING},
								HTTPMapping: descriptor.DefaultNewMapping("hello"),
							},
						},
					},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeHTTPRequest(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeHTTPRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeRequestBase(t *testing.T) {
	type args struct {
		ctx   context.Context
		val   interface{}
		out   thrift.TProtocol
		field *descriptor.FieldDescriptor
		opt   *writerOption
	}
	depth := 1
	mockTTransport := &mocks.MockThriftTTransport{
		WriteStructBeginFunc: func(name string) error {
			test.Assert(t, name == "Base", name)
			return nil
		},
		WriteFieldBeginFunc: func(name string, typeID thrift.TType, id int16) error {
			if depth == 1 {
				test.Assert(t, name == "base", name)
				test.Assert(t, typeID == thrift.STRUCT, typeID)
				test.Assert(t, id == 255)
				depth++
			}
			return nil
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		// TODO: Add test cases.
		{
			"writeStruct",
			args{
				val: map[string]interface{}{"Extra": map[string]interface{}{"hello": "world"}},
				out: mockTTransport,
				field: &descriptor.FieldDescriptor{
					Name: "base",
					ID:   255,
					Type: &descriptor.TypeDescriptor{Type: descriptor.STRUCT, Name: "base.Base"},
				},
				opt: &writerOption{requestBase: &Base{}},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeRequestBase(tt.args.ctx, tt.args.val, tt.args.out, tt.args.field, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeRequestBase() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeJSON(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteStructBeginFunc: func(name string) error {
			test.Assert(t, name == "Demo")
			return nil
		},
		WriteFieldBeginFunc: func(name string, typeID thrift.TType, id int16) error {
			test.Assert(t, name == "hello")
			test.Assert(t, typeID == thrift.STRING)
			test.Assert(t, id == 1)
			return nil
		},
	}
	data := gjson.Parse(`{"hello": "world"}`)
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeJSON",
			args{
				val: &data,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type: descriptor.STRUCT,
					Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{
						Name: "Demo",
						FieldsByName: map[string]*descriptor.FieldDescriptor{
							"hello": {Name: "hello", ID: 1, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
						RequiredFields: map[int32]*descriptor.FieldDescriptor{
							1: {Name: "hello", ID: 1, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
					},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeJSON(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
