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
	"errors"
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

func newJsonThriftCodec(p DescriptorProvider, codec remote.PayloadCodec, convOpts ...conv.Options) (*jsonThriftCodec, error) {
	svc := <-p.Provide()
	var c *jsonThriftCodec
	if convOpts == nil {
		c = &jsonThriftCodec{codec: codec, provider: p, binaryWithBase64: true}
	} else {
		// codec with dynamicgo
		c = &jsonThriftCodec{codec: codec, provider: p, binaryWithBase64: true, convOpts: convOpts[0], enableDynamicgo: true}
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

func (c *jsonThriftCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	method := msg.RPCInfo().Invocation().MethodName()
	if method == "" {
		return perrors.NewProtocolErrorWithMsg("empty methodName in thrift Marshal")
	}

	msgType := msg.MessageType()

	if !c.enableDynamicgo {
		if msgType == remote.Exception {
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

	data := msg.Data()
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}
	var tyDsc *dthrift.TypeDescriptor
	if msgType == remote.Exception {
		transErr, isTransErr := data.(*remote.TransError)
		if !isTransErr {
			if err, isError := data.(error); isError {
				data = athrift.NewTApplicationException(remote.InternalError, err.Error())
			}
		} else {
			return errors.New("exception relay need error type data")
		}
		data = athrift.NewTApplicationException(transErr.TypeID(), transErr.Error())
	} else {
		if svcDsc.DynamicgoDesc == nil {
			return perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDesc is nil")
		}
		if msgType == remote.Reply {
			tyDsc = svcDsc.DynamicgoDesc.Functions()[method].Response().Struct().Fields()[0].Type()
		} else { // TODO: think about remote.OneWay, remote.Stream, remote.InvalidProtocol
			tyDsc = svcDsc.DynamicgoDesc.Functions()[method].Request().Struct().Fields()[0].Type()
		}
	}

	// TODO: discuss encode with hyper codec
	// TODO: discuss encode with FastWrite

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
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteMessageBegin failed: %s", err.Error()))
	}
	if err := tProt.WriteStructBegin(""); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteStructBegin failed: %s", err.Error()))
	}
	if msgType == remote.Reply {
		if err := tProt.WriteFieldBegin("", athrift.STRUCT, 0); err != nil {
			return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteFieldBegin failed: %s", err.Error()))
		}
	} else {
		if err := tProt.WriteFieldBegin("", athrift.STRUCT, 1); err != nil {
			return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteFieldBegin failed: %s", err.Error()))
		}
	}
	// TODO: discuss *(*[]byte)(unsafe.Pointer(&transBuff))
	// encode json binary to thrift binary
	err := cv.DoInto(ctx, tyDsc, *(*[]byte)(unsafe.Pointer(&transBuff)), &buf)
	//err := cv.DoInto(ctx, tyDsc, []byte(transBuff), &buf)
	if err != nil {
		return err
	}

	// write thrift []byte
	if _, err = out.WriteBinary(buf); err != nil {
		return err
	}
	tProt.WriteFieldStop()
	if err := tProt.WriteFieldEnd(); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteFieldEnd failed: %s", err.Error()))
	}
	if err := tProt.WriteStructEnd(); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteStructEnd failed: %s", err.Error()))
	}
	if err := tProt.WriteMessageEnd(); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteMessageEnd failed: %s", err.Error()))
	}
	tProt.Recycle()
	return nil
}

func (c *jsonThriftCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}

	if !c.enableDynamicgo || msg.PayloadLen() == 0 {
		rm := thrift.NewReadJSON(svcDsc, msg.RPCRole() == remote.Client)
		rm.SetBinaryWithBase64(c.binaryWithBase64)
		msg.Data().(WithCodec).SetCodec(rm)
		return c.codec.Unmarshal(ctx, msg, in)
	}

	// decode with dynamicgo
	tProt := cthrift.NewBinaryProtocol(in)
	method, msgType, seqID, err := tProt.ReadMessageBegin()
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal, ReadMessageBegin failed: %s", err.Error()))
	}
	sname, err := tProt.ReadStructBegin()
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal, ReadStructBegin failed: %s", err.Error()))
	}
	fname, ftype, fid, err := tProt.ReadFieldBegin()
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err,
			fmt.Sprintf("thrift unmarshal, ReadFieldBegin failed: %s", err.Error()))
	}
	if err = codec.UpdateMsgType(uint32(msgType), msg); err != nil {
		return err
	}

	var tyDsc *dthrift.TypeDescriptor
	messageType := msg.MessageType()
	// exception message
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
		if svcDsc.DynamicgoDesc == nil {
			return perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDesc is nil")
		}
		if messageType == remote.Reply {
			tyDsc = svcDsc.DynamicgoDesc.Functions()[method].Response().Struct().Fields()[0].Type()
		} else { // TODO: think about remote.Stream, remote.InvalidProtocol
			tyDsc = svcDsc.DynamicgoDesc.Functions()[method].Request().Struct().Fields()[0].Type()
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

	// TODO: discuss decode with hyper unmarshal
	// TODO: discuss decode with FastRead

	// decode in normal way
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

	cv := t2j.NewBinaryConv(c.convOpts)
	buf := bytesPool.Get().(*[]byte)
	// decode thrift binary to json binary
	if err := cv.DoInto(ctx, tyDsc, transBuff, buf); err != nil {
		return err
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
