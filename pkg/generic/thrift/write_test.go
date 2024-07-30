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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/tidwall/gjson"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/proto"
	thrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
)

func Test_nextWriter(t *testing.T) {
	// add some testcases
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := func(v interface{}) *mocks.MockThriftTTransport {
		toint := func(i interface{}) int64 {
			switch ti := i.(type) {
			case int8:
				return int64(ti)
			case int16:
				return int64(ti)
			case int32:
				return int64(ti)
			case int64:
				return int64(ti)
			case int:
				return int64(ti)
			default:
				t.Errorf("type %v not support toint", reflect.TypeOf(v))
			}
			return 0
		}
		return &mocks.MockThriftTTransport{
			WriteByteFunc: func(val int8) error {
				test.Assert(t, val == int8(toint(v)))
				return nil
			},
			WriteI16Func: func(val int16) error {
				test.Assert(t, val == int16(toint(v)))
				return nil
			},
			WriteI32Func: func(val int32) error {
				test.Assert(t, val == int32(toint(v)))
				return nil
			},
			WriteI64Func: func(val int64) error {
				test.Assert(t, val == toint(v))
				return nil
			},
			WriteDoubleFunc: func(val float64) error {
				test.Assert(t, val == v.(float64))
				return nil
			},
			WriteBoolFunc: func(val bool) error {
				test.Assert(t, val == v.(bool))
				return nil
			},
		}
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"nextWriteri8 Success",
			args{
				val: int8(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			false,
		},
		{
			"nextWriteri16 Success",
			args{
				val: int16(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			false,
		},
		{
			"nextWriteri32 Success",
			args{
				val: int32(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I32,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			false,
		},
		{
			"nextWriteri64 Success",
			args{
				val: int64(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I64,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			false,
		},
		{
			"nextWriterbool Success",
			args{
				val: true,
				out: mockTTransport(true),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.BOOL,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			false,
		},
		{
			"nextWriterdouble Success",
			args{
				val: float64(1.0),
				out: mockTTransport(float64(1.0)),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.DOUBLE,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			false,
		},
		{
			"nextWriteri8 Failed",
			args{
				val: 10000000,
				out: mockTTransport(10000000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			true,
		},
		{
			"nextWriteri16 Failed",
			args{
				val: 10000000,
				out: mockTTransport(10000000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			true,
		},
		{
			"nextWriteri32 Failed",
			args{
				val: 10000000,
				out: mockTTransport(10000000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I32,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			true,
		},
		{
			"nextWriteri64 Failed",
			args{
				val: "10000000",
				out: mockTTransport(10000000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I64,
					Struct: &descriptor.StructDescriptor{},
				},
				opt: &writerOption{
					requestBase:      &Base{},
					binaryWithBase64: false,
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			var writerfunc writer
			if writerfunc, err = nextWriter(tt.args.val, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("nextWriter() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if writerfunc == nil {
				t.Error("nextWriter() error = nil, but writerfunc == nil")
				return
			}
			if err := writerfunc(context.Background(), tt.args.val, tt.args.out, tt.args.t, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writerfunc() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

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
	mockTTransport := func(v int8) *mocks.MockThriftTTransport {
		return &mocks.MockThriftTTransport{
			WriteByteFunc: func(val int8) error {
				test.Assert(t, val == v)
				return nil
			},
			WriteI16Func: func(val int16) error {
				test.Assert(t, val == int16(v))
				return nil
			},
		}
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
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			name: "writeInt8 byte",
			args: args{
				val: byte(128),
				out: mockTTransport(-128), // overflow
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			wantErr: false,
		},
		{
			name: "writeInt8 error",
			args: args{
				val: int16(2),
				out: mockTTransport(2),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			wantErr: true,
		},
		{
			name: "writeInt8 to i16",
			args: args{
				val: int8(2),
				out: mockTTransport(2),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			wantErr: false,
		},
		{
			name: "writeInt8 to i32",
			args: args{
				val: int8(2),
				out: mockTTransport(2),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I32,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			wantErr: false,
		},
		{
			name: "writeInt8 to i64",
			args: args{
				val: int8(2),
				out: mockTTransport(2),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I64,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			wantErr: false,
		},
		{
			name: "writeInt8 to i64",
			args: args{
				val: int8(2),
				out: mockTTransport(2),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.DOUBLE,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			wantErr: true,
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
		{
			"writeJSONFloat64 bool Failed",
			args{
				val: 1.0,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.BOOL,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
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
	mockTTransport := func(v int64) *mocks.MockThriftTTransport {
		return &mocks.MockThriftTTransport{
			WriteI32Func: func(val int32) error {
				test.Assert(t, val == int32(v))
				return nil
			},
			WriteI16Func: func(val int16) error {
				test.Assert(t, val == int16(v))
				return nil
			},
			WriteByteFunc: func(val int8) error {
				test.Assert(t, val == int8(v))
				return nil
			},
		}
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
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt16toInt8 Success",
			args{
				val: int16(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt16toInt8 Failed",
			args{
				val: int16(10000),
				out: mockTTransport(10000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
		},
		{
			"writeInt16toInt32 Success",
			args{
				val: int16(10000),
				out: mockTTransport(10000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I32,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt16toInt64 Success",
			args{
				val: int16(10000),
				out: mockTTransport(10000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I64,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt16 Failed",
			args{
				val: int16(10000),
				out: mockTTransport(10000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.DOUBLE,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
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
	mockTTransport := func(v int64) *mocks.MockThriftTTransport {
		return &mocks.MockThriftTTransport{
			WriteI64Func: func(val int64) error {
				test.Assert(t, val == v)
				return nil
			},
			WriteI32Func: func(val int32) error {
				test.Assert(t, val == int32(v))
				return nil
			},
			WriteI16Func: func(val int16) error {
				test.Assert(t, val == int16(v))
				return nil
			},
			WriteByteFunc: func(val int8) error {
				test.Assert(t, val == int8(v))
				return nil
			},
		}
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeInt32 Success",
			args{
				val: int32(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I32,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt32 Failed",
			args{
				val: int32(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.DOUBLE,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
		},
		{
			"writeInt32ToInt8 Success",
			args{
				val: int32(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt32ToInt8 Failed",
			args{
				val: int32(100000),
				out: mockTTransport(100000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
		},
		{
			"writeInt32ToInt16 success",
			args{
				val: int32(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt32ToInt16 Failed",
			args{
				val: int32(100000),
				out: mockTTransport(100000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
		},
		{
			"writeInt32ToInt64 Success",
			args{
				val: int32(10000000),
				out: mockTTransport(10000000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I64,
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
	mockTTransport := func(v int64) *mocks.MockThriftTTransport {
		return &mocks.MockThriftTTransport{
			WriteI64Func: func(val int64) error {
				test.Assert(t, val == v)
				return nil
			},
			WriteI32Func: func(val int32) error {
				test.Assert(t, val == int32(v))
				return nil
			},
			WriteI16Func: func(val int16) error {
				test.Assert(t, val == int16(v))
				return nil
			},
			WriteByteFunc: func(val int8) error {
				test.Assert(t, val == int8(v))
				return nil
			},
		}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeInt64 Success",
			args{
				val: int64(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I64,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt64 Failed",
			args{
				val: int64(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.DOUBLE,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
		},
		{
			"writeInt64ToInt8 Success",
			args{
				val: int64(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt64ToInt8 failed",
			args{
				val: int64(1000),
				out: mockTTransport(1000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I08,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
		},
		{
			"writeInt64ToInt16 Success",
			args{
				val: int64(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt64ToInt16 failed",
			args{
				val: int64(100000000000),
				out: mockTTransport(100000000000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I16,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
		},
		{
			"writeInt64ToInt32 Success",
			args{
				val: int64(1),
				out: mockTTransport(1),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I32,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInt64ToInt32 failed",
			args{
				val: int64(100000000000),
				out: mockTTransport(100000000000),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.I32,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
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
		WriteBinaryFunc: func(val []byte) error {
			test.DeepEqual(t, val, binaryInput)
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

func Test_writeBase64String(t *testing.T) {
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
		WriteBinaryFunc: func(val []byte) error {
			test.DeepEqual(t, val, binaryInput)
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
			"writeBase64Binary", // write to binary field with base64 string
			args{
				val: base64.StdEncoding.EncodeToString(binaryInput),
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Name:   "binary",
					Type:   descriptor.STRING,
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeBase64Binary(context.Background(), tt.args.val, tt.args.out, tt.args.t,
				tt.args.opt); (err != nil) != tt.wantErr {
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

func Test_writeBinaryList(t *testing.T) {
	type args struct {
		val []byte
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	type params struct {
		listBeginErr error
		writeByteErr error
		listEndErr   error
	}
	commonArgs := args{
		val: []byte(stringInput),
		t: &descriptor.TypeDescriptor{
			Type:   descriptor.LIST,
			Elem:   &descriptor.TypeDescriptor{Type: descriptor.BYTE},
			Struct: &descriptor.StructDescriptor{},
		},
	}
	tests := []struct {
		name    string
		args    args
		params  params
		wantErr bool
	}{
		{
			name:    "writeBinaryList",
			args:    commonArgs,
			wantErr: false,
		},
		{
			name: "list begin error",
			args: commonArgs,
			params: params{
				listBeginErr: errors.New("test error"),
			},
			wantErr: true,
		},
		{
			name: "write byte error",
			args: commonArgs,
			params: params{
				writeByteErr: errors.New("test error"),
			},
			wantErr: true,
		},
		{
			name: "list end error",
			args: commonArgs,
			params: params{
				listEndErr: errors.New("test error"),
			},
			wantErr: true,
		},
		{
			name: "empty slice",
			args: args{
				val: []byte(""),
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.BYTE},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var writtenData []byte
			var endHit bool
			mockTTransport := &mocks.MockThriftTTransport{
				WriteListBeginFunc: func(elemType thrift.TType, size int) error {
					test.Assert(t, elemType == thrift.BYTE)
					test.Assert(t, size == len(tt.args.val))
					return tt.params.listBeginErr
				},
				WriteByteFunc: func(val int8) error {
					writtenData = append(writtenData, byte(val))
					return tt.params.writeByteErr
				},
				WriteListEndFunc: func() error {
					endHit = true
					return tt.params.listEndErr
				},
			}

			if err := writeBinaryList(context.Background(), tt.args.val, mockTTransport, tt.args.t,
				tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("writeBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				test.Assert(t, bytes.Equal(tt.args.val, writtenData))
				test.Assert(t, endHit == true)
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
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.STRING)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeListWithNil",
			args{
				val: []interface{}{stringInput, nil, stringInput},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.STRING)
						test.Assert(t, size == 3)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeListWithNilOnly",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.STRING)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeListWithNextWriterError",
			args{
				val: []interface{}{stringInput},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.I08)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.I08},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
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
				out: &mocks.MockThriftTTransport{
					WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
						test.Assert(t, keyType == thrift.STRING)
						test.Assert(t, valueType == thrift.STRING)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInterfaceMapWithNil",
			args{
				val: map[interface{}]interface{}{"hello": "world", "hi": nil, "hey": "kitex"},
				out: &mocks.MockThriftTTransport{
					WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
						test.Assert(t, keyType == thrift.STRING)
						test.Assert(t, valueType == thrift.STRING)
						test.Assert(t, size == 3)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInterfaceMapWithNilOnly",
			args{
				val: map[interface{}]interface{}{"hello": nil},
				out: &mocks.MockThriftTTransport{
					WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
						test.Assert(t, keyType == thrift.STRING)
						test.Assert(t, valueType == thrift.STRING)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeInterfaceMapWithElemNextWriterError",
			args{
				val: map[interface{}]interface{}{"hello": "world"},
				out: &mocks.MockThriftTTransport{
					WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
						test.Assert(t, keyType == thrift.STRING)
						test.Assert(t, valueType == thrift.BOOL)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.BOOL},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
		},
		{
			"writeInterfaceMapWithKeyWriterError",
			args{
				val: map[interface{}]interface{}{"hello": "world"},
				out: &mocks.MockThriftTTransport{
					WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
						test.Assert(t, keyType == thrift.I08)
						test.Assert(t, valueType == thrift.STRING)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.I08},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeInterfaceMap(context.Background(), tt.args.val, tt.args.out, tt.args.t,
				tt.args.opt); (err != nil) != tt.wantErr {
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
				out: &mocks.MockThriftTTransport{
					WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
						test.Assert(t, keyType == thrift.STRING)
						test.Assert(t, valueType == thrift.STRING)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeStringMapWithNil",
			args{
				val: map[string]interface{}{"hello": "world", "hi": nil, "hey": "kitex"},
				out: &mocks.MockThriftTTransport{
					WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
						test.Assert(t, keyType == thrift.STRING)
						test.Assert(t, valueType == thrift.STRING)
						test.Assert(t, size == 3)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeStringMapWithNilOnly",
			args{
				val: map[string]interface{}{"hello": nil},
				out: &mocks.MockThriftTTransport{
					WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
						test.Assert(t, keyType == thrift.STRING)
						test.Assert(t, valueType == thrift.STRING)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"writeStringMapWithElemNextWriterError",
			args{
				val: map[string]interface{}{"hello": "world"},
				out: &mocks.MockThriftTTransport{
					WriteMapBeginFunc: func(keyType, valueType thrift.TType, size int) error {
						test.Assert(t, keyType == thrift.STRING)
						test.Assert(t, valueType == thrift.BOOL)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.MAP,
					Key:    &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.BOOL},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			true,
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
	mockTTransportError := &mocks.MockThriftTTransport{
		WriteStructBeginFunc: func(name string) error {
			test.Assert(t, name == "Demo")
			return nil
		},
		WriteFieldBeginFunc: func(name string, typeID thrift.TType, id int16) error {
			test.Assert(t, name == "strList")
			test.Assert(t, typeID == thrift.LIST)
			test.Assert(t, id == 1)
			return nil
		},
		WriteListBeginFunc: func(elemType thrift.TType, size int) error {
			test.Assert(t, elemType == thrift.STRING)
			return nil
		},
		WriteStringFunc: func(value string) error {
			return errors.New("need STRING type, but got: I64")
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
		{
			"writeStructRequired",
			args{
				val: map[string]interface{}{"hello": nil},
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type: descriptor.STRUCT,
					Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{
						Name: "Demo",
						FieldsByName: map[string]*descriptor.FieldDescriptor{
							"hello": {Name: "hello", ID: 1, Required: true, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
						RequiredFields: map[int32]*descriptor.FieldDescriptor{
							1: {Name: "hello", ID: 1, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
					},
				},
			},
			false,
		},
		{
			"writeStructOptional",
			args{
				val: map[string]interface{}{},
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type: descriptor.STRUCT,
					Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{
						Name: "Demo",
						FieldsByName: map[string]*descriptor.FieldDescriptor{
							"hello": {Name: "hello", ID: 1, Optional: true, DefaultValue: "Hello", Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
					},
				},
			},
			false,
		},
		{
			"writeStructError",
			args{
				val: map[string]interface{}{"strList": []interface{}{int64(123)}},
				out: mockTTransportError,
				t: &descriptor.TypeDescriptor{
					Type: descriptor.STRUCT,
					Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem: &descriptor.TypeDescriptor{Type: descriptor.LIST, Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
					Struct: &descriptor.StructDescriptor{
						Name: "Demo",
						FieldsByName: map[string]*descriptor.FieldDescriptor{
							"strList": {Name: "strList", ID: 1, Type: &descriptor.TypeDescriptor{Type: descriptor.LIST, Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING}}},
						},
					},
				},
			},
			true,
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
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeStruct",
			args{
				val: &descriptor.HTTPRequest{
					Body: map[string]interface{}{"hello": "world"},
				},
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
		{
			"writeStructRequired",
			args{
				val: &descriptor.HTTPRequest{
					Body: map[string]interface{}{"hello": nil},
				},
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
								Required:    true,
								Type:        &descriptor.TypeDescriptor{Type: descriptor.STRING},
								HTTPMapping: descriptor.DefaultNewMapping("hello"),
							},
						},
					},
				},
			},
			false,
		},
		{
			"writeStructDefault",
			args{
				val: &descriptor.HTTPRequest{
					Body: map[string]interface{}{"hello": nil},
				},
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type: descriptor.STRUCT,
					Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{
						Name: "Demo",
						FieldsByName: map[string]*descriptor.FieldDescriptor{
							"hello": {
								Name:         "hello",
								ID:           1,
								DefaultValue: "world",
								Type:         &descriptor.TypeDescriptor{Type: descriptor.STRING},
								HTTPMapping:  descriptor.DefaultNewMapping("hello"),
							},
						},
					},
				},
			},
			false,
		},
		{
			"writeStructOptional",
			args{
				val: &descriptor.HTTPRequest{
					Body: map[string]interface{}{},
				},
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
								Optional:    true,
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

func Test_writeHTTPRequestWithPbBody(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	body, err := getReqPbBody()
	if err != nil {
		t.Error(err)
	}
	req := &descriptor.HTTPRequest{
		GeneralBody: body,
		ContentType: descriptor.MIMEApplicationProtobuf,
	}
	typeDescriptor := &descriptor.TypeDescriptor{
		Type: descriptor.STRUCT,
		Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
		Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
		Struct: &descriptor.StructDescriptor{
			Name: "BizReq",
			FieldsByName: map[string]*descriptor.FieldDescriptor{
				"user_id": {
					Name:        "user_id",
					ID:          1,
					Type:        &descriptor.TypeDescriptor{Type: descriptor.I32},
					HTTPMapping: descriptor.DefaultNewMapping("user_id"),
				},
				"user_name": {
					Name:        "user_name",
					ID:          2,
					Type:        &descriptor.TypeDescriptor{Type: descriptor.STRING},
					HTTPMapping: descriptor.DefaultNewMapping("user_name"),
				},
			},
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"writeStructSuccess",
			args{
				val: req,
				out: &mocks.MockThriftTTransport{
					WriteI32Func: func(value int32) error {
						test.Assert(t, value == 1234)
						return nil
					},
					WriteStringFunc: func(value string) error {
						test.Assert(t, value == "John")
						return nil
					},
				},
				t: typeDescriptor,
			},
			false,
		},
		{
			"writeStructFail",
			args{
				val: req,
				out: &mocks.MockThriftTTransport{
					WriteI32Func: func(value int32) error {
						test.Assert(t, value == 1234)
						return nil
					},
					WriteStringFunc: func(value string) error {
						if value == "John" {
							return fmt.Errorf("MakeSureThisExecuted")
						}
						return nil
					},
				},
				t: typeDescriptor,
			},
			true,
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

func getReqPbBody() (proto.Message, error) {
	path := "main.proto"
	content := `
	package kitex.test.server;
	
	message BizReq {
		optional int32 user_id = 1;
		optional string user_name = 2;
	}
	`

	var pbParser protoparse.Parser
	pbParser.Accessor = protoparse.FileContentsFromMap(map[string]string{path: content})
	fds, err := pbParser.ParseFiles(path)
	if err != nil {
		return nil, err
	}

	md := fds[0].GetMessageTypes()[0]
	msg := proto.NewMessage(md)
	items := map[int]interface{}{1: int32(1234), 2: "John"}
	for id, value := range items {
		err = msg.TrySetFieldByNumber(id, value)
		if err != nil {
			return nil, err
		}
	}

	return msg, nil
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
	dataEmpty := gjson.Parse(`{"hello": nil}`)
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
		{
			"writeJSONRequired",
			args{
				val: &dataEmpty,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type: descriptor.STRUCT,
					Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{
						Name: "Demo",
						FieldsByName: map[string]*descriptor.FieldDescriptor{
							"hello": {Name: "hello", ID: 1, Required: true, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
						RequiredFields: map[int32]*descriptor.FieldDescriptor{
							1: {Name: "hello", ID: 1, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
					},
				},
			},
			false,
		},
		{
			"writeJSONOptional",
			args{
				val: &dataEmpty,
				out: mockTTransport,
				t: &descriptor.TypeDescriptor{
					Type: descriptor.STRUCT,
					Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{
						Name: "Demo",
						FieldsByName: map[string]*descriptor.FieldDescriptor{
							"hello": {Name: "hello", ID: 1, Optional: true, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
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

func Test_writeJSONBase(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		WriteStructBeginFunc: func(name string) error {
			return nil
		},
		WriteFieldBeginFunc: func(name string, typeID thrift.TType, id int16) error {
			return nil
		},
	}
	data := gjson.Parse(`{"hello":"world", "base": {"Extra": {"hello":"world"}}}`)
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"writeJSONBase",
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
							"base": {Name: "base", ID: 255, Type: &descriptor.TypeDescriptor{
								Type:          descriptor.STRUCT,
								IsRequestBase: true,
							}},
						},
						RequiredFields: map[int32]*descriptor.FieldDescriptor{
							1: {Name: "hello", ID: 1, Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
						},
					},
				},
				opt: &writerOption{
					requestBase: &Base{
						LogID:  "logID-12345",
						Caller: "Caller.Name",
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
			test.DeepEqual(t, tt.args.opt.requestBase.Extra, map[string]string{"hello": "world"})
		})
	}
}

func Test_getDefaultValueAndWriter(t *testing.T) {
	type args struct {
		val interface{}
		out thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *writerOption
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"bool",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.BOOL)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.BOOL},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"i08",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.I08)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.I08},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"i16",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.I16)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.I16},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"i32",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.I32)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.I32},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"i64",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.I64)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.I64},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"double",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.DOUBLE)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.DOUBLE},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"stringBinary",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.STRING)
						test.Assert(t, size == 1)
						return nil
					},
				},
				opt: &writerOption{
					binaryWithBase64: true,
				},
				t: &descriptor.TypeDescriptor{
					Type: descriptor.LIST,
					Elem: &descriptor.TypeDescriptor{
						Name: "binary",
						Type: descriptor.STRING,
					},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"stringNonBinary",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.STRING)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type:   descriptor.LIST,
					Elem:   &descriptor.TypeDescriptor{Type: descriptor.STRING},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"list",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type: descriptor.LIST,
					Elem: &descriptor.TypeDescriptor{
						Type: descriptor.LIST,
						Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"set",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type: descriptor.LIST,
					Elem: &descriptor.TypeDescriptor{
						Type: descriptor.SET,
						Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"map",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.MAP)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type: descriptor.LIST,
					Elem: &descriptor.TypeDescriptor{
						Type: descriptor.MAP,
						Key:  &descriptor.TypeDescriptor{Type: descriptor.STRING},
						Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING},
					},
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"struct",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.STRUCT)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type: descriptor.LIST,
					Elem: &descriptor.TypeDescriptor{
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
					Struct: &descriptor.StructDescriptor{},
				},
			},
			false,
		},
		{
			"void",
			args{
				val: []interface{}{nil},
				out: &mocks.MockThriftTTransport{
					WriteListBeginFunc: func(elemType thrift.TType, size int) error {
						test.Assert(t, elemType == thrift.VOID)
						test.Assert(t, size == 1)
						return nil
					},
				},
				t: &descriptor.TypeDescriptor{
					Type: descriptor.LIST,
					Elem: &descriptor.TypeDescriptor{
						Type:   descriptor.VOID,
						Struct: &descriptor.StructDescriptor{},
					},
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
