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
	"github.com/bytedance/sonic"
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
	svc              *descriptor.ServiceDescriptor
	binaryWithBase64 bool
	enableDynamicgo  bool
	opts             conv.Options
	method           string
}

var (
	_          MessageWriter = (*WriteHTTPRequest)(nil)
	customJson               = sonic.Config{
		EscapeHTML: true,
		UseNumber:  true,
	}.Froze()
)

// NewWriteHTTPRequest ...
// Base64 decoding for binary is enabled by default.
func NewWriteHTTPRequest(svc *descriptor.ServiceDescriptor) *WriteHTTPRequest {
	return &WriteHTTPRequest{svc, true, false, conv.Options{}, ""}
}

// SetBinaryWithBase64 enable/disable Base64 decoding for binary.
// Note that this method is not concurrent-safe.
func (w *WriteHTTPRequest) SetBinaryWithBase64(enable bool) {
	w.binaryWithBase64 = enable
}

// SetEnableDynamicgo enable/disable dynamicgo encoding.
func (w *WriteHTTPRequest) SetEnableDynamicgo(enable bool, opts conv.Options, method string) {
	w.enableDynamicgo = enable
	w.opts = opts
	w.method = method
}

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
	svc             *descriptor.ServiceDescriptor
	base64Binary    bool
	enableDynamicgo bool
	opts            conv.Options
	msg             remote.Message
}

var _ MessageReader = (*ReadHTTPResponse)(nil)

// NewReadHTTPResponse ...
// Base64 encoding for binary is enabled by default.
func NewReadHTTPResponse(svc *descriptor.ServiceDescriptor) *ReadHTTPResponse {
	return &ReadHTTPResponse{svc, true, false, conv.Options{}, nil}
}

// SetBase64Binary enable/disable Base64 encoding for binary.
// Note that this method is not concurrent-safe.
func (r *ReadHTTPResponse) SetBase64Binary(enable bool) {
	r.base64Binary = enable
}

// SetEnableDynamicgo enable/disable dynamicgo decoding.
func (r *ReadHTTPResponse) SetEnableDynamicgo(enable bool, opts conv.Options, msg remote.Message) {
	r.enableDynamicgo = enable
	r.opts = opts
	r.msg = msg
}

// Read ...
func (r *ReadHTTPResponse) Read(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	if r.enableDynamicgo {
		tProt, ok := in.(*cthrift.BinaryProtocol)
		if !ok {
			return nil, perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
		}

		if r.svc.DynamicgoDsc == nil {
			return nil, perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDsc is nil")
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
		cv := t2j.NewBinaryConv(r.opts)
		// decode with dynamicgo
		// thrift []byte to json []byte
		if err = cv.DoInto(ctx, tyDsc, transBuf, &buf); err != nil {
			return nil, err
		}

		// unwrap one level of json string
		if len(buf) > structWrapLen {
			secondBracket := strings.Index(string(buf)[1:], "{") + 1
			if secondBracket != 0 {
				buf = buf[secondBracket : len(buf)-1]
			}
		}
		resp.RawBody = buf
		return resp, nil
	}
	fnDsc, err := r.svc.LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	fDsc := fnDsc.Response
	return skipStructReader(ctx, in, fDsc, &readerOption{forJSON: true, http: true, binaryWithBase64: r.base64Binary})
}
