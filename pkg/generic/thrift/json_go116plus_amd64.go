//go:build amd64 && go1.16
// +build amd64,go1.16

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

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/dynamicgo/conv/j2t"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/utils"
)

// Write write json string to out thrift.TProtocol
func (m *WriteJSON) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	if !m.enableDynamicgo {
		return m.originalWrite(ctx, out, msg, requestBase)
	}

	// dynamicgo logic
	var msgLen int
	_, isVoid := msg.(descriptor.Void)
	transBuff, isString := msg.(string)
	if !isVoid && !isString {
		return perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is neither void nor string")
	} else if isVoid {
		msgLen = 0
	} else {
		msgLen = len(transBuff)
	}

	if m.dty == nil {
		return perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDsc is nil")
	}

	// TODO: use Malloc and MallocAck to replace WriteBinary
	buf := mcache.Malloc(msgLen + structWrapLen)

	var offset int
	offset += bthrift.Binary.WriteStructBegin(buf[offset:], m.dty.Struct().Name())

	for _, field := range m.dty.Struct().Fields() {
		if m.isClient {
			offset += bthrift.Binary.WriteFieldBegin(buf[offset:], field.Name(), field.Type().Type().ToThriftTType(), int16(field.ID()))
		} else {
			// Exception field
			if field.ID() != 0 {
				// generic server ignore the exception, because no description for exception
				// generic handler just return error
				continue
			}
			offset += bthrift.Binary.WriteFieldBegin(buf[offset:], field.Name(), field.Type().Type().ToThriftTType(), int16(field.ID()))
		}

		if isString {
			// encode using dynamicgo
			cv := j2t.NewBinaryConv(m.opts)
			v := utils.StringToSliceByte(transBuff)
			// json []byte to thrift []byte
			dbuf := buf[offset:offset]
			if err := cv.DoInto(ctx, field.Type(), v, &dbuf); err != nil {
				return err
			} else {
				// sometimes len of thrift []byte gets larger than len of json []byte when writing required/default fields
				if offset+len(dbuf) > len(buf) {
					buf = append(buf, dbuf[len(buf)-offset:]...)
				}
				offset += len(dbuf)
			}
		} else {
			offset += bthrift.Binary.WriteStructEnd(buf[offset:])
			offset += bthrift.Binary.WriteFieldStop(buf[offset:])
			offset += bthrift.Binary.WriteStructEnd(buf[offset:])
		}
		offset += bthrift.Binary.WriteFieldEnd(buf[offset:])
	}

	// in case there is no space to write field stop
	if offset+writeFieldStopLen > len(buf) {
		buf = append(buf, make([]byte, (offset+writeFieldStopLen)-len(buf))...)
	}
	offset += bthrift.Binary.WriteFieldStop(buf[offset:])
	bthrift.Binary.WriteStructEnd(buf[offset:])

	tProt, ok := out.(*cthrift.BinaryProtocol)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
	}
	_, err := tProt.ByteBuffer().WriteBinary(buf[:offset])
	return err
}
