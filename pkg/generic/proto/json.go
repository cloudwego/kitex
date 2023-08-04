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
	"encoding/json"
	"fmt"

	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

// NewWriteJSON build WriteJSON according to ServiceDescriptor
func NewWriteJSON(svc *desc.ServiceDescriptor, method string, isClient bool) (*WriteJSON, error) {
	fnDsc := svc.FindMethodByName(method)
	if fnDsc == nil {
		return nil, fmt.Errorf("missing method: %s in service: %s", method, svc.GetFullyQualifiedName())
	}

	// create dynamic message, get InputType if client, get OutputType if server
	msgDescriptor := fnDsc.GetInputType()
	if !isClient {
		msgDescriptor = fnDsc.GetOutputType()
	}
	inputMessage := dynamic.NewMessage(msgDescriptor)

	ws := &WriteJSON{
		dynamicMessage: *inputMessage,
		isClient:       isClient,
	}
	return ws, nil
}

// WriteJSON implement of MessageWriter
type WriteJSON struct {
	dynamicMessage dynamic.Message
	isClient       bool
}

var _ MessageWriter = (*WriteJSON)(nil)

// Write writes to a dynamic message and returns an output bytebuffer
func (m *WriteJSON) Write(ctx context.Context, msg interface{}) (interface{}, error) {
	// msg is string
	s, ok := msg.(string)
	if !ok {
		return nil, perrors.NewProtocolErrorWithType(perrors.InvalidData, "decode msg failed, is not string")
	}

	// Convert string into json
	var jsonMap map[string]interface{}
	err := json.Unmarshal([]byte(s), &jsonMap)
	if err != nil {
		return nil, perrors.NewProtocolErrorWithMsg("Incorrect JSON format")
	}

	// map data into inputMessage
	for fieldName, fieldValue := range jsonMap {
		err = m.dynamicMessage.TrySetFieldByName(fieldName, fieldValue)
		if err != nil {
			return nil, perrors.NewProtocolErrorWithMsg("Incorrect JSON format")
		}
	}

	actualMsgBuf, err := m.dynamicMessage.Marshal()
	if err != nil {
		return nil, perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("protobuf marshal message failed: %s", err.Error()))
	}
	return actualMsgBuf, nil
}

// NewReadJSON build ReadJSON according to ServiceDescriptor
func NewReadJSON(svc *desc.ServiceDescriptor, isClient bool) (*ReadJSON, error) {
	// extract svc to be used to create dynamic message later
	return &ReadJSON{
		svc:      svc,
		isClient: isClient,
	}, nil
}

// ReadJSON implement of MessageReaderWithMethod
type ReadJSON struct {
	svc      *desc.ServiceDescriptor
	isClient bool
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
	fnDsc := m.svc.FindMethodByName(method)
	if fnDsc == nil {
		return nil, fmt.Errorf("missing method: %s in service: %s", method, m.svc.GetFullyQualifiedName())
	}

	// create dynamic message, get InputType if client, get OutputType if server
	msgDescriptor := fnDsc.GetInputType()
	if !m.isClient {
		msgDescriptor = fnDsc.GetOutputType()
	}

	// create dynamic message
	dynamicMessage := dynamic.NewMessage(msgDescriptor)

	err := dynamicMessage.Unmarshal(actualMsgBuf)
	if err != nil {
		return nil, err
	}

	buf, err := dynamicMessage.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return string(buf), nil
}
