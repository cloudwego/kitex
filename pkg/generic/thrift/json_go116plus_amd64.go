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

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/dynamicgo/conv/j2t"

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

	var cv j2t.BinaryConv
	// dynamicgo logic
	if !m.hasRequestBase {
		requestBase = nil
	}
	if requestBase != nil {
		cv = j2t.NewBinaryConv(*m.dynamicgoConvOptsWithThriftBase)
	} else {
		cv = j2t.NewBinaryConv(*m.dynamicgoConvOpts)
	}

	// msg is void
	if _, ok := msg.(descriptor.Void); ok {
		if err := m.writeHead(out); err != nil {
			return err
		}
		if err := m.writeFields(ctx, out, Void, nil, nil, nil); err != nil {
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
	dbuf := mcache.Malloc(len(transBuff))[0:0]
	defer mcache.Free(dbuf)

	if err := m.writeHead(out); err != nil {
		return err
	}
	if err := m.writeFields(ctx, out, String, &cv, transBuff, dbuf); err != nil {
		return err
	}
	return writeTail(out)
}

type MsgType int

const (
	Void MsgType = iota
	String
)

func (m *WriteJSON) writeFields(ctx context.Context, out thrift.TProtocol, msgType MsgType, cv *j2t.BinaryConv, transBuff, dbuf []byte) error {
	for _, field := range m.dynamicgoTy.Struct().Fields() {
		// Exception field
		if !m.isClient && field.ID() != 0 {
			// generic server ignore the exception, because no description for exception
			// generic handler just return error
			continue
		}

		if err := out.WriteFieldBegin(field.Name(), field.Type().Type().ToThriftTType(), int16(field.ID())); err != nil {
			return err
		}
		switch msgType {
		case Void:
			if err := out.WriteStructBegin(field.Name()); err != nil {
				return err
			}
			if err := out.WriteFieldStop(); err != nil {
				return err
			}
			if err := out.WriteStructEnd(); err != nil {
				return err
			}
		case String:
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
	if msgType == String {
		tProt, ok := out.(*cthrift.BinaryProtocol)
		if !ok {
			return perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
		}
		buf, err := tProt.ByteBuffer().Malloc(len(dbuf))
		if err != nil {
			return err
		} else {
			// TODO: implement MallocAck() to achieve zero copy
			copy(buf, dbuf)
		}
	}
	return nil
}

func (m *WriteJSON) writeHead(out thrift.TProtocol) error {
	if err := out.WriteStructBegin(m.dynamicgoTy.Struct().Name()); err != nil {
		return err
	}
	return nil
}

func writeTail(out thrift.TProtocol) error {
	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	if err := out.WriteStructEnd(); err != nil {
		return err
	}
	return nil
}
