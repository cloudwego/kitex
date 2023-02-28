//go:build amd64
// +build amd64

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

package generic

import (
	"context"
	"fmt"
	"unsafe"

	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/dynamicgo/conv/j2t"
	dthrift "github.com/cloudwego/dynamicgo/thrift"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
)

type GoSlice struct {
	Ptr unsafe.Pointer
	Len int
	Cap int
}

type GoString struct {
	Ptr unsafe.Pointer
	Len int
}

func (c *jsonThriftCodec) marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	msgType := msg.MessageType()
	if !c.enableDynamicgo || msgType == remote.Exception {
		return c.originalMarshal(ctx, msg, out)
	}

	method := msg.RPCInfo().Invocation().MethodName()
	if method == "" {
		return perrors.NewProtocolErrorWithMsg("empty methodName in thrift Marshal")
	}

	data := msg.Data()
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}
	var tyDsc *dthrift.TypeDescriptor
	if svcDsc.DynamicgoDsc == nil {
		return perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDsc is nil")
	}
	if msgType == remote.Reply {
		tyDsc = svcDsc.DynamicgoDsc.Functions()[method].Response().Struct().Fields()[0].Type()
	} else {
		tyDsc = svcDsc.DynamicgoDsc.Functions()[method].Request().Struct().Fields()[0].Type()
	}

	var transBuff string
	if msg.RPCRole() == remote.Server {
		gResult := data.(*Result)
		transData := gResult.Success
		// TODO: deal with void
		if transBuff, ok = transData.(string); !ok {
			return perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is not string")
		}
	} else {
		gArg := data.(*Args)
		transData := gArg.Request
		if transBuff, ok = transData.(string); !ok {
			return perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is not string")
		}
	}

	// encode in normal way using dynamicgo
	cv := j2t.NewBinaryConv(c.convOpts)
	buf := make([]byte, 0, len(transBuff))
	tProt := cthrift.NewBinaryProtocol(out)
	if err := tProt.WriteMessageBegin(method, athrift.TMessageType(msgType), msg.RPCInfo().Invocation().SeqID()); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshalHTTP, WriteMessageBegin failed: %s", err.Error()))
	}
	if err := tProt.WriteStructBegin(""); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshalHTTP, WriteStructBegin failed: %s", err.Error()))
	}
	if msgType == remote.Reply {
		if err := tProt.WriteFieldBegin("", athrift.STRUCT, 0); err != nil {
			return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshalHTTP, WriteFieldBegin failed: %s", err.Error()))
		}
	} else {
		if err := tProt.WriteFieldBegin("", athrift.STRUCT, 1); err != nil {
			return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshalHTTP, WriteFieldBegin failed: %s", err.Error()))
		}
	}
	// TODO: discuss *(*[]byte)(unsafe.Pointer(&transBuff))
	// encode json binary to thrift binary
	var v []byte
	(*GoSlice)(unsafe.Pointer(&v)).Cap = (*GoString)(unsafe.Pointer(&transBuff)).Len
	(*GoSlice)(unsafe.Pointer(&v)).Len = (*GoString)(unsafe.Pointer(&transBuff)).Len
	(*GoSlice)(unsafe.Pointer(&v)).Ptr = (*GoString)(unsafe.Pointer(&transBuff)).Ptr
	err := cv.DoInto(ctx, tyDsc, v, &buf)
	// err := cv.DoInto(ctx, tyDsc, *(*[]byte)(unsafe.Pointer(&transBuff)), &buf)
	// err := cv.DoInto(ctx, tyDsc, []byte(transBuff), &buf)
	if err != nil {
		return err
	}

	// write thrift []byte
	if _, err = out.WriteBinary(buf); err != nil {
		return err
	}
	tProt.WriteFieldStop()
	if err := tProt.WriteFieldEnd(); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshalHTTP, WriteFieldEnd failed: %s", err.Error()))
	}
	if err := tProt.WriteStructEnd(); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshalHTTP, WriteStructEnd failed: %s", err.Error()))
	}
	if err := tProt.WriteMessageEnd(); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshalHTTP, WriteMessageEnd failed: %s", err.Error()))
	}
	tProt.Recycle()
	return nil
}
