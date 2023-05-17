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
	"strings"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/t2j"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
)

// WriteHTTPRequest implement of MessageWriter
type WriteHTTPRequest struct {
	svc    *descriptor.ServiceDescriptor
	dyOpts *conv.Options
	method string
}

var _ MessageWriter = (*WriteHTTPRequest)(nil)

// NewWriteHTTPRequest ...
// Base64 decoding for binary is enabled by default.
func NewWriteHTTPRequest(svc *descriptor.ServiceDescriptor) *WriteHTTPRequest {
	return &WriteHTTPRequest{svc, &conv.Options{}, ""}
}

// ReadHTTPResponse implement of MessageReaderWithMethod
type ReadHTTPResponse struct {
	svc             *descriptor.ServiceDescriptor
	enableDynamicgo bool
	dyOpts          *conv.Options
	msg             remote.Message
}

var _ MessageReader = (*ReadHTTPResponse)(nil)

// NewReadHTTPResponse ...
// Base64 encoding for binary is enabled by default.
func NewReadHTTPResponse(svc *descriptor.ServiceDescriptor) *ReadHTTPResponse {
	return &ReadHTTPResponse{svc, false, &conv.Options{}, nil}
}

// SetReadHTTPResponse ...
func (r *ReadHTTPResponse) SetReadHTTPResponse(enable bool, opts *conv.Options, msg remote.Message) error {
	r.enableDynamicgo = enable
	r.dyOpts = opts
	r.msg = msg
	if r.enableDynamicgo && r.msg.PayloadLen() != 0 && r.svc.DynamicgoDsc == nil {
		return perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDsc is nil")
	}
	return nil
}

// Read ...
func (r *ReadHTTPResponse) Read(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	// Transport protocol should be TTHeader, Framed, or TTHeaderFramed to enable dynamicgo
	// fallback logic
	if !r.enableDynamicgo || r.msg.PayloadLen() == 0 {
		fnDsc, err := r.svc.LookupFunctionByMethod(method)
		if err != nil {
			return nil, err
		}
		fDsc := fnDsc.Response
		return skipStructReader(ctx, in, fDsc, &readerOption{forJSON: true, http: true, binaryWithBase64: !r.dyOpts.NoBase64Binary})
	}

	// dynamicgo logic
	tProt, ok := in.(*cthrift.BinaryProtocol)
	if !ok {
		return nil, perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
	}

	tyDsc := r.svc.DynamicgoDsc.Functions()[method].Response()

	msgBeginLen := bthrift.Binary.MessageBeginLength(method, thrift.TMessageType(r.msg.MessageType()), r.msg.RPCInfo().Invocation().SeqID())
	transBuf, err := tProt.ByteBuffer().ReadBinary(r.msg.PayloadLen() - msgBeginLen - bthrift.Binary.MessageEndLength())
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 0, len(transBuf)*2)
	resp := descriptor.NewHTTPResponse()
	ctx = context.WithValue(ctx, conv.CtxKeyHTTPResponse, resp)
	cv := t2j.NewBinaryConv(*r.dyOpts)
	// decode with dynamicgo
	// thrift []byte to json []byte
	if err = cv.DoInto(ctx, tyDsc, transBuf, &buf); err != nil {
		return nil, err
	}

	// unwrap one level of json string to make resp compatible with the resp in fallback way
	if len(buf) > structWrapLen {
		secondBracket := strings.Index(string(buf)[1:], "{") + 1
		if secondBracket != 0 {
			buf = buf[secondBracket : len(buf)-1]
		}
	}
	resp.RawBody = buf
	return resp, nil
}
