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
	"strconv"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/t2j"
	dthrift "github.com/cloudwego/dynamicgo/thrift"
	jsoniter "github.com/json-iterator/go"
	"github.com/tidwall/gjson"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
)

const voidWholeLen = 5

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
	var dty *dthrift.TypeDescriptor
	if svc.DynamicgoDsc != nil {
		dty = svc.DynamicgoDsc.Functions()[method].Request().Struct().Fields()[0].Type()
		if !isClient {
			dty = svc.DynamicgoDsc.Functions()[method].Response().Struct().Fields()[0].Type()
		}
	}
	ws := &WriteJSON{
		ty:             ty,
		dty:            dty,
		hasRequestBase: fnDsc.HasRequestBase && isClient,
		base64Binary:   true,
		isClient:       isClient,
	}
	return ws, nil
}

// WriteJSON implement of MessageWriter
type WriteJSON struct {
	ty              *descriptor.TypeDescriptor
	dty             *dthrift.TypeDescriptor
	hasRequestBase  bool
	base64Binary    bool
	enableDynamicgo bool
	isClient        bool
	opts            conv.Options
}

var _ MessageWriter = (*WriteJSON)(nil)

// SetBase64Binary enable/disable Base64 decoding for binary.
// Note that this method is not concurrent-safe.
func (m *WriteJSON) SetBase64Binary(enable bool) {
	m.base64Binary = enable
}

// SetEnableDynamicgo enable/disable dynamicgo encoding.
func (m *WriteJSON) SetEnableDynamicgo(enable bool) {
	m.enableDynamicgo = enable
}

// SetConvOptions set the options to be used for dynamicgo encoding.
func (m *WriteJSON) SetConvOptions(opts conv.Options) {
	m.opts = opts
}

func (m *WriteJSON) originalWrite(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
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
	enableDynamicgo  bool
	opts             conv.Options
	msg              remote.Message
}

var _ MessageReader = (*ReadJSON)(nil)

// SetBinaryWithBase64 enable/disable Base64 encoding for binary.
// Note that this method is not concurrent-safe.
func (m *ReadJSON) SetBinaryWithBase64(enable bool) {
	m.binaryWithBase64 = enable
}

// SetEnableDynamicgo enable/disable dynamicgo decoding.
func (m *ReadJSON) SetEnableDynamicgo(enable bool) {
	m.enableDynamicgo = enable
}

// SetConvOptions set the options to be used for dynamicgo decoding.
func (m *ReadJSON) SetConvOptions(opts conv.Options) {
	m.opts = opts
}

// SetRemoteMessage set the remote message to be used for calculation of message length.
func (m *ReadJSON) SetRemoteMessage(msg remote.Message) {
	m.msg = msg
}

// Read read data from in thrift.TProtocol and convert to json string
func (m *ReadJSON) Read(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	// Transport protocol should be TTHeader, Framed, or TTHeaderFramed to enable dynamicgo
	if m.enableDynamicgo && m.msg.PayloadLen() != 0 {
		tProt, ok := in.(*cthrift.BinaryProtocol)
		if !ok {
			return nil, perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
		}

		var tyDsc *dthrift.TypeDescriptor
		if m.svc.DynamicgoDsc == nil {
			return nil, perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDsc is nil")
		}
		if m.msg.MessageType() == remote.Reply {
			tyDsc = m.svc.DynamicgoDsc.Functions()[method].Response()
		} else {
			tyDsc = m.svc.DynamicgoDsc.Functions()[method].Request()
		}
		var resp interface{}
		if tyDsc.Struct().Fields()[0].Type().Type() == dthrift.VOID {
			if _, err := tProt.ByteBuffer().ReadBinary(voidWholeLen); err != nil {
				return nil, err
			}
			resp = descriptor.Void{}
		} else {
			msgBeginLen := bthrift.Binary.MessageBeginLength(method, thrift.TMessageType(m.msg.MessageType()), m.msg.RPCInfo().Invocation().SeqID())
			transBuff, err := tProt.ByteBuffer().ReadBinary(m.msg.PayloadLen() - msgBeginLen - bthrift.Binary.MessageEndLength())
			if err != nil {
				return nil, err
			}

			buf := make([]byte, 0, len(transBuff)*2)
			cv := t2j.NewBinaryConv(m.opts)
			// thrift []byte to json []byte
			if err := cv.DoInto(ctx, tyDsc, transBuff, &buf); err != nil {
				return nil, err
			}
			if len(buf) > structWrapLen {
				buf = buf[structWrapLen : len(buf)-1]
			}
			resp = string(buf)
			if tyDsc.Struct().Fields()[0].Type().Type() == dthrift.STRING {
				resp, err = strconv.Unquote(resp.(string))
				if err != nil {
					return nil, err
				}
			}
		}

		return resp, nil
	}
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
