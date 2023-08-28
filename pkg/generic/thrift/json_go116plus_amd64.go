//go:build amd64 && go1.16
// +build amd64,go1.16

/*
 * Copyright 2023 CloudWeGo Authors
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
	"unsafe"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/j2t"
	dthrift "github.com/cloudwego/dynamicgo/thrift"
	"github.com/cloudwego/dynamicgo/thrift/base"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/utils"
)

// Write write json string to out thrift.TProtocol
func (m *WriteJSON) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	// fallback logic
	if !m.dynamicgoEnabled {
		return m.originalWrite(ctx, out, msg, requestBase)
	}

	// dynamicgo logic
	var cv j2t.BinaryConv
	if !m.hasRequestBase {
		requestBase = nil
	}
	if requestBase != nil {
		base := (*base.Base)(unsafe.Pointer(requestBase))
		ctx = context.WithValue(ctx, conv.CtxKeyThriftReqBase, base)
		cv = j2t.NewBinaryConv(m.convOptsWithThriftBase)
	} else {
		cv = j2t.NewBinaryConv(m.convOpts)
	}

	// msg is void
	if _, ok := msg.(descriptor.Void); ok {
		if err := m.writeHead(out); err != nil {
			return err
		}
		if err := m.writeFields(ctx, out, nil, nil); err != nil {
			return err
		}
		return writeTail(out)
	}

	// msg is string
	s, ok := msg.(string)
	if !ok {
		return perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is not string")
	}
	transBuff := utils.StringToSliceByte(s)

	if err := m.writeHead(out); err != nil {
		return err
	}
	if err := m.writeFields(ctx, out, &cv, transBuff); err != nil {
		return err
	}
	return writeTail(out)
}

type MsgType int

const (
	Void MsgType = iota
	String
)

func (m *WriteJSON) writeFields(ctx context.Context, out thrift.TProtocol, cv *j2t.BinaryConv, transBuff []byte) error {
	dbuf := mcache.Malloc(len(transBuff))[0:0]
	defer mcache.Free(dbuf)

	for _, field := range m.dynamicgoTypeDsc.Struct().Fields() {
		// Exception field
		if !m.isClient && field.ID() != 0 {
			// generic server ignore the exception, because no description for exception
			// generic handler just return error
			continue
		}

		if err := out.WriteFieldBegin(field.Name(), field.Type().Type().ToThriftTType(), int16(field.ID())); err != nil {
			return err
		}
		// if the field type is void, just write void and return
		if field.Type().Type() == dthrift.VOID {
			if err := writeFieldForVoid(field.Name(), out); err != nil {
				return err
			}
			return nil
		} else {
			// encode using dynamicgo
			// json []byte to thrift []byte
			if err := cv.DoInto(ctx, field.Type(), transBuff, &dbuf); err != nil {
				return err
			}
		}
		// WriteFieldEnd has no content
		// if err := out.WriteFieldEnd(); err != nil {
		// 	return err
		// }
	}
	tProt, ok := out.(*cthrift.BinaryProtocol)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
	}
	buf, err := tProt.ByteBuffer().Malloc(len(dbuf))
	if err != nil {
		return err
	}
	// TODO: implement MallocAck() to achieve zero copy
	copy(buf, dbuf)
	return nil
}

func (m *WriteJSON) writeHead(out thrift.TProtocol) error {
	if err := out.WriteStructBegin(m.dynamicgoTypeDsc.Struct().Name()); err != nil {
		return err
	}
	return nil
}

func writeTail(out thrift.TProtocol) error {
	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	return out.WriteStructEnd()
}

func writeFieldForVoid(name string, out thrift.TProtocol) error {
	if err := out.WriteStructBegin(name); err != nil {
		return err
	}
	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	if err := out.WriteStructEnd(); err != nil {
		return err
	}
	return nil
}
