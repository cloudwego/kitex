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
	"sync/atomic"
	"unsafe"

	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/j2t"
	"github.com/cloudwego/dynamicgo/conv/t2j"
	dthrift "github.com/cloudwego/dynamicgo/thrift"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	_ remote.PayloadCodec = &jsonThriftCodec{}
	_ Closer              = &jsonThriftCodec{}
	_ remote.PayloadCodec = &jsonThriftDynamicgoCodec{}
	_ Closer              = &jsonThriftDynamicgoCodec{}
)

// JSONRequest alias of string
type JSONRequest = string

type jsonThriftCodec struct {
	svcDsc           atomic.Value // *idl
	provider         DescriptorProvider
	codec            remote.PayloadCodec
	binaryWithBase64 bool
}

func newJsonThriftCodec(p DescriptorProvider, codec remote.PayloadCodec) (*jsonThriftCodec, error) {
	svc := <-p.Provide()
	c := &jsonThriftCodec{codec: codec, provider: p, binaryWithBase64: true}
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

func (c *jsonThriftCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
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

func (c *jsonThriftCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
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
	return "JSONThrift"
}

func (c *jsonThriftCodec) Close() error {
	return c.provider.Close()
}

type jsonThriftDynamicgoCodec struct {
	svcDsc   atomic.Value // *idl
	provider DynamicgoDescriptorProvider
	codec    remote.PayloadCodec
	convOpts conv.Options
}

func newJsonThriftDynamicgoCodec(p DynamicgoDescriptorProvider, codec remote.PayloadCodec, convOpts conv.Options) (*jsonThriftDynamicgoCodec, error) {
	svc := <-p.Provide()
	c := &jsonThriftDynamicgoCodec{codec: codec, provider: p, convOpts: convOpts}
	c.svcDsc.Store(svc)
	go c.update()
	return c, nil
}

func (c *jsonThriftDynamicgoCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcDsc.Store(svc)
	}
}

func (c *jsonThriftDynamicgoCodec) getMethod(req interface{}, method string) (*Method, error) {
	fnSvc, err := c.svcDsc.Load().(*dthrift.ServiceDescriptor).LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	return &Method{method, fnSvc.Oneway()}, nil
}

func (c *jsonThriftDynamicgoCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	method := msg.RPCInfo().Invocation().MethodName()
	if method == "" {
		return perrors.NewProtocolErrorWithMsg("empty methodName in thrift Marshal")
	}
	if err := codec.NewDataIfNeeded(method, msg); err != nil {
		return err
	}

	msgType := msg.MessageType()
	svcDsc, ok := c.svcDsc.Load().(*dthrift.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}
	data := msg.Data()

	// TODO: discuss encode with hyper codec
	// TODO: discuss encode with FastWrite

	// encode with normal way using dynamicgo
	tyDsc := svcDsc.Functions()[method].Request().Struct().Fields()[0].Type()
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
	cv := j2t.NewBinaryConv(c.convOpts)
	buf := make([]byte, 0, len(transBuff))

	tProt := cthrift.NewBinaryProtocol(out)
	if err := tProt.WriteMessageBegin(method, athrift.TMessageType(msgType), msg.RPCInfo().Invocation().SeqID()); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteMessageBegin failed: %s", err.Error()))
	}
	// TODO: discuss *(*[]byte)(unsafe.Pointer(&transBuff))
	// get thrift binary
	err := cv.DoInto(ctx, tyDsc, *(*[]byte)(unsafe.Pointer(&transBuff)), &buf)
	if err != nil {
		return err
	}

	// write thrift []byte
	_, err = out.WriteBinary(buf)
	//_, err = out.Write(buf)
	if err != nil {
		return err
	}
	if err := tProt.WriteMessageEnd(); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteMessageEnd failed: %s", err.Error()))
	}
	tProt.Recycle()
	return nil
}

func (c *jsonThriftDynamicgoCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	svcDsc, ok := c.svcDsc.Load().(*dthrift.ServiceDescriptor)
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
	// exception message
	if msg.MessageType() == remote.Exception {
		exception := athrift.NewTApplicationException(athrift.UNKNOWN_APPLICATION_EXCEPTION, "")
		if err := exception.Read(tProt); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal Exception failed: %s", err.Error()))
		}
		if err := tProt.ReadMessageEnd(); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal, ReadMessageEnd failed: %s", err.Error()))
		}
		return remote.NewTransError(exception.TypeId(), exception)
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

	// TODO: discuss decode with hyper unmarshal
	// TODO: discuss decode with FastRead

	transBuff, err := in.ReadBinary(in.ReadableLen())
	if err != nil {
		return err
	}
	tyDsc := svcDsc.Functions()[method].Request().Struct().Fields()[0].Type()
	cv := t2j.NewBinaryConv(c.convOpts)
	// thrift []byte -> json []byte
	buf := bytesPool.Get().(*[]byte)
	// get json binary
	if err := cv.DoInto(ctx, tyDsc, transBuff, buf); err != nil {
		return err
	}
	tProt.ReadMessageEnd()
	tProt.Recycle()

	data := msg.Data()
	if msg.RPCRole() == remote.Server {
		gArg := data.(*Args)
		gArg.Method = method
		gArg.Request = string(*buf)
	} else {
		gResult := data.(*Result)
		// TODO: consider what to respond
		gResult.Success = string(*buf)
	}
	*buf = (*buf)[:0]
	bytesPool.Put(buf)
	return nil
}

func (c *jsonThriftDynamicgoCodec) Name() string {
	return "JSONDynamicgoThrift"
}

func (c *jsonThriftDynamicgoCodec) Close() error {
	return c.provider.Close()
}
