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
	"github.com/cloudwego/dynamicgo/conv/j2t"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/utils"
)

// Write write json string to out thrift.TProtocol
func (m *WriteJSON) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	if m.enableDynamicgo {
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

		buf := make([]byte, fieldBeginLen, msgLen+structWrapLen)
		offset := 0
		offset += bthrift.Binary.WriteStructBegin(buf[offset:], "")
		if m.isClient {
			offset += bthrift.Binary.WriteFieldBegin(buf[offset:], "", m.dty.Type().ToThriftTType(), 1)
		} else {
			offset += bthrift.Binary.WriteFieldBegin(buf[offset:], "", m.dty.Type().ToThriftTType(), 0)
		}
		if isString {
			// encode using dynamicgo
			cv := j2t.NewBinaryConv(m.opts)
			v := utils.StringToSliceByte(transBuff)
			// json []byte to thrift []byte
			err := cv.DoInto(ctx, m.dty, v, &buf)
			if err != nil {
				return err
			}
			offset = len(buf)
		} else {
			offset += bthrift.Binary.WriteStructEnd(buf[offset:])
			buf = append(buf, 0)
			offset += bthrift.Binary.WriteFieldStop(buf[offset:])
			offset += bthrift.Binary.WriteStructEnd(buf[offset:])
		}
		offset += bthrift.Binary.WriteFieldEnd(buf[offset:])
		buf = append(buf, 0)
		offset += bthrift.Binary.WriteFieldStop(buf[offset:])
		bthrift.Binary.WriteStructEnd(buf[offset:])

		tProt, ok := out.(*cthrift.BinaryProtocol)
		if !ok {
			return perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
		}
		_, err := tProt.ByteBuffer().WriteBinary(buf)
		return err
	}
	return m.originalWrite(ctx, out, msg, requestBase)
}
