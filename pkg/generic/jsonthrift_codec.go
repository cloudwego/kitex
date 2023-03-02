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
	"strconv"
	"sync"
	"sync/atomic"

	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/t2j"
	dthrift "github.com/cloudwego/dynamicgo/thrift"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	_ remote.PayloadCodec = &jsonThriftCodec{}
	_ Closer              = &jsonThriftCodec{}
)

// JSONRequest alias of string
type JSONRequest = string

type jsonThriftCodec struct {
	svcDsc           atomic.Value // *idl
	provider         DescriptorProvider
	codec            remote.PayloadCodec
	binaryWithBase64 bool
	enableDynamicgo  bool
	convOpts         conv.Options
}

func newJsonThriftCodec(p DescriptorProvider, codec remote.PayloadCodec, opts ...DynamicgoOptions) (*jsonThriftCodec, error) {
	svc := <-p.Provide()
	var c *jsonThriftCodec
	if opts == nil {
		c = &jsonThriftCodec{codec: codec, provider: p, binaryWithBase64: true}
	} else {
		// codec with dynamicgo
		convOpts := opts[0].ConvOpts
		binaryWithBase64 := true
		if convOpts.NoBase64Binary {
			binaryWithBase64 = false
		}
		c = &jsonThriftCodec{codec: codec, provider: p, binaryWithBase64: binaryWithBase64, convOpts: convOpts, enableDynamicgo: true}
	}
	c.svcDsc.Store(svc)
	go c.update()
	return c, nil
}

func (c *jsonThriftCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcDsc.Store(svc)
	}
}

func (c *jsonThriftCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if !c.enableDynamicgo || msg.PayloadLen() == 0 {
		return c.originalUnmarshal(ctx, msg, in)
	}

	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}

	tProt := cthrift.NewBinaryProtocol(in)
	method, msgType, seqID, err := tProt.ReadMessageBegin()
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal, ReadMessageBegin failed: %s", err.Error()))
	}
	if err = codec.UpdateMsgType(uint32(msgType), msg); err != nil {
		return err
	}

	var tyDsc *dthrift.TypeDescriptor
	messageType := msg.MessageType()
	if messageType == remote.Exception {
		exception := athrift.NewTApplicationException(athrift.UNKNOWN_APPLICATION_EXCEPTION, "")
		if err := exception.Read(tProt); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal Exception failed: %s", err.Error()))
		}
		if err := tProt.ReadMessageEnd(); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal, ReadMessageEnd failed: %s", err.Error()))
		}
		return remote.NewTransError(exception.TypeId(), exception)
	} else {
		if svcDsc.DynamicgoDsc == nil {
			return perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDsc is nil")
		}
		if messageType == remote.Reply {
			tyDsc = svcDsc.DynamicgoDsc.Functions()[method].Response().Struct().Fields()[0].Type()
		} else {
			tyDsc = svcDsc.DynamicgoDsc.Functions()[method].Request().Struct().Fields()[0].Type()
		}
	}

	// Must check after Exception handle.
	// For server side, the following error can be sent back and 'SetSeqID' should be executed first to ensure the seqID
	// is right when return Exception back.
	if err = codec.SetOrCheckSeqID(seqID, msg); err != nil {
		return err
	}
	if err = codec.SetOrCheckMethodName(method, msg); err != nil {
		return err
	}

	if err = codec.NewDataIfNeeded(method, msg); err != nil {
		return err
	}

	var resp interface{}
	if tyDsc.Type() == dthrift.VOID {
		tProt.ReadStructBegin()
		in.ReadBinary(bthrift.Binary.FieldStopLength())
		tProt.ReadStructEnd()
		resp = descriptor.Void{}
	} else {
		sname, err := tProt.ReadStructBegin()
		if err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal, ReadStructBegin failed: %s", err.Error()))
		}
		fname, ftype, fid, err := tProt.ReadFieldBegin()
		if err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err,
				fmt.Sprintf("thrift unmarshal, ReadFieldBegin failed: %s", err.Error()))
		}
		msgBeginLen := bthrift.Binary.MessageBeginLength(method, msgType, seqID)
		structBeginLen := bthrift.Binary.StructBeginLength(sname)
		fieldBeginLen := bthrift.Binary.FieldBeginLength(fname, ftype, fid)
		transBuff, err := in.ReadBinary(
			msg.PayloadLen() - msgBeginLen - bthrift.Binary.MessageEndLength() -
				structBeginLen - bthrift.Binary.StructEndLength() -
				fieldBeginLen - bthrift.Binary.FieldEndLength() - bthrift.Binary.FieldStopLength())
		if err != nil {
			return err
		}

		var bytesPool = sync.Pool{
			New: func() interface{} {
				bufLen := float64(len(transBuff)) * 1.5
				b := make([]byte, 0, int(bufLen))
				return &b
			},
		}
		buf := bytesPool.Get().(*[]byte)
		cv := t2j.NewBinaryConv(c.convOpts)
		// decode thrift binary to json binary
		if err := cv.DoInto(ctx, tyDsc, transBuff, buf); err != nil {
			return err
		}
		resp = string(*buf)
		if tyDsc.Type() == dthrift.STRING {
			resp, err = strconv.Unquote(resp.(string))
			if err != nil {
				return err
			}
		}
		*buf = (*buf)[:0]
		bytesPool.Put(buf)
	}

	tProt.ReadFieldEnd()
	in.ReadBinary(bthrift.Binary.FieldStopLength())
	tProt.ReadStructEnd()
	tProt.ReadMessageEnd()
	tProt.Recycle()

	data := msg.Data()
	if msg.RPCRole() == remote.Server {
		gArg := data.(*Args)
		gArg.Method = method
		gArg.Request = resp
	} else {
		gResult := data.(*Result)
		gResult.Success = resp
	}
	return nil
}

func (c *jsonThriftCodec) originalMarshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	method := msg.RPCInfo().Invocation().MethodName()
	if method == "" {
		return perrors.NewProtocolErrorWithMsg("empty methodName in thrift Marshal")
	}
	if msg.MessageType() == remote.Exception {
		return c.codec.Marshal(ctx, msg, out)
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}
	wm, err := thrift.NewWriteJSON(svcDsc, method, msg.RPCRole() == remote.Client)
	if err != nil {
		return err
	}
	wm.SetBase64Binary(c.binaryWithBase64)
	msg.Data().(WithCodec).SetCodec(wm)
	return c.codec.Marshal(ctx, msg, out)
}

func (c *jsonThriftCodec) originalUnmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}
	rm := thrift.NewReadJSON(svcDsc, msg.RPCRole() == remote.Client)
	rm.SetBinaryWithBase64(c.binaryWithBase64)
	msg.Data().(WithCodec).SetCodec(rm)
	return c.codec.Unmarshal(ctx, msg, in)
}

func (c *jsonThriftCodec) getMethod(req interface{}, method string) (*Method, error) {
	fnSvc, err := c.svcDsc.Load().(*descriptor.ServiceDescriptor).LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	return &Method{method, fnSvc.Oneway}, nil
}

func (c *jsonThriftCodec) Name() string {
	if c.enableDynamicgo {
		return "JSONDynamicgoThrift"
	}
	return "JSONThrift"
}

func (c *jsonThriftCodec) Close() error {
	return c.provider.Close()
}
