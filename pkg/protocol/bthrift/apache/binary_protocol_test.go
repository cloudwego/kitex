/*
 * Copyright 2025 CloudWeGo Authors
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

package apache

import (
	"context"
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift/internal/test"
	"testing"
)

func TestBinaryProtocolWrite(t *testing.T) {

	var bytes []byte
	w := bufiox.NewBytesWriter(&bytes)
	bp := NewBinaryProtocol(nil, w)
	test.Assert(t, bp.GetBufioxWriter() == w)
	test.Assert(t, bp.WriteMessageBegin("hello", 1, 2) == nil)
	test.Assert(t, bp.WriteStructBegin("struct") == nil)
	test.Assert(t, bp.WriteFieldBegin("fieldName", 3, 4) == nil)
	test.Assert(t, bp.WriteFieldEnd() == nil)
	test.Assert(t, bp.WriteFieldStop() == nil)
	test.Assert(t, bp.WriteStructEnd() == nil)
	test.Assert(t, bp.WriteMapBegin(5, 6, 7) == nil)
	test.Assert(t, bp.WriteMapEnd() == nil)
	test.Assert(t, bp.WriteListBegin(8, 9) == nil)
	test.Assert(t, bp.WriteListEnd() == nil)
	test.Assert(t, bp.WriteSetBegin(10, 11) == nil)
	test.Assert(t, bp.WriteSetEnd() == nil)
	test.Assert(t, bp.WriteBinary([]byte("12")) == nil)
	test.Assert(t, bp.WriteString("13") == nil)
	test.Assert(t, bp.WriteBool(true) == nil)
	test.Assert(t, bp.WriteByte(14) == nil)
	test.Assert(t, bp.WriteI16(15) == nil)
	test.Assert(t, bp.WriteI32(16) == nil)
	test.Assert(t, bp.WriteI64(17) == nil)
	test.Assert(t, bp.WriteDouble(18.5) == nil)
	test.Assert(t, bp.WriteMessageEnd() == nil)
	test.Assert(t, bp.Flush(context.Background()) == nil)

	r := thrift.NewBufferReader(bufiox.NewBytesReader(bytes))

	name, mt, seq, err := r.ReadMessageBegin()
	test.Assert(t, err == nil)
	test.Assert(t, name == "hello")
	test.Assert(t, mt == 1)
	test.Assert(t, seq == int32(2))

	ft, fid, err := r.ReadFieldBegin()
	test.Assert(t, err == nil)
	test.Assert(t, ft == 3, ft)
	test.Assert(t, fid == int16(4), fid)

	ft, fid, err = r.ReadFieldBegin()
	test.Assert(t, err == nil)
	test.Assert(t, ft == STOP, ft)
	test.Assert(t, fid == int16(0), fid)

	kt, vt, sz, err := r.ReadMapBegin()
	test.Assert(t, err == nil)
	test.Assert(t, kt == 5, kt)
	test.Assert(t, vt == 6, vt)
	test.Assert(t, sz == 7, vt)

	et, sz, err := r.ReadListBegin()
	test.Assert(t, err == nil)
	test.Assert(t, et == 8, et)
	test.Assert(t, sz == 9, et)

	et, sz, err = r.ReadSetBegin()
	test.Assert(t, err == nil)
	test.Assert(t, et == 10, et)

	bin, err := r.ReadBinary()
	test.Assert(t, err == nil)
	test.Assert(t, string(bin) == "12", string(bin))

	s, err := r.ReadString()
	test.Assert(t, err == nil)
	test.Assert(t, s == "13", s)

	vb, err := r.ReadBool()
	test.Assert(t, err == nil)
	test.Assert(t, vb)

	v8, err := r.ReadByte()
	test.Assert(t, err == nil)
	test.Assert(t, v8 == int8(14), v8)

	v16, err := r.ReadI16()
	test.Assert(t, err == nil)
	test.Assert(t, v16 == int16(15), v16)

	v32, err := r.ReadI32()
	test.Assert(t, err == nil)
	test.Assert(t, v32 == int32(16), v32)

	v64, err := r.ReadI64()
	test.Assert(t, err == nil)
	test.Assert(t, v64 == int64(17), v64)

	vf, err := r.ReadDouble()
	test.Assert(t, err == nil)
	test.Assert(t, vf == float64(18.5), vf)
}

func TestBinaryProtocolRead(t *testing.T) {

	var bytes []byte
	bw := bufiox.NewBytesWriter(&bytes)
	w := thrift.NewBufferWriter(bw)
	test.Assert(t, w.WriteMessageBegin("hello", 1, 2) == nil)
	// write a bool, we will test skip bool later
	test.Assert(t, w.WriteBool(true) == nil)
	test.Assert(t, w.WriteFieldBegin(3, 4) == nil)
	test.Assert(t, w.WriteFieldStop() == nil)
	test.Assert(t, w.WriteMapBegin(5, 6, 7) == nil)
	test.Assert(t, w.WriteListBegin(8, 9) == nil)
	test.Assert(t, w.WriteSetBegin(10, 11) == nil)
	test.Assert(t, w.WriteBinary([]byte("12")) == nil)
	test.Assert(t, w.WriteString("13") == nil)
	test.Assert(t, w.WriteBool(true) == nil)
	test.Assert(t, w.WriteByte(14) == nil)
	test.Assert(t, w.WriteI16(15) == nil)
	test.Assert(t, w.WriteI32(16) == nil)
	test.Assert(t, w.WriteI64(17) == nil)
	test.Assert(t, w.WriteDouble(18.5) == nil)
	test.Assert(t, bw.Flush() == nil)

	r := bufiox.NewBytesReader(bytes)
	bp := NewBinaryProtocol(r, nil)

	test.Assert(t, bp.GetBufioxReader() == r)

	name, mt, seq, err := bp.ReadMessageBegin()
	test.Assert(t, err == nil)
	test.Assert(t, name == "hello")
	test.Assert(t, mt == 1)
	test.Assert(t, seq == int32(2))

	// skip the bool we write
	test.Assert(t, bp.Skip(TType(2)) == nil)

	_, ft, fid, err := bp.ReadFieldBegin()
	test.Assert(t, err == nil)
	test.Assert(t, ft == 3, ft)
	test.Assert(t, fid == int16(4), fid)

	_, ft, fid, err = bp.ReadFieldBegin()
	test.Assert(t, err == nil)
	test.Assert(t, ft == STOP, ft)
	test.Assert(t, fid == int16(0), fid)

	kt, vt, sz, err := bp.ReadMapBegin()
	test.Assert(t, err == nil)
	test.Assert(t, kt == 5, kt)
	test.Assert(t, vt == 6, vt)
	test.Assert(t, sz == 7, vt)

	et, sz, err := bp.ReadListBegin()
	test.Assert(t, err == nil)
	test.Assert(t, et == 8, et)
	test.Assert(t, sz == 9, et)

	et, sz, err = bp.ReadSetBegin()
	test.Assert(t, err == nil)
	test.Assert(t, et == 10, et)

	bin, err := bp.ReadBinary()
	test.Assert(t, err == nil)
	test.Assert(t, string(bin) == "12", string(bin))

	s, err := bp.ReadString()
	test.Assert(t, err == nil)
	test.Assert(t, s == "13", s)

	vb, err := bp.ReadBool()
	test.Assert(t, err == nil)
	test.Assert(t, vb)

	v8, err := bp.ReadByte()
	test.Assert(t, err == nil)
	test.Assert(t, v8 == int8(14), v8)

	v16, err := bp.ReadI16()
	test.Assert(t, err == nil)
	test.Assert(t, v16 == int16(15), v16)

	v32, err := bp.ReadI32()
	test.Assert(t, err == nil)
	test.Assert(t, v32 == int32(16), v32)

	v64, err := bp.ReadI64()
	test.Assert(t, err == nil)
	test.Assert(t, v64 == int64(17), v64)

	vf, err := bp.ReadDouble()
	test.Assert(t, err == nil)
	test.Assert(t, vf == float64(18.5), vf)

	// these functions should always return nil, because in thrift protocol it's not defined
	test.Assert(t, bp.ReadMessageEnd() == nil)
	emptyName, err := bp.ReadStructBegin()
	test.Assert(t, err == nil)
	test.Assert(t, emptyName == "", emptyName)
	test.Assert(t, bp.ReadStructEnd() == nil)
	test.Assert(t, bp.ReadFieldEnd() == nil)
	test.Assert(t, bp.ReadMapEnd() == nil)
	test.Assert(t, bp.ReadListEnd() == nil)
	test.Assert(t, bp.ReadSetEnd() == nil)
}
