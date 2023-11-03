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

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
)

// NewWriteStruct ...
func NewWriteStruct(svc *descriptor.ServiceDescriptor, method string, isClient bool) (*WriteStruct, error) {
	fnDsc, err := svc.LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	ty := fnDsc.Request
	if !isClient {
		ty = fnDsc.Response
	}
	ws := &WriteStruct{
		ty:             ty,
		hasRequestBase: fnDsc.HasRequestBase && isClient,
	}
	return ws, nil
}

// WriteStruct implement of MessageWriter
type WriteStruct struct {
	ty               *descriptor.TypeDescriptor
	hasRequestBase   bool
	binaryWithBase64 bool
}

var _ MessageWriter = (*WriteStruct)(nil)

// SetBinaryWithBase64 enable/disable Base64 decoding for binary.
// Note that this method is not concurrent-safe.
func (m *WriteStruct) SetBinaryWithBase64(enable bool) {
	m.binaryWithBase64 = enable
}

// Write ...
func (m *WriteStruct) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	if !m.hasRequestBase {
		requestBase = nil
	}
	return wrapStructWriter(ctx, msg, out, m.ty, &writerOption{requestBase: requestBase, binaryWithBase64: m.binaryWithBase64})
}

// NewReadStruct ...
func NewReadStruct(svc *descriptor.ServiceDescriptor, isClient bool) *ReadStruct {
	return &ReadStruct{
		svc:      svc,
		isClient: isClient,
	}
}

func NewReadStructForJSON(svc *descriptor.ServiceDescriptor, isClient bool) *ReadStruct {
	return &ReadStruct{
		svc:      svc,
		isClient: isClient,
		forJSON:  true,
	}
}

// ReadStruct implement of MessageReaderWithMethod
type ReadStruct struct {
	svc                 *descriptor.ServiceDescriptor
	isClient            bool
	forJSON             bool
	binaryWithBase64    bool
	binaryWithByteSlice bool
}

var _ MessageReader = (*ReadStruct)(nil)

// SetBinaryOption enable/disable Base64 encoding or returning []byte for binary.
// Note that this method is not concurrent-safe.
func (m *ReadStruct) SetBinaryOption(base64 bool, byteSlice bool) {
	m.binaryWithBase64 = base64
	m.binaryWithByteSlice = byteSlice
}

// Read ...
func (m *ReadStruct) Read(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	fnDsc, err := m.svc.LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	fDsc := fnDsc.Response
	if !m.isClient {
		fDsc = fnDsc.Request
	}
	return skipStructReader(ctx, in, fDsc, &readerOption{throwException: true, forJSON: m.forJSON, binaryWithBase64: m.binaryWithBase64, binaryWithByteSlice: m.binaryWithByteSlice})
}
