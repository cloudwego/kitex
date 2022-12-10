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
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/jhump/protoreflect/desc/protoparse"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/proto"
)

var (
	stringInput = "hello world"
	binaryInput = []byte(stringInput)
)

func Test_nextReader(t *testing.T) {
	type args struct {
		tt  descriptor.Type
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	tests := []struct {
		name    string
		args    args
		want    reader
		wantErr bool
	}{
		// TODO: Add test cases.
		{"void", args{tt: descriptor.VOID, t: &descriptor.TypeDescriptor{Type: descriptor.VOID}, opt: &readerOption{}}, readVoid, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextReader(tt.args.tt, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("nextReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if fmt.Sprintf("%p", got) != fmt.Sprintf("%p", tt.want) {
				t.Errorf("nextReader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readVoid(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}

	mockTTransport := &mocks.MockThriftTTransport{}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"void", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.VOID}}, descriptor.Void{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readVoid(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readVoid() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readVoid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readDouble(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		ReadDoubleFunc: func() (value float64, err error) {
			return 1.0, nil
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"readDouble", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.DOUBLE}}, 1.0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readDouble(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readDouble() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readDouble() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readBool(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		ReadBoolFunc: func() (bool, error) { return true, nil },
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"readBool", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.BOOL}}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readBool(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readByte(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		ReadByteFunc: func() (int8, error) {
			return 1, nil
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"readByte", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.BYTE}, opt: &readerOption{}}, int8(1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readByte(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readByte() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readByte() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readInt16(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		ReadI16Func: func() (int16, error) {
			return 1, nil
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"readInt16", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.I16}, opt: &readerOption{}}, int16(1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readInt16(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readInt16() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readInt16() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readInt32(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}

	mockTTransport := &mocks.MockThriftTTransport{
		ReadI32Func: func() (int32, error) {
			return 1, nil
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"readInt32", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.I32}}, int32(1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readInt32(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readInt32() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readInt32() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readInt64(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		ReadI64Func: func() (int64, error) {
			return 1, nil
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"readInt64", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.I64}}, int64(1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readInt64(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readInt64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readString(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		ReadStringFunc: func() (string, error) {
			return stringInput, nil
		},
		ReadBinaryFunc: func() ([]byte, error) {
			return binaryInput, nil
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"readString", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.STRING}}, stringInput, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readString(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readBase64String(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		ReadStringFunc: func() (string, error) {
			return stringInput, nil
		},
		ReadBinaryFunc: func() ([]byte, error) {
			return binaryInput, nil
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"readBase64Binary", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Name: "binary", Type: descriptor.STRING}}, base64.StdEncoding.EncodeToString(binaryInput), false}, // read base64 string from binary field
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readBase64Binary(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readList(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	mockTTransport := &mocks.MockThriftTTransport{
		ReadListBeginFunc: func() (elemType thrift.TType, size int, err error) {
			return thrift.STRING, 3, nil
		},

		ReadStringFunc: func() (string, error) {
			return stringInput, nil
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{"readList", args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.LIST, Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING}}}, []interface{}{stringInput, stringInput, stringInput}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readList(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readMap(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	count := 0
	mockTTransport := &mocks.MockThriftTTransport{
		ReadMapBeginFunc: func() (keyType, valueType thrift.TType, size int, err error) {
			return thrift.STRING, thrift.STRING, 1, nil
		},
		ReadStringFunc: func() (string, error) {
			defer func() { count++ }()
			if count%2 == 0 {
				return "hello", nil
			}
			return "world", nil
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"readMap",
			args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.MAP, Key: &descriptor.TypeDescriptor{Type: descriptor.STRING}, Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING}}, opt: &readerOption{}},
			map[interface{}]interface{}{"hello": "world"},
			false,
		},
		{
			"readJsonMap",
			args{in: mockTTransport, t: &descriptor.TypeDescriptor{Type: descriptor.MAP, Key: &descriptor.TypeDescriptor{Type: descriptor.STRING}, Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING}}, opt: &readerOption{forJSON: true}},
			map[string]interface{}{"hello": "world"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readMap(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readStruct(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	read := false
	mockTTransport := &mocks.MockThriftTTransport{
		ReadStructBeginFunc: func() (name string, err error) {
			return "Demo", nil
		},
		ReadFieldBeginFunc: func() (name string, typeID thrift.TType, id int16, err error) {
			if !read {
				read = true
				return "", thrift.STRING, 1, nil
			}
			return "", thrift.STOP, 0, nil
		},
		ReadStringFunc: func() (string, error) {
			return "world", nil
		},
	}
	readError := false
	mockTTransportError := &mocks.MockThriftTTransport{
		ReadStructBeginFunc: func() (name string, err error) {
			return "Demo", nil
		},
		ReadFieldBeginFunc: func() (name string, typeID thrift.TType, id int16, err error) {
			if !readError {
				readError = true
				return "", thrift.LIST, 1, nil
			}
			return "", thrift.STOP, 0, nil
		},
		ReadListBeginFunc: func() (elemType thrift.TType, size int, err error) {
			return thrift.STRING, 1, nil
		},
		ReadStringFunc: func() (string, error) {
			return "123", errors.New("need STRING type, but got: I64")
		},
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"readStruct",
			args{in: mockTTransport, t: &descriptor.TypeDescriptor{
				Type: descriptor.STRUCT,
				Struct: &descriptor.StructDescriptor{
					FieldsByID: map[int32]*descriptor.FieldDescriptor{
						1: {Name: "hello", Type: &descriptor.TypeDescriptor{Type: descriptor.STRING}},
					},
				},
			}},
			map[string]interface{}{"hello": "world"},
			false,
		},
		{
			"readStructError",
			args{in: mockTTransportError, t: &descriptor.TypeDescriptor{
				Type: descriptor.STRUCT,
				Struct: &descriptor.StructDescriptor{
					FieldsByID: map[int32]*descriptor.FieldDescriptor{
						1: {Name: "strList", Type: &descriptor.TypeDescriptor{Type: descriptor.LIST, Elem: &descriptor.TypeDescriptor{Type: descriptor.STRING}}},
					},
				},
			}},
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readStruct(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readStruct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readStruct() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readHTTPResponse(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	read := false
	mockTTransport := &mocks.MockThriftTTransport{
		ReadStructBeginFunc: func() (name string, err error) {
			return "Demo", nil
		},
		ReadFieldBeginFunc: func() (name string, typeID thrift.TType, id int16, err error) {
			if !read {
				read = true
				return "", thrift.STRING, 1, nil
			}
			return "", thrift.STOP, 0, nil
		},
		ReadStringFunc: func() (string, error) {
			return "world", nil
		},
	}
	resp := descriptor.NewHTTPResponse()
	resp.Body = map[string]interface{}{"hello": "world"}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"readHTTPResponse",
			args{in: mockTTransport, t: &descriptor.TypeDescriptor{
				Type: descriptor.STRUCT,
				Struct: &descriptor.StructDescriptor{
					FieldsByID: map[int32]*descriptor.FieldDescriptor{
						1: {
							Name:        "hello",
							Type:        &descriptor.TypeDescriptor{Type: descriptor.STRING},
							HTTPMapping: descriptor.DefaultNewMapping("hello"),
						},
					},
				},
			}},
			resp,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readHTTPResponse(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readHTTPResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readHTTPResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readHTTPResponseWithPbBody(t *testing.T) {
	type args struct {
		in  thrift.TProtocol
		t   *descriptor.TypeDescriptor
		opt *readerOption
	}
	read := false
	mockTTransport := &mocks.MockThriftTTransport{
		ReadStructBeginFunc: func() (name string, err error) {
			return "BizResp", nil
		},
		ReadFieldBeginFunc: func() (name string, typeID thrift.TType, id int16, err error) {
			if !read {
				read = true
				return "", thrift.STRING, 1, nil
			}
			return "", thrift.STOP, 0, nil
		},
		ReadStringFunc: func() (string, error) {
			return "hello world", nil
		},
	}
	desc, err := getRespPbDesc()
	if err != nil {
		t.Error(err)
		return
	}
	tests := []struct {
		name    string
		args    args
		want    map[int]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			"readHTTPResponse",
			args{in: mockTTransport, t: &descriptor.TypeDescriptor{
				Type: descriptor.STRUCT,
				Struct: &descriptor.StructDescriptor{
					FieldsByID: map[int32]*descriptor.FieldDescriptor{
						1: {
							ID:          1,
							Name:        "msg",
							Type:        &descriptor.TypeDescriptor{Type: descriptor.STRING},
							HTTPMapping: descriptor.DefaultNewMapping("msg"),
						},
					},
				},
			}, opt: &readerOption{pbDsc: desc}},
			map[int]interface{}{
				1: "hello world",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readHTTPResponse(context.Background(), tt.args.in, tt.args.t, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("readHTTPResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			respGot := got.(*descriptor.HTTPResponse)
			if respGot.ContentType != descriptor.MIMEApplicationProtobuf {
				t.Errorf("expected content type: %v, got: %v", descriptor.MIMEApplicationProtobuf, respGot.ContentType)
			}
			body := respGot.GeneralBody.(proto.Message)
			for fieldID, expectedVal := range tt.want {
				val, err := body.TryGetFieldByNumber(fieldID)
				if err != nil {
					t.Errorf("get by fieldID [%v] err: %v", fieldID, err)
				}
				if val != expectedVal {
					t.Errorf("expected field value: %v, got: %v", expectedVal, val)
				}
			}
		})
	}
}

func getRespPbDesc() (proto.MessageDescriptor, error) {
	path := "main.proto"
	content := `
	package kitex.test.server;

	message BizResp {
		optional string msg = 1;
	}
	`

	var pbParser protoparse.Parser
	pbParser.Accessor = protoparse.FileContentsFromMap(map[string]string{path: content})
	fds, err := pbParser.ParseFiles(path)
	if err != nil {
		return nil, err
	}

	return fds[0].GetMessageTypes()[0], nil
}
