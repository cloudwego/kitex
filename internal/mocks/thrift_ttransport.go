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

package mocks

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
)

type MockThriftTTransport struct {
	WriteMessageBeginFunc func(name string, typeID thrift.TMessageType, seqID int32) error
	WriteMessageEndFunc   func() error
	WriteStructBeginFunc  func(name string) error
	WriteStructEndFunc    func() error
	WriteFieldBeginFunc   func(name string, typeID thrift.TType, id int16) error
	WriteFieldEndFunc     func() error
	WriteFieldStopFunc    func() error
	WriteMapBeginFunc     func(keyType, valueType thrift.TType, size int) error
	WriteMapEndFunc       func() error
	WriteListBeginFunc    func(elemType thrift.TType, size int) error
	WriteListEndFunc      func() error
	WriteSetBeginFunc     func(elemType thrift.TType, size int) error
	WriteSetEndFunc       func() error
	WriteBoolFunc         func(value bool) error
	WriteByteFunc         func(value int8) error
	WriteI16Func          func(value int16) error
	WriteI32Func          func(value int32) error
	WriteI64Func          func(value int64) error
	WriteDoubleFunc       func(value float64) error
	WriteStringFunc       func(value string) error
	WriteBinaryFunc       func(value []byte) error
	ReadMessageBeginFunc  func() (name string, typeID thrift.TMessageType, seqID int32, err error)
	ReadMessageEndFunc    func() error
	ReadStructBeginFunc   func() (name string, err error)
	ReadStructEndFunc     func() error
	ReadFieldBeginFunc    func() (name string, typeID thrift.TType, id int16, err error)
	ReadFieldEndFunc      func() error
	ReadMapBeginFunc      func() (keyType, valueType thrift.TType, size int, err error)
	ReadMapEndFunc        func() error
	ReadListBeginFunc     func() (elemType thrift.TType, size int, err error)
	ReadListEndFunc       func() error
	ReadSetBeginFunc      func() (elemType thrift.TType, size int, err error)
	ReadSetEndFunc        func() error
	ReadBoolFunc          func() (value bool, err error)
	ReadByteFunc          func() (value int8, err error)
	ReadI16Func           func() (value int16, err error)
	ReadI32Func           func() (value int32, err error)
	ReadI64Func           func() (value int64, err error)
	ReadDoubleFunc        func() (value float64, err error)
	ReadStringFunc        func() (value string, err error)
	ReadBinaryFunc        func() (value []byte, err error)
	SkipFunc              func(fieldType thrift.TType) (err error)
	FlushFunc             func(ctx context.Context) (err error)
	TransportFunc         func() thrift.TTransport
}

func (m *MockThriftTTransport) WriteMessageBegin(name string, typeID thrift.TMessageType, seqID int32) error {
	if m.WriteMessageBeginFunc != nil {
		return m.WriteMessageBeginFunc(name, typeID, seqID)
	}
	return nil
}

func (m *MockThriftTTransport) WriteMessageEnd() error {
	if m.WriteMessageEndFunc != nil {
		return m.WriteMessageEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) WriteStructBegin(name string) error {
	if m.WriteStructBeginFunc != nil {
		return m.WriteStructBeginFunc(name)
	}
	return nil
}

func (m *MockThriftTTransport) WriteStructEnd() error {
	if m.WriteStructEndFunc != nil {
		return m.WriteStructEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) WriteFieldBegin(name string, typeID thrift.TType, id int16) error {
	if m.WriteFieldBeginFunc != nil {
		return m.WriteFieldBeginFunc(name, typeID, id)
	}
	return nil
}

func (m *MockThriftTTransport) WriteFieldEnd() error {
	if m.WriteFieldEndFunc != nil {
		return m.WriteFieldEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) WriteFieldStop() error {
	if m.WriteFieldStopFunc != nil {
		return m.WriteFieldStopFunc()
	}
	return nil
}

func (m *MockThriftTTransport) WriteMapBegin(keyType, valueType thrift.TType, size int) error {
	if m.WriteMapBeginFunc != nil {
		return m.WriteMapBeginFunc(keyType, valueType, size)
	}
	return nil
}

func (m *MockThriftTTransport) WriteMapEnd() error {
	if m.WriteMapEndFunc != nil {
		return m.WriteMapEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) WriteListBegin(elemType thrift.TType, size int) error {
	if m.WriteListBeginFunc != nil {
		return m.WriteListBeginFunc(elemType, size)
	}
	return nil
}

func (m *MockThriftTTransport) WriteListEnd() error {
	if m.WriteListEndFunc != nil {
		return m.WriteListEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) WriteSetBegin(elemType thrift.TType, size int) error {
	if m.WriteSetBeginFunc != nil {
		return m.WriteSetBeginFunc(elemType, size)
	}
	return nil
}

func (m *MockThriftTTransport) WriteSetEnd() error {
	if m.WriteSetEndFunc != nil {
		return m.WriteSetEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) WriteBool(value bool) error {
	if m.WriteBoolFunc != nil {
		return m.WriteBoolFunc(value)
	}
	return nil
}

func (m *MockThriftTTransport) WriteByte(value int8) error {
	if m.WriteByteFunc != nil {
		return m.WriteByteFunc(value)
	}
	return nil
}

func (m *MockThriftTTransport) WriteI16(value int16) error {
	if m.WriteI16Func != nil {
		return m.WriteI16Func(value)
	}
	return nil
}

func (m *MockThriftTTransport) WriteI32(value int32) error {
	if m.WriteI32Func != nil {
		return m.WriteI32Func(value)
	}
	return nil
}

func (m *MockThriftTTransport) WriteI64(value int64) error {
	if m.WriteI64Func != nil {
		return m.WriteI64Func(value)
	}
	return nil
}

func (m *MockThriftTTransport) WriteDouble(value float64) error {
	if m.WriteDoubleFunc != nil {
		return m.WriteDoubleFunc(value)
	}
	return nil
}

func (m *MockThriftTTransport) WriteString(value string) error {
	if m.WriteStringFunc != nil {
		return m.WriteStringFunc(value)
	}
	return nil
}

func (m *MockThriftTTransport) WriteBinary(value []byte) error {
	if m.WriteBinaryFunc != nil {
		return m.WriteBinaryFunc(value)
	}
	return nil
}

func (m *MockThriftTTransport) ReadMessageBegin() (name string, typeID thrift.TMessageType, seqID int32, err error) {
	if m.ReadMessageBeginFunc != nil {
		return m.ReadMessageBeginFunc()
	}
	return "", thrift.INVALID_TMESSAGE_TYPE, 0, nil
}

func (m *MockThriftTTransport) ReadMessageEnd() error {
	if m.ReadMessageEndFunc != nil {
		return m.ReadMessageEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) ReadStructBegin() (name string, err error) {
	if m.ReadStructBeginFunc != nil {
		return m.ReadStructBeginFunc()
	}
	return "", nil
}

func (m *MockThriftTTransport) ReadStructEnd() error {
	if m.ReadStructEndFunc != nil {
		return m.ReadStructEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) ReadFieldBegin() (name string, typeID thrift.TType, id int16, err error) {
	if m.ReadFieldBeginFunc != nil {
		return m.ReadFieldBeginFunc()
	}
	return "", thrift.STOP, 0, nil
}

func (m *MockThriftTTransport) ReadFieldEnd() error {
	if m.ReadFieldEndFunc != nil {
		return m.ReadFieldEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) ReadMapBegin() (keyType, valueType thrift.TType, size int, err error) {
	if m.ReadMapBeginFunc != nil {
		return m.ReadMapBeginFunc()
	}
	return thrift.STOP, thrift.STOP, 0, nil
}

func (m *MockThriftTTransport) ReadMapEnd() error {
	if m.ReadMapEndFunc != nil {
		return m.ReadMapEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) ReadListBegin() (elemType thrift.TType, size int, err error) {
	if m.ReadListBeginFunc != nil {
		return m.ReadListBeginFunc()
	}
	return thrift.STOP, 0, nil
}

func (m *MockThriftTTransport) ReadListEnd() error {
	if m.ReadListEndFunc != nil {
		return m.ReadListEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) ReadSetBegin() (elemType thrift.TType, size int, err error) {
	if m.ReadSetBeginFunc != nil {
		return m.ReadSetBeginFunc()
	}
	return thrift.STOP, 0, nil
}

func (m *MockThriftTTransport) ReadSetEnd() error {
	if m.ReadSetEndFunc != nil {
		return m.ReadSetEndFunc()
	}
	return nil
}

func (m *MockThriftTTransport) ReadBool() (value bool, err error) {
	if m.ReadBoolFunc != nil {
		return m.ReadBoolFunc()
	}
	return false, nil
}

func (m *MockThriftTTransport) ReadByte() (value int8, err error) {
	if m.ReadByteFunc != nil {
		return m.ReadByteFunc()
	}
	return 0, nil
}

func (m *MockThriftTTransport) ReadI16() (value int16, err error) {
	if m.ReadI16Func != nil {
		return m.ReadI16Func()
	}
	return 0, nil
}

func (m *MockThriftTTransport) ReadI32() (value int32, err error) {
	if m.ReadI32Func != nil {
		return m.ReadI32Func()
	}
	return 0, nil
}

func (m *MockThriftTTransport) ReadI64() (value int64, err error) {
	if m.ReadI64Func != nil {
		return m.ReadI64Func()
	}
	return 0, nil
}

func (m *MockThriftTTransport) ReadDouble() (value float64, err error) {
	if m.ReadDoubleFunc != nil {
		return m.ReadDoubleFunc()
	}
	return 0.0, nil
}

func (m *MockThriftTTransport) ReadString() (value string, err error) {
	if m.ReadStringFunc != nil {
		return m.ReadStringFunc()
	}
	return "", nil
}

func (m *MockThriftTTransport) ReadBinary() (value []byte, err error) {
	if m.ReadBinaryFunc != nil {
		return m.ReadBinaryFunc()
	}
	return nil, nil
}

func (m *MockThriftTTransport) Skip(fieldType thrift.TType) (err error) {
	if m.SkipFunc != nil {
		return m.SkipFunc(fieldType)
	}
	return nil
}

func (m *MockThriftTTransport) Flush(ctx context.Context) (err error) {
	if m.FlushFunc != nil {
		return m.FlushFunc(ctx)
	}
	return nil
}

func (m *MockThriftTTransport) Transport() thrift.TTransport {
	if m.TransportFunc != nil {
		return m.TransportFunc()
	}
	return nil
}
