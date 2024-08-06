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
	"encoding/binary"
	"math"
	"sync"

	athrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

// MessageReader read from athrift.TProtocol
// Deprecated: use github.com/apache/thrift/lib/go/thrift.TStruct
type MessageReader interface {
	Read(iprot athrift.TProtocol) error
}

// MessageWriter write to athrift.TProtocol
// Deprecated: use github.com/apache/thrift/lib/go/thrift.TStruct
type MessageWriter interface {
	Write(oprot athrift.TProtocol) error
}

// UnmarshalThriftException decode thrift exception from tProt
// TODO: this func should be removed in the future. it's exposed accidentally.
// Deprecated: Use `SkipDecoder` + `ApplicationException` of `cloudwego/gopkg/protocol/thrift` instead.
func UnmarshalThriftException(tProt athrift.TProtocol) error {
	return unmarshalThriftException(tProt.Transport())
}

var bpPool = sync.Pool{
	New: func() interface{} {
		return &BinaryProtocol{}
	},
}

// BinaryProtocol ...
// Deprecated: use github.com/apache/thrift/lib/go/thrift.NewTBinaryProtocol
type BinaryProtocol struct {
	trans remote.ByteBuffer
}

var _ athrift.TProtocol = (*BinaryProtocol)(nil)

// NewBinaryProtocol ...
// Deprecated: use github.com/apache/thrift/lib/go/thrift.NewTBinaryProtocol
func NewBinaryProtocol(t remote.ByteBuffer) *BinaryProtocol {
	bp := bpPool.Get().(*BinaryProtocol)
	bp.trans = t
	return bp
}

// Recycle ...
func (p *BinaryProtocol) Recycle() {
	p.trans = nil
	bpPool.Put(p)
}

/**
 * Writing Methods
 */

// WriteMessageBegin ...
func (p *BinaryProtocol) WriteMessageBegin(name string, typeID athrift.TMessageType, seqID int32) error {
	version := uint32(athrift.VERSION_1) | uint32(typeID)
	e := p.WriteI32(int32(version))
	if e != nil {
		return e
	}
	e = p.WriteString(name)
	if e != nil {
		return e
	}
	e = p.WriteI32(seqID)
	return e
}

// WriteMessageEnd ...
func (p *BinaryProtocol) WriteMessageEnd() error {
	return nil
}

// WriteStructBegin ...
func (p *BinaryProtocol) WriteStructBegin(name string) error {
	return nil
}

// WriteStructEnd ...
func (p *BinaryProtocol) WriteStructEnd() error {
	return nil
}

// WriteFieldBegin ...
func (p *BinaryProtocol) WriteFieldBegin(name string, typeID athrift.TType, id int16) error {
	e := p.WriteByte(int8(typeID))
	if e != nil {
		return e
	}
	e = p.WriteI16(id)
	return e
}

// WriteFieldEnd ...
func (p *BinaryProtocol) WriteFieldEnd() error {
	return nil
}

// WriteFieldStop ...
func (p *BinaryProtocol) WriteFieldStop() error {
	e := p.WriteByte(athrift.STOP)
	return e
}

// WriteMapBegin ...
func (p *BinaryProtocol) WriteMapBegin(keyType, valueType athrift.TType, size int) error {
	e := p.WriteByte(int8(keyType))
	if e != nil {
		return e
	}
	e = p.WriteByte(int8(valueType))
	if e != nil {
		return e
	}
	e = p.WriteI32(int32(size))
	return e
}

// WriteMapEnd ...
func (p *BinaryProtocol) WriteMapEnd() error {
	return nil
}

// WriteListBegin ...
func (p *BinaryProtocol) WriteListBegin(elemType athrift.TType, size int) error {
	e := p.WriteByte(int8(elemType))
	if e != nil {
		return e
	}
	e = p.WriteI32(int32(size))
	return e
}

// WriteListEnd ...
func (p *BinaryProtocol) WriteListEnd() error {
	return nil
}

// WriteSetBegin ...
func (p *BinaryProtocol) WriteSetBegin(elemType athrift.TType, size int) error {
	e := p.WriteByte(int8(elemType))
	if e != nil {
		return e
	}
	e = p.WriteI32(int32(size))
	return e
}

// WriteSetEnd ...
func (p *BinaryProtocol) WriteSetEnd() error {
	return nil
}

// WriteBool ...
func (p *BinaryProtocol) WriteBool(value bool) error {
	if value {
		return p.WriteByte(1)
	}
	return p.WriteByte(0)
}

// WriteByte ...
func (p *BinaryProtocol) WriteByte(value int8) error {
	v, err := p.malloc(1)
	if err != nil {
		return err
	}
	v[0] = byte(value)
	return err
}

// WriteI16 ...
func (p *BinaryProtocol) WriteI16(value int16) error {
	v, err := p.malloc(2)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint16(v, uint16(value))
	return err
}

// WriteI32 ...
func (p *BinaryProtocol) WriteI32(value int32) error {
	v, err := p.malloc(4)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint32(v, uint32(value))
	return err
}

// WriteI64 ...
func (p *BinaryProtocol) WriteI64(value int64) error {
	v, err := p.malloc(8)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint64(v, uint64(value))
	return err
}

// WriteDouble ...
func (p *BinaryProtocol) WriteDouble(value float64) error {
	return p.WriteI64(int64(math.Float64bits(value)))
}

// WriteString ...
func (p *BinaryProtocol) WriteString(value string) error {
	len := len(value)
	e := p.WriteI32(int32(len))
	if e != nil {
		return e
	}
	_, e = p.trans.WriteString(value)
	return e
}

// WriteBinary ...
func (p *BinaryProtocol) WriteBinary(value []byte) error {
	e := p.WriteI32(int32(len(value)))
	if e != nil {
		return e
	}
	_, e = p.trans.WriteBinary(value)
	return e
}

// malloc ...
func (p *BinaryProtocol) malloc(size int) ([]byte, error) {
	buf, err := p.trans.Malloc(size)
	if err != nil {
		return buf, perrors.NewProtocolError(err)
	}
	return buf, nil
}

/**
 * Reading methods
 */

// ReadMessageBegin ...
func (p *BinaryProtocol) ReadMessageBegin() (name string, typeID athrift.TMessageType, seqID int32, err error) {
	size, e := p.ReadI32()
	if e != nil {
		return "", typeID, 0, perrors.NewProtocolError(e)
	}
	if size > 0 {
		return name, typeID, seqID, perrors.NewProtocolErrorWithType(perrors.BadVersion, "Missing version in ReadMessageBegin")
	}
	typeID = athrift.TMessageType(size & 0x0ff)
	version := int64(int64(size) & athrift.VERSION_MASK)
	if version != athrift.VERSION_1 {
		return name, typeID, seqID, perrors.NewProtocolErrorWithType(perrors.BadVersion, "Bad version in ReadMessageBegin")
	}
	name, e = p.ReadString()
	if e != nil {
		return name, typeID, seqID, perrors.NewProtocolError(e)
	}
	seqID, e = p.ReadI32()
	if e != nil {
		return name, typeID, seqID, perrors.NewProtocolError(e)
	}
	return name, typeID, seqID, nil
}

// ReadMessageEnd ...
func (p *BinaryProtocol) ReadMessageEnd() error {
	return nil
}

// ReadStructBegin ...
func (p *BinaryProtocol) ReadStructBegin() (name string, err error) {
	return
}

// ReadStructEnd ...
func (p *BinaryProtocol) ReadStructEnd() error {
	return nil
}

// ReadFieldBegin ...
func (p *BinaryProtocol) ReadFieldBegin() (name string, typeID athrift.TType, id int16, err error) {
	t, err := p.ReadByte()
	typeID = athrift.TType(t)
	if err != nil {
		return name, typeID, id, err
	}
	if t != athrift.STOP {
		id, err = p.ReadI16()
	}
	return name, typeID, id, err
}

// ReadFieldEnd ...
func (p *BinaryProtocol) ReadFieldEnd() error {
	return nil
}

// ReadMapBegin ...
func (p *BinaryProtocol) ReadMapBegin() (kType, vType athrift.TType, size int, err error) {
	k, e := p.ReadByte()
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	kType = athrift.TType(k)
	v, e := p.ReadByte()
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	vType = athrift.TType(v)
	size32, e := p.ReadI32()
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	if size32 < 0 {
		err = perrors.InvalidDataLength
		return
	}
	size = int(size32)
	return kType, vType, size, nil
}

// ReadMapEnd ...
func (p *BinaryProtocol) ReadMapEnd() error {
	return nil
}

// ReadListBegin ...
func (p *BinaryProtocol) ReadListBegin() (elemType athrift.TType, size int, err error) {
	b, e := p.ReadByte()
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	elemType = athrift.TType(b)
	size32, e := p.ReadI32()
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	if size32 < 0 {
		err = perrors.InvalidDataLength
		return
	}
	size = int(size32)

	return
}

// ReadListEnd ...
func (p *BinaryProtocol) ReadListEnd() error {
	return nil
}

// ReadSetBegin ...
func (p *BinaryProtocol) ReadSetBegin() (elemType athrift.TType, size int, err error) {
	b, e := p.ReadByte()
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	elemType = athrift.TType(b)
	size32, e := p.ReadI32()
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	if size32 < 0 {
		err = perrors.InvalidDataLength
		return
	}
	size = int(size32)
	return elemType, size, nil
}

// ReadSetEnd ...
func (p *BinaryProtocol) ReadSetEnd() error {
	return nil
}

// ReadBool ...
func (p *BinaryProtocol) ReadBool() (bool, error) {
	b, e := p.ReadByte()
	v := true
	if b != 1 {
		v = false
	}
	return v, e
}

// ReadByte ...
func (p *BinaryProtocol) ReadByte() (value int8, err error) {
	buf, err := p.next(1)
	if err != nil {
		return value, err
	}
	return int8(buf[0]), err
}

// ReadI16 ...
func (p *BinaryProtocol) ReadI16() (value int16, err error) {
	buf, err := p.next(2)
	if err != nil {
		return value, err
	}
	value = int16(binary.BigEndian.Uint16(buf))
	return value, err
}

// ReadI32 ...
func (p *BinaryProtocol) ReadI32() (value int32, err error) {
	buf, err := p.next(4)
	if err != nil {
		return value, err
	}
	value = int32(binary.BigEndian.Uint32(buf))
	return value, err
}

// ReadI64 ...
func (p *BinaryProtocol) ReadI64() (value int64, err error) {
	buf, err := p.next(8)
	if err != nil {
		return value, err
	}
	value = int64(binary.BigEndian.Uint64(buf))
	return value, err
}

// ReadDouble ...
func (p *BinaryProtocol) ReadDouble() (value float64, err error) {
	buf, err := p.next(8)
	if err != nil {
		return value, err
	}
	value = math.Float64frombits(binary.BigEndian.Uint64(buf))
	return value, err
}

// ReadString ...
func (p *BinaryProtocol) ReadString() (value string, err error) {
	size, e := p.ReadI32()
	if e != nil {
		return "", e
	}
	if size < 0 {
		err = perrors.InvalidDataLength
		return
	}
	value, err = p.trans.ReadString(int(size))
	if err != nil {
		return value, perrors.NewProtocolError(err)
	}
	return value, nil
}

// ReadBinary ...
func (p *BinaryProtocol) ReadBinary() ([]byte, error) {
	size, e := p.ReadI32()
	if e != nil {
		return nil, e
	}
	if size < 0 {
		return nil, perrors.InvalidDataLength
	}
	return p.trans.ReadBinary(int(size))
}

// Flush ...
func (p *BinaryProtocol) Flush(ctx context.Context) (err error) {
	err = p.trans.Flush()
	if err != nil {
		return perrors.NewProtocolError(err)
	}
	return nil
}

// Skip ...
func (p *BinaryProtocol) Skip(fieldType athrift.TType) (err error) {
	return athrift.SkipDefaultDepth(p, fieldType)
}

// ttransportByteBuffer ...
// for exposing remote.ByteBuffer via p.Transport(),
// mainly for testing purpose, see internal/mocks/athrift/utils.go
type ttransportByteBuffer struct {
	remote.ByteBuffer
}

func (ttransportByteBuffer) Close() error                          { panic("not implemented") }
func (ttransportByteBuffer) Flush(ctx context.Context) (err error) { panic("not implemented") }
func (ttransportByteBuffer) IsOpen() bool                          { panic("not implemented") }
func (ttransportByteBuffer) Open() error                           { panic("not implemented") }
func (p ttransportByteBuffer) RemainingBytes() uint64              { return uint64(p.ReadableLen()) }

// Transport ...
func (p *BinaryProtocol) Transport() athrift.TTransport {
	return ttransportByteBuffer{p.trans}
}

// ByteBuffer ...
func (p *BinaryProtocol) ByteBuffer() remote.ByteBuffer {
	return p.trans
}

// next ...
func (p *BinaryProtocol) next(size int) ([]byte, error) {
	buf, err := p.trans.Next(size)
	if err != nil {
		return buf, perrors.NewProtocolError(err)
	}
	return buf, nil
}
