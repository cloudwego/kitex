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

	"github.com/jackedelic/kitex/internal/pkg/generic/descriptor"
)

// WriteHTTPRequest implement of MessageWriter
type WriteHTTPRequest struct {
	svc *descriptor.ServiceDescriptor
}

var _ MessageWriter = (*WriteHTTPRequest)(nil)

// NewWriteHTTPRequest ...
func NewWriteHTTPRequest(svc *descriptor.ServiceDescriptor) *WriteHTTPRequest {
	return &WriteHTTPRequest{svc}
}

// Write ...
func (w *WriteHTTPRequest) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	req := msg.(*descriptor.HTTPRequest)
	fn, err := w.svc.Router.Lookup(req)
	if err != nil {
		return err
	}
	if !fn.HasRequestBase {
		requestBase = nil
	}
	return wrapStructWriter(ctx, req, out, fn.Request, &writerOption{requestBase: requestBase})
}

// ReadHTTPResponse implement of MessageReaderWithMethod
type ReadHTTPResponse struct {
	svc *descriptor.ServiceDescriptor
}

var _ MessageReader = (*ReadHTTPResponse)(nil)

// NewReadHTTPResponse ...
func NewReadHTTPResponse(svc *descriptor.ServiceDescriptor) *ReadHTTPResponse {
	return &ReadHTTPResponse{svc}
}

// Read ...
func (r *ReadHTTPResponse) Read(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	fnDsc, err := r.svc.LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	fDsc := fnDsc.Response
	return skipStructReader(ctx, in, fDsc, &readerOption{forJSON: true, http: true})
}
