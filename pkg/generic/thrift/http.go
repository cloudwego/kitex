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
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/t2j"
	dthrift "github.com/cloudwego/dynamicgo/thrift"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
)

// WriteHTTPRequest implement of MessageWriter
type WriteHTTPRequest struct {
	svc                             *descriptor.ServiceDescriptor
	dynamicgoTy                     *dthrift.TypeDescriptor
	binaryWithBase64                bool
	dynamicgoConvOpts               *conv.Options // conversion options for dynamicgo
	dynamicgoConvOptsWithThriftBase *conv.Options // conversion options for dynamicgo with EnableThriftBase turned on
	hasRequestBase                  bool
	dynamicgoEnabled                bool
}

var _ MessageWriter = (*WriteHTTPRequest)(nil)

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
func (w *WriteHTTPRequest) SetDynamicGo(convOpts, convOptsWithThriftBase *conv.Options, method string) {
	w.dynamicgoConvOpts = convOpts
	w.dynamicgoConvOptsWithThriftBase = convOptsWithThriftBase
	w.dynamicgoEnabled = true
	fn := w.svc.DynamicGoDsc.Functions()[method]
	w.hasRequestBase = fn.HasRequestBase()
	w.dynamicgoTy = fn.Request()
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
	cv                    t2j.BinaryConv // used for dynamicgo thrift to json conversion
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
func (r *ReadHTTPResponse) SetDynamicGo(cv t2j.BinaryConv, msg remote.Message) {
	r.cv = cv
	r.msg = msg
	r.dynamicgoEnabled = true
}

// Read ...
func (r *ReadHTTPResponse) Read(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	// fallback logic
	if !r.dynamicgoEnabled {
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

	// dynamicgo logic
	if r.msg.PayloadLen() == 0 {
		return nil, perrors.NewProtocolErrorWithMsg("msg.PayloadLen should always be greater than zero")
	}

	tProt, ok := in.(*cthrift.BinaryProtocol)
	if !ok {
		return nil, perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
	}

	tyDsc := r.svc.DynamicGoDsc.Functions()[method].Response()

	mBeginLen := bthrift.Binary.MessageBeginLength(method, thrift.TMessageType(r.msg.MessageType()), r.msg.RPCInfo().Invocation().SeqID())
	sName, err := in.ReadStructBegin()
	if err != nil {
		return nil, err
	}
	sBeginLen := bthrift.Binary.StructBeginLength(sName)
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

	buf := make([]byte, 0, len(transBuf)*2)
	resp := descriptor.NewHTTPResponse()
	ctx = context.WithValue(ctx, conv.CtxKeyHTTPResponse, resp)

	for _, field := range tyDsc.Struct().Fields() {
		if fid == field.ID() {
			// decode with dynamicgo
			// thrift []byte to json []byte
			if err = r.cv.DoInto(ctx, field.Type(), transBuf, &buf); err != nil {
				return nil, err
			}
			break
		}
	}
	resp.RawBody = buf
	return resp, nil
}
