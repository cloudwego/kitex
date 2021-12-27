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

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	jsoniter "github.com/json-iterator/go"
	"github.com/tidwall/gjson"
)

// NewWriteJSON build WriteJSON according to ServiceDescriptor
func NewWriteJSON(svc *descriptor.ServiceDescriptor, method string, isClient bool) (*WriteJSON, error) {
	fnDsc, err := svc.LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	ty := fnDsc.Request
	if !isClient {
		ty = fnDsc.Response
	}
	ws := &WriteJSON{
		ty:             ty,
		hasRequestBase: fnDsc.HasRequestBase && isClient,
		base64Binary:   true,
	}
	return ws, nil
}

// WriteJSON implement of MessageWriter
type WriteJSON struct {
	ty             *descriptor.TypeDescriptor
	hasRequestBase bool
	base64Binary   bool
}

var _ MessageWriter = (*WriteJSON)(nil)

// SetBase64Binary enable/disable Base64 decoding for binary.
// Note that this method is not concurrent-safe.
func (m *WriteJSON) SetBase64Binary(enable bool) {
	m.base64Binary = enable
}

// Write write json string to out thrift.TProtocol
func (m *WriteJSON) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	if !m.hasRequestBase {
		requestBase = nil
	}

	// msg is void
	if _, ok := msg.(descriptor.Void); ok {
		return wrapStructWriter(ctx, msg, out, m.ty, &writerOption{requestBase: requestBase, binaryWithBase64: m.base64Binary})
	}

	// msg is string
	s, ok := msg.(string)
	if !ok {
		return perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is not string")
	}

	body := gjson.Parse(s)
	if body.Type == gjson.Null {
		body = gjson.Result{
			Type:  gjson.String,
			Raw:   s,
			Str:   s,
			Num:   0,
			Index: 0,
		}
	}
	return wrapJSONWriter(ctx, &body, out, m.ty, &writerOption{requestBase: requestBase, binaryWithBase64: m.base64Binary})
}

// NewReadJSON build ReadJSON according to ServiceDescriptor
func NewReadJSON(svc *descriptor.ServiceDescriptor, isClient bool) *ReadJSON {
	return &ReadJSON{
		svc:              svc,
		isClient:         isClient,
		binaryWithBase64: true,
	}
}

// ReadJSON implement of MessageReaderWithMethod
type ReadJSON struct {
	svc              *descriptor.ServiceDescriptor
	isClient         bool
	binaryWithBase64 bool
}

var _ MessageReader = (*ReadJSON)(nil)

// SetBinaryWithBase64 enable/disable Base64 encoding for binary.
// Note that this method is not concurrent-safe.
func (m *ReadJSON) SetBinaryWithBase64(enable bool) {
	m.binaryWithBase64 = enable
}

// Read read data from in thrift.TProtocol and convert to json string
func (m *ReadJSON) Read(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	fnDsc, err := m.svc.LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	fDsc := fnDsc.Response
	if !m.isClient {
		fDsc = fnDsc.Request
	}
	resp, err := skipStructReader(ctx, in, fDsc, &readerOption{forJSON: true, throwException: true, binaryWithBase64: m.binaryWithBase64})
	if err != nil {
		return nil, err
	}

	// resp is void
	if _, ok := resp.(descriptor.Void); ok {
		return resp, nil
	}

	// resp is string
	if _, ok := resp.(string); ok {
		return resp, nil
	}

	// resp is map
	respNode, err := jsoniter.Marshal(resp)
	if err != nil {
		return nil, perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("response marshal failed. err:%#v", err))
	}

	return string(respNode), nil
}
