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

	"github.com/cloudwego/dynamicgo/conv"
	dconvj2p "github.com/cloudwego/dynamicgo/conv/j2p"
	dconvp2j "github.com/cloudwego/dynamicgo/conv/p2j"
	dproto "github.com/cloudwego/dynamicgo/proto"

	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/utils"
)

type JSONReaderWriter struct {
	*ReadJSON
	*WriteJSON
}

func NewJsonReaderWriter(svc *dproto.ServiceDescriptor, method string, isClient bool, convOpts *conv.Options) (*JSONReaderWriter, error) {
	reader, err := NewReadJSON(svc, isClient, convOpts)
	if err != nil {
		return nil, err
	}
	writer, err := NewWriteJSON(svc, method, isClient, convOpts)
	if err != nil {
		return nil, err
	}
	return &JSONReaderWriter{ReadJSON: reader, WriteJSON: writer}, nil
}

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
	var s string
	if msg == nil {
		s = "{}"
	} else {
		// msg is string
		var ok bool
		s, ok = msg.(string)
		if !ok {
			return nil, perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is not string")
		}
	}

	cv := dconvj2p.NewBinaryConv(*m.dynamicgoConvOpts)

	// get protobuf-encode bytes
	actualMsgBuf, err := cv.Do(ctx, m.dynamicgoTypeDsc, utils.StringToSliceByte(s))
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

// Read reads data from actualMsgBuf and convert to json string
func (m *ReadJSON) Read(ctx context.Context, method string, actualMsgBuf []byte) (interface{}, error) {
	// create dynamic message here, once method string has been extracted
	fnDsc := m.dynamicgoSvcDsc.LookupMethodByName(method)
	if fnDsc == nil {
		return nil, fmt.Errorf("missing method: %s in service: %s", method, m.dynamicgoSvcDsc.Name())
	}

	// from the dproto.ServiceDescriptor, get the TypeDescriptor
	typeDescriptor := fnDsc.Output()
	if !m.isClient {
		typeDescriptor = fnDsc.Input()
	}

	cv := dconvp2j.NewBinaryConv(*m.dynamicgoConvOpts)
	out, err := cv.Do(context.Background(), typeDescriptor, actualMsgBuf)
	if err != nil {
		return nil, err
	}

	return string(out), nil
}
