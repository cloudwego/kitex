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
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"sync/atomic"

	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/j2t"
	"github.com/cloudwego/dynamicgo/conv/t2j"
	dhttp "github.com/cloudwego/dynamicgo/http"
	dthrift "github.com/cloudwego/dynamicgo/thrift"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	jsoniter "github.com/json-iterator/go"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	_ remote.PayloadCodec = &httpThriftCodec{}
	_ Closer              = &httpThriftCodec{}
)

// HTTPRequest alias of descriptor HTTPRequest
type HTTPRequest = descriptor.HTTPRequest

// HTTPResponse alias of descriptor HTTPResponse
type HTTPResponse = descriptor.HTTPResponse

type httpThriftCodec struct {
	svcDsc           atomic.Value // *idl
	provider         DescriptorProvider
	codec            remote.PayloadCodec
	binaryWithBase64 bool
}

func newHTTPThriftCodec(p DescriptorProvider, codec remote.PayloadCodec) (*httpThriftCodec, error) {
	svc := <-p.Provide()
	c := &httpThriftCodec{codec: codec, provider: p, binaryWithBase64: false}
	c.svcDsc.Store(svc)
	go c.update()
	return c, nil
}

func (c *httpThriftCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcDsc.Store(svc)
	}
}

func (c *httpThriftCodec) getMethod(req interface{}) (*Method, error) {
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return nil, errors.New("get method name failed, no ServiceDescriptor")
	}
	r, ok := req.(*HTTPRequest)
	if !ok {
		return nil, errors.New("req is invalid, need descriptor.HTTPRequest")
	}
	function, err := svcDsc.Router.Lookup(r)
	if err != nil {
		return nil, err
	}
	fnDsc := function.(*descriptor.FunctionDescriptor)
	return &Method{fnDsc.Name, fnDsc.Oneway}, nil
}

func (c *httpThriftCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {

	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("get parser ServiceDescriptor failed")
	}

	inner := thrift.NewWriteHTTPRequest(svcDsc)
	inner.SetBinaryWithBase64(c.binaryWithBase64)
	msg.Data().(WithCodec).SetCodec(inner)
	return c.codec.Marshal(ctx, msg, out)
}

func (c *httpThriftCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("get parser ServiceDescriptor failed")
	}
	inner := thrift.NewReadHTTPResponse(svcDsc)
	inner.SetBase64Binary(c.binaryWithBase64)
	msg.Data().(WithCodec).SetCodec(inner)
	return c.codec.Unmarshal(ctx, msg, in)
}

func (c *httpThriftCodec) Name() string {
	return "HttpThrift"
}

func (c *httpThriftCodec) Close() error {
	return c.provider.Close()
}

var json = jsoniter.Config{
	EscapeHTML: true,
	UseNumber:  true,
}.Froze()

// FromHTTPRequest parse  HTTPRequest from http.Request
func FromHTTPRequest(req *http.Request) (customReq *HTTPRequest, err error) {
	if enableDynamicgo {
		req.Header.Set(dhttp.HeaderContentType, descriptor.MIMEApplicationJson)
		dreq, err := dhttp.NewHTTPRequestFromStdReq(req)
		if err != nil {
			return nil, err
		}
		customReq = &HTTPRequest{
			Method:       req.Method,
			Path:         req.URL.Path,
			DHTTPRequest: dreq,
		}
	} else {
		customReq = &HTTPRequest{
			Header:      req.Header,
			Query:       req.URL.Query(),
			Cookies:     descriptor.Cookies{},
			Method:      req.Method,
			Host:        req.Host,
			Path:        req.URL.Path,
			ContentType: descriptor.MIMEApplicationJson,
			Body:        map[string]interface{}{},
		}
		for _, cookie := range req.Cookies() {
			customReq.Cookies[cookie.Name] = cookie.Value
		}
		// copy the request body, maybe user used it after parse
		var b io.ReadCloser
		if req.GetBody != nil {
			// req from ServerHTTP or create by http.NewRequest
			if b, err = req.GetBody(); err != nil {
				return nil, err
			}
		} else {
			b = req.Body
		}
		if b == nil {
			// body == nil if from Get request
			return customReq, nil
		}
		if customReq.RawBody, err = ioutil.ReadAll(b); err != nil {
			return nil, err
		}
		if len(customReq.RawBody) == 0 {
			return customReq, nil
		}
		err = json.Unmarshal(customReq.RawBody, &customReq.Body)
	}
	return customReq, err
}

type httpThriftDynamicgoCodec struct {
	svcDsc   atomic.Value // *idl
	provider DescriptorProvider
	codec    remote.PayloadCodec
	convOpts conv.Options
	j2t      j2t.BinaryConv
	t2j      t2j.BinaryConv
	router   descriptor.Router
}

func newHttpThriftDynamicgoCodec(p DescriptorProvider, codec remote.PayloadCodec, convOpts conv.Options) (*httpThriftDynamicgoCodec, error) {
	svc := <-p.Provide()
	c := &httpThriftDynamicgoCodec{codec: codec, provider: p, convOpts: convOpts}
	c.svcDsc.Store(svc)
	c.j2t = j2t.NewBinaryConv(convOpts)
	c.t2j = t2j.NewBinaryConv(convOpts)
	c.router = descriptor.NewRouter()
	// initialize router
	for _, fnDsc := range svc.DynamicgoDesc.Functions() {
		for _, ep := range fnDsc.Endpoints() {
			c.router.Handle(descriptor.NewDynamicgoRoute(ep.Method, ep.Path, fnDsc))
		}
	}
	go c.update()
	return c, nil
}

func (c *httpThriftDynamicgoCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcDsc.Store(svc)
	}
}

// This method makes users to specify the method name even though the method name should be included in http request...
func (c *httpThriftDynamicgoCodec) getMethod(req interface{}) (*Method, error) {
	r, ok := req.(*HTTPRequest)
	if !ok {
		return nil, errors.New("req is invalid, need descriptor.HTTPRequest")
	}
	function, err := c.router.Lookup(r)
	if err != nil {
		return nil, err
	}
	fnDsc := function.(*dthrift.FunctionDescriptor)
	return &Method{fnDsc.Name(), fnDsc.Oneway()}, nil
}

func (c *httpThriftDynamicgoCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	method := msg.RPCInfo().Invocation().MethodName()
	if method == "" {
		return perrors.NewProtocolErrorWithMsg("empty methodName in thrift Marshal")
	}

	msgType := msg.MessageType()
	data := msg.Data()
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
	}

	// TODO: discuss encode with hyper codec
	// TODO: discuss encode with FastWrite

	gArg := data.(*Args)
	transData := gArg.Request
	transBuff, ok := transData.(*HTTPRequest)
	//transBuff, ok := transData.(*dhttp.HTTPRequest)
	if !ok {
		return perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("decode msg failed, type(%v) is not *HTTPRequest", reflect.TypeOf(transData)))
		//return perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("decode msg failed, type(%v) is not *dhttp.HTTPRequest", reflect.TypeOf(transData)))
	}

	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}
	if svcDsc.DynamicgoDesc == nil {
		return perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDesc is nil")
	}
	tyDsc := svcDsc.DynamicgoDesc.Functions()[method].Request().Struct().Fields()[0].Type()

	// encode in normal way using dynamicgo
	seqID := msg.RPCInfo().Invocation().SeqID()
	tProt := cthrift.NewBinaryProtocol(out)
	if err := tProt.WriteMessageBegin(method, athrift.TMessageType(msgType), seqID); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteMessageBegin failed: %s", err.Error()))
	}
	if err := tProt.WriteStructBegin(""); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteStructBegin failed: %s", err.Error()))
	}
	if err := tProt.WriteFieldBegin("", athrift.STRUCT, 1); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteFieldBegin failed: %s", err.Error()))
	}

	//body := transBuff.RawBody
	body := transBuff.DHTTPRequest.Body()
	//body := transBuff.Body()
	buf := make([]byte, 0, len(body))
	ctx = context.WithValue(ctx, conv.CtxKeyHTTPRequest, transBuff.DHTTPRequest)
	// json to thrift
	if err := c.j2t.DoInto(ctx, tyDsc, body, &buf); err != nil {
		return err
	}

	if _, err := out.WriteBinary(buf); err != nil {
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

func (c *httpThriftDynamicgoCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}

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

	// decode in normal way
	if msg.PayloadLen() == 0 {
		return errors.New("msg.PayloadLen must not be 0. Please use TTHeader protocol")
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
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}
	if svcDsc.DynamicgoDesc == nil {
		return perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDesc is nil")
	}
	tyDsc := svcDsc.DynamicgoDesc.Functions()[method].Response().Struct().Fields()[0].Type()

	buf := bytesPool.Get().(*[]byte)
	if err != nil {
		panic(err)
	}
	ctx = context.WithValue(ctx, conv.CtxKeyHTTPResponse, dhttp.NewHTTPResponse())
	if err = c.t2j.DoInto(ctx, tyDsc, transBuff, buf); err != nil {
		return err
	}

	tProt.ReadFieldEnd()
	in.ReadBinary(bthrift.Binary.FieldStopLength())
	tProt.ReadStructEnd()
	tProt.ReadMessageEnd()
	tProt.Recycle()

	gResult := msg.Data().(*Result)
	// TODO: consider what to respond
	resp := descriptor.NewHTTPResponse()
	// TODO: make resp.Body json string in the future?
	resp.GeneralBody = string(*buf)
	gResult.Success = resp

	*buf = (*buf)[:0]
	bytesPool.Put(buf)
	return nil
}

func (c *httpThriftDynamicgoCodec) Name() string {
	return "HttpThriftDynamicgo"
}

func (c *httpThriftDynamicgoCodec) Close() error {
	return c.provider.Close()
}
