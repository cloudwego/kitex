/*
 * Copyright 2023 CloudWeGo Authors
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

package proto

import (
	"context"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/cloudwego/dynamicgo/conv"
	dconvj2p "github.com/cloudwego/dynamicgo/conv/j2p"
	dconvp2j "github.com/cloudwego/dynamicgo/conv/p2j"
	dproto "github.com/cloudwego/dynamicgo/proto"

	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

// NewWriteJSON build WriteJSON according to ServiceDescriptor
func NewWriteJSON(svc *dproto.ServiceDescriptor, method string, isClient bool, convOpts *conv.Options) (*WriteJSON, error) {
	fnDsc := svc.LookupMethodByName(method)
	if fnDsc == nil {
		return nil, fmt.Errorf("missing method: %s in service: %s", method, svc.Name())
	}

	// from the proto.ServiceDescriptor, get the TypeDescriptor
	typeDescriptor := fnDsc.Input()
	if !isClient {
		typeDescriptor = fnDsc.Output()
	}

	ws := &WriteJSON{
		dynamicgoConvOpts: convOpts,
		dynamicgoTypeDsc:  typeDescriptor,
		isClient:          isClient,
	}
	return ws, nil
}

// WriteJSON implement of MessageWriter
type WriteJSON struct {
	dynamicgoConvOpts *conv.Options
	dynamicgoTypeDsc  *dproto.TypeDescriptor
	isClient          bool
}

var _ MessageWriter = (*WriteJSON)(nil)

// Write converts msg to protobuf wire format and returns an output bytebuffer
func (m *WriteJSON) Write(ctx context.Context, msg interface{}) (interface{}, error) {
	// msg is string
	s, ok := msg.(string)
	if !ok {
		return nil, perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is not string")
	}

	cv := dconvj2p.NewBinaryConv(*m.dynamicgoConvOpts)

	// get protobuf-encode bytes
	actualMsgBuf, err := cv.Do(ctx, m.dynamicgoTypeDsc, stringToByteSliceUnsafe(s))
	if err != nil {
		return nil, perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("protobuf marshal message failed: %s", err.Error()))
	}
	return actualMsgBuf, nil
}

// NewReadJSON build ReadJSON according to ServiceDescriptor
func NewReadJSON(svc *dproto.ServiceDescriptor, isClient bool, convOpts *conv.Options) (*ReadJSON, error) {
	// extract svc to be used to convert later
	return &ReadJSON{
		dynamicgoConvOpts: convOpts,
		dynamicgoSvcDsc:   svc,
		isClient:          isClient,
	}, nil
}

// ReadJSON implement of MessageReaderWithMethod
type ReadJSON struct {
	dynamicgoConvOpts *conv.Options
	dynamicgoSvcDsc   *dproto.ServiceDescriptor
	isClient          bool
}

var _ MessageReader = (*ReadJSON)(nil)

//// SetBinaryWithBase64 enable/disable Base64 encoding for binary.
//// Note that this method is not concurrent-safe.
//func (m *ReadJSON) SetBinaryWithBase64(enable bool) {
//	m.binaryWithBase64 = enable
//}

// Read read data from actualMsgBuf and convert to json string
func (m *ReadJSON) Read(ctx context.Context, method string, actualMsgBuf []byte) (interface{}, error) {
	// create dynamic message here, once method string has been extracted
	fnDsc := m.dynamicgoSvcDsc.LookupMethodByName(method)
	if fnDsc == nil {
		return nil, fmt.Errorf("missing method: %s in service: %s", method, m.dynamicgoSvcDsc.Name())
	}

	// from the proto.ServiceDescriptor, get the TypeDescriptor
	typeDescriptor := fnDsc.Input()
	if !m.isClient {
		typeDescriptor = fnDsc.Output()
	}

	cv := dconvp2j.NewBinaryConv(*m.dynamicgoConvOpts)
	out, err := cv.Do(context.Background(), typeDescriptor, actualMsgBuf)
	if err != nil {
		return nil, err
	}

	return string(out), nil
}

func stringToByteSliceUnsafe(s string) []byte {
	stringHeader := (*reflect.StringHeader)(unsafe.Pointer(&s))
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: stringHeader.Data,
		Len:  stringHeader.Len,
		Cap:  stringHeader.Len,
	}))
}
