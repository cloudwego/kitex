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
	dthrift "github.com/cloudwego/dynamicgo/thrift"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/utils"
)

// NewWriteJSON build WriteJSON according to ServiceDescriptor
func NewWriteJSON(svc *descriptor.ServiceDescriptor, method string, isClient bool) (*WriteJSON, error) {
	fnDsc, err := svc.LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	ty := fnDsc.Request
	if !isClient {
		ty = fnDsc.Response
	}
	var dty *dthrift.TypeDescriptor
	if svc.DynamicgoDsc != nil {
		dty = svc.DynamicgoDsc.Functions()[method].Request()
		if !isClient {
			dty = svc.DynamicgoDsc.Functions()[method].Response()
		}
		if dty == nil {
			return nil, perrors.NewProtocolErrorWithMsg("dynamicgo TypeDescriptor is nil")
		}
	}
	ws := &WriteJSON{
		ty:             ty,
		dty:            dty,
		hasRequestBase: fnDsc.HasRequestBase && isClient,
		isClient:       isClient,
	}
	return ws, nil
}

// Write write json string to out thrift.TProtocol
func (m *WriteJSON) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	if !m.hasRequestBase {
		requestBase = nil
	}
	if requestBase != nil {
		m.dyOpts.EnableThriftBase = true
	}

	_, isVoid := msg.(descriptor.Void)
	transBuff, isString := msg.(string)
	if !isVoid && !isString {
		return perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is neither void nor string")
	}

	tProt, ok := out.(*cthrift.BinaryProtocol)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
	}

	if err := out.WriteStructBegin(m.dty.Struct().Name()); err != nil {
		return err
	}

	for _, field := range m.dty.Struct().Fields() {
		// Exception field
		if !m.isClient && field.ID() != 0 {
			// generic server ignore the exception, because no description for exception
			// generic handler just return error
			continue
		}

		if err := out.WriteFieldBegin(field.Name(), field.Type().Type().ToThriftTType(), int16(field.ID())); err != nil {
			return err
		}

		if isVoid { // msg is void
			if err := out.WriteStructBegin(field.Name()); err != nil {
				return err
			}
			if err := out.WriteFieldStop(); err != nil {
				return err
			}
			if err := out.WriteStructEnd(); err != nil {
				return err
			}
		} else { // msg is string
			// encode using dynamicgo
			cv := j2t.NewBinaryConv(*m.dyOpts)
			v := utils.StringToSliceByte(transBuff)

			// TODO: use Malloc and MallocAck
			dbuf := mcache.Malloc(len(transBuff))[0:0]
			// json []byte to thrift []byte
			if err := cv.DoInto(ctx, field.Type(), v, &dbuf); err != nil {
				return err
			}
			buf, err := tProt.ByteBuffer().Malloc(len(dbuf))
			if err != nil {
				return err
			} else {
				copy(buf, dbuf)
			}
			mcache.Free(dbuf)
		}

		if err := out.WriteFieldEnd(); err != nil {
			return err
		}
	}

	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	if err := out.WriteStructEnd(); err != nil {
		return err
	}
	return nil
}
