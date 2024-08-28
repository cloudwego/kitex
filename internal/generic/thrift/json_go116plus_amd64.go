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
	"fmt"
	"io"
	"unsafe"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/j2t"
	dthrift "github.com/cloudwego/dynamicgo/thrift"
	dbase "github.com/cloudwego/dynamicgo/thrift/base"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/thrift/base"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/utils"
)

// Write write json string to out thrift.TProtocol
func (m *WriteJSON) Write(ctx context.Context, out io.Writer, bw *thrift.BinaryWriter, msg interface{}, method string, isClient bool, requestBase *base.Base) error {
	// fallback logic
	if !m.dynamicgoEnabled {
		return m.originalWrite(ctx, out, bw, msg, method, isClient, requestBase)
	}

	// dynamicgo logic
	fnDsc := m.svcDsc.DynamicGoDsc.Functions()[method]
	if fnDsc == nil {
		return fmt.Errorf("missing method: %s in service: %s in dynamicgo", method, m.svcDsc.DynamicGoDsc.Name())
	}
	dynamicgoTypeDsc := fnDsc.Request()
	if !isClient {
		dynamicgoTypeDsc = fnDsc.Response()
	}
	hasRequestBase := fnDsc.HasRequestBase() && isClient

	var cv j2t.BinaryConv
	if !hasRequestBase {
		requestBase = nil
	}
	if requestBase != nil {
		base := (*dbase.Base)(unsafe.Pointer(requestBase))
		ctx = context.WithValue(ctx, conv.CtxKeyThriftReqBase, base)
		cv = j2t.NewBinaryConv(m.convOptsWithThriftBase)
	} else {
		cv = j2t.NewBinaryConv(m.convOpts)
	}

	_, isGRPC := out.(remote.FrameWrite)

	// msg is void or nil
	if _, ok := msg.(descriptor.Void); ok || msg == nil {
		if err := writeFields(ctx, out, bw, dynamicgoTypeDsc, nil, nil, isClient, isGRPC); err != nil {
			return err
		}
		bw.WriteFieldStop()
		if isGRPC {
			return nil
		}
		_, err := out.Write(bw.Bytes())
		return err
	}

	// msg is string
	s, ok := msg.(string)
	if !ok {
		return perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is not string")
	}
	transBuff := utils.StringToSliceByte(s)

	if isStreaming(m.svcDsc.Functions[method].StreamingMode) {
		// unwrap one struct layer
		dynamicgoTypeDsc = dynamicgoTypeDsc.Struct().FieldById(dthrift.FieldID(getStreamingFieldID(isClient, true))).Type()
		return writeStreamingContentWithDynamicgo(ctx, bw, dynamicgoTypeDsc, &cv, transBuff)
	} else {
		if err := writeFields(ctx, out, bw, dynamicgoTypeDsc, &cv, transBuff, isClient, isGRPC); err != nil {
			return err
		}
		bw.WriteFieldStop()
		if isGRPC {
			return nil
		}
		_, err := out.Write(bw.Bytes())
		return err
	}
}

type MsgType int

func writeFields(ctx context.Context, out io.Writer, bw *thrift.BinaryWriter, dynamicgoTypeDsc *dthrift.TypeDescriptor, cv *j2t.BinaryConv, transBuff []byte, isClient, isGRPC bool) error {
	dbuf := mcache.Malloc(len(transBuff))[0:0]
	defer mcache.Free(dbuf)

	for _, field := range dynamicgoTypeDsc.Struct().Fields() {
		// Exception field
		if !isClient && field.ID() != 0 {
			// generic server ignore the exception, because no description for exception
			// generic handler just return error
			continue
		}

		bw.WriteFieldBegin(thrift.TType(field.Type().Type()), int16(field.ID()))
		// if the field type is void, just write void and return
		if field.Type().Type() == dthrift.VOID {
			bw.WriteFieldStop()
			if isGRPC {
				return nil
			}
			_, err := out.Write(bw.Bytes())
			bw.Reset()
			return err
		} else {
			// encode using dynamicgo
			// json []byte to thrift []byte
			if err := cv.DoInto(ctx, field.Type(), transBuff, &dbuf); err != nil {
				return err
			}
		}
	}
	if isGRPC {
		for _, b := range dbuf {
			bw.WriteByte(int8(b))
		}
		return nil
	}
	if _, err := out.Write(bw.Bytes()); err != nil {
		return err
	}
	bw.Reset()
	_, err := out.Write(dbuf)
	return err
}

func writeStreamingContentWithDynamicgo(ctx context.Context, bw *thrift.BinaryWriter, dynamicgoTypeDsc *dthrift.TypeDescriptor, cv *j2t.BinaryConv, transBuff []byte) error {
	dbuf := mcache.Malloc(len(transBuff))[0:0]
	defer mcache.Free(dbuf)

	if err := cv.DoInto(ctx, dynamicgoTypeDsc, transBuff, &dbuf); err != nil {
		return err
	}
	for _, b := range dbuf {
		bw.WriteByte(int8(b))
	}
	return nil
}
