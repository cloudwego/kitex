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
	"fmt"

	"github.com/bytedance/gopkg/lang/dirtmake"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/t2j"
	dthrift "github.com/cloudwego/dynamicgo/thrift"
	jsoniter "github.com/json-iterator/go"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	thrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
)

// WriteHTTPRequest implement of MessageWriter
type WriteHTTPRequest struct {
	svc                    *descriptor.ServiceDescriptor
	dynamicgoTypeDsc       *dthrift.TypeDescriptor
	binaryWithBase64       bool
	convOpts               conv.Options // used for dynamicgo conversion
	convOptsWithThriftBase conv.Options // used for dynamicgo conversion with EnableThriftBase turned on
	hasRequestBase         bool
	dynamicgoEnabled       bool
}

var (
	_          MessageWriter = (*WriteHTTPRequest)(nil)
	customJson               = jsoniter.Config{
		EscapeHTML: true,
		UseNumber:  true,
	}.Froze()
)

// NewWriteHTTPRequest ...
// Base64 decoding for binary is enabled by default.
func NewWriteHTTPRequest(svc *descriptor.ServiceDescriptor) *WriteHTTPRequest {
	return &WriteHTTPRequest{svc: svc, binaryWithBase64: true, dynamicgoEnabled: false}
}

// SetBinaryWithBase64 enable/disable Base64 decoding for binary.
// Note that this method is not concurrent-safe.
func (w *WriteHTTPRequest) SetBinaryWithBase64(enable bool) {
	w.binaryWithBase64 = enable
}

// SetDynamicGo ...
func (w *WriteHTTPRequest) SetDynamicGo(convOpts, convOptsWithThriftBase *conv.Options, method string) error {
	w.convOpts = *convOpts
	w.convOptsWithThriftBase = *convOptsWithThriftBase
	w.dynamicgoEnabled = true
	fnDsc := w.svc.DynamicGoDsc.Functions()[method]
	if fnDsc == nil {
		return fmt.Errorf("missing method: %s in service: %s in dynamicgo", method, w.svc.DynamicGoDsc.Name())
	}
	w.hasRequestBase = fnDsc.HasRequestBase()
	w.dynamicgoTypeDsc = fnDsc.Request()
	return nil
}

// originalWrite ...
func (w *WriteHTTPRequest) originalWrite(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	req := msg.(*descriptor.HTTPRequest)
	if req.Body == nil && len(req.RawBody) != 0 {
		if err := customJson.Unmarshal(req.RawBody, &req.Body); err != nil {
			return err
		}
	}
	fn, err := w.svc.Router.Lookup(req)
	if err != nil {
		return err
	}
	if !fn.HasRequestBase {
		requestBase = nil
	}
	return wrapStructWriter(ctx, req, out, fn.Request, &writerOption{requestBase: requestBase, binaryWithBase64: w.binaryWithBase64})
}

// ReadHTTPResponse implement of MessageReaderWithMethod
type ReadHTTPResponse struct {
	svc                   *descriptor.ServiceDescriptor
	base64Binary          bool
	msg                   remote.Message
	dynamicgoEnabled      bool
	useRawBodyForHTTPResp bool
	t2jBinaryConv         t2j.BinaryConv // used for dynamicgo thrift to json conversion
}

var _ MessageReader = (*ReadHTTPResponse)(nil)

// NewReadHTTPResponse ...
// Base64 encoding for binary is enabled by default.
func NewReadHTTPResponse(svc *descriptor.ServiceDescriptor) *ReadHTTPResponse {
	return &ReadHTTPResponse{svc: svc, base64Binary: true, dynamicgoEnabled: false, useRawBodyForHTTPResp: false}
}

// SetBase64Binary enable/disable Base64 encoding for binary.
// Note that this method is not concurrent-safe.
func (r *ReadHTTPResponse) SetBase64Binary(enable bool) {
	r.base64Binary = enable
}

// SetUseRawBodyForHTTPResp ...
func (r *ReadHTTPResponse) SetUseRawBodyForHTTPResp(useRawBodyForHTTPResp bool) {
	r.useRawBodyForHTTPResp = useRawBodyForHTTPResp
}

// SetDynamicGo ...
func (r *ReadHTTPResponse) SetDynamicGo(convOpts *conv.Options, msg remote.Message) {
	r.t2jBinaryConv = t2j.NewBinaryConv(*convOpts)
	r.msg = msg
	r.dynamicgoEnabled = true
}

// Read ...
func (r *ReadHTTPResponse) Read(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	// fallback logic
	if !r.dynamicgoEnabled {
		return r.originalRead(ctx, method, in)
	}

	tProt, ok := in.(*cthrift.BinaryProtocol)
	if !ok {
		return nil, perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
	}
	mBeginLen := bthrift.Binary.MessageBeginLength(method, thrift.TMessageType(r.msg.MessageType()), r.msg.RPCInfo().Invocation().SeqID())
	sName, err := in.ReadStructBegin()
	if err != nil {
		return nil, err
	}
	sBeginLen := bthrift.Binary.StructBeginLength(sName)
	// TODO: support exception field
	fName, typeId, id, err := in.ReadFieldBegin()
	if err != nil {
		return nil, err
	}
	fBeginLen := bthrift.Binary.FieldBeginLength(fName, typeId, id)
	transBuf, err := tProt.ByteBuffer().ReadBinary(r.msg.PayloadLen() - mBeginLen - sBeginLen - fBeginLen - bthrift.Binary.MessageEndLength())
	if err != nil {
		return nil, err
	}
	fid := dthrift.FieldID(id)

	resp := descriptor.NewHTTPResponse()
	ctx = context.WithValue(ctx, conv.CtxKeyHTTPResponse, resp)
	fnDsc := r.svc.DynamicGoDsc.Functions()[method]
	if fnDsc == nil {
		return nil, fmt.Errorf("missing method: %s in service: %s in dynamicgo", method, r.svc.DynamicGoDsc.Name())
	}
	tyDsc := fnDsc.Response()
	// json size is usually 2 times larger than equivalent thrift data
	buf := dirtmake.Bytes(0, len(transBuf)*2)

	for _, field := range tyDsc.Struct().Fields() {
		if fid == field.ID() {
			// decode with dynamicgo
			// thrift []byte to json []byte
			if err = r.t2jBinaryConv.DoInto(ctx, field.Type(), transBuf, &buf); err != nil {
				return nil, err
			}
			break
		}
	}
	resp.RawBody = buf
	return resp, nil
}

func (r *ReadHTTPResponse) originalRead(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	fnDsc, err := r.svc.LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	fDsc := fnDsc.Response
	resp, err := skipStructReader(ctx, in, fDsc, &readerOption{forJSON: true, http: true, binaryWithBase64: r.base64Binary})
	if r.useRawBodyForHTTPResp {
		if httpResp, ok := resp.(*descriptor.HTTPResponse); ok && httpResp.Body != nil {
			rawBody, err := customJson.Marshal(httpResp.Body)
			if err != nil {
				return nil, err
			}
			httpResp.RawBody = rawBody
		}
	}
	return resp, err
}
