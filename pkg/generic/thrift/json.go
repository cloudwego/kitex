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

	"github.com/bytedance/gopkg/lang/dirtmake"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/t2j"
	dthrift "github.com/cloudwego/dynamicgo/thrift"
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/thrift/base"
	"github.com/tidwall/gjson"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/utils"
)

var jsonSonic = sonic.Config{EscapeHTML: true}.Froze()

type JSONReaderWriter struct {
	*ReadJSON
	*WriteJSON
}

func NewJsonReaderWriter(svc *descriptor.ServiceDescriptor) *JSONReaderWriter {
	return &JSONReaderWriter{ReadJSON: NewReadJSON(svc), WriteJSON: NewWriteJSON(svc)}
}

// NewWriteJSON build WriteJSON according to ServiceDescriptor
func NewWriteJSON(svc *descriptor.ServiceDescriptor) *WriteJSON {
	return &WriteJSON{
		svcDsc:           svc,
		base64Binary:     true,
		dynamicgoEnabled: false,
	}
}

const voidWholeLen = 5

var _ = wrapJSONWriter

// WriteJSON implement of MessageWriter
type WriteJSON struct {
	svcDsc                 *descriptor.ServiceDescriptor
	base64Binary           bool
	convOpts               conv.Options // used for dynamicgo conversion
	convOptsWithThriftBase conv.Options // used for dynamicgo conversion with EnableThriftBase turned on
	dynamicgoEnabled       bool
}

var _ MessageWriter = (*WriteJSON)(nil)

// SetBase64Binary enable/disable Base64 decoding for binary.
// Note that this method is not concurrent-safe.
func (m *WriteJSON) SetBase64Binary(enable bool) {
	m.base64Binary = enable
}

// SetDynamicGo ...
func (m *WriteJSON) SetDynamicGo(convOpts, convOptsWithThriftBase *conv.Options) {
	m.convOpts = *convOpts
	m.convOptsWithThriftBase = *convOptsWithThriftBase
	m.dynamicgoEnabled = true
}

func (m *WriteJSON) originalWrite(ctx context.Context, out bufiox.Writer, msg interface{}, method string, isClient bool, requestBase *base.Base) error {
	fnDsc, err := m.svcDsc.LookupFunctionByMethod(method)
	if err != nil {
		return fmt.Errorf("missing method: %s in service: %s", method, m.svcDsc.Name)
	}
	typeDsc := fnDsc.Request
	if !isClient {
		typeDsc = fnDsc.Response
	}

	hasRequestBase := fnDsc.HasRequestBase && isClient
	if !hasRequestBase {
		requestBase = nil
	}

	bw := thrift.NewBufferWriter(out)
	defer bw.Recycle()

	// msg is void or nil
	if _, ok := msg.(descriptor.Void); ok || msg == nil {
		if err = wrapStructWriter(ctx, msg, bw, typeDsc, &writerOption{requestBase: requestBase, binaryWithBase64: m.base64Binary}); err != nil {
			return err
		}
		return err
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

	opt := &writerOption{requestBase: requestBase, binaryWithBase64: m.base64Binary}
	if fnDsc.IsWithoutWrapping {
		return jsonWriter(ctx, &body, typeDsc, opt, bw)
	}

	return wrapJSONWriter(ctx, &body, bw, typeDsc, &writerOption{requestBase: requestBase, binaryWithBase64: m.base64Binary})
}

// NewReadJSON build ReadJSON according to ServiceDescriptor
func NewReadJSON(svc *descriptor.ServiceDescriptor) *ReadJSON {
	return &ReadJSON{
		svc:              svc,
		binaryWithBase64: true,
		dynamicgoEnabled: false,
	}
}

// ReadJSON implement of MessageReaderWithMethod
type ReadJSON struct {
	svc                   *descriptor.ServiceDescriptor
	binaryWithBase64      bool
	convOpts              conv.Options // used for dynamicgo conversion
	convOptsWithException conv.Options // used for dynamicgo conversion which also handles an exception field
	dynamicgoEnabled      bool
}

var _ MessageReader = (*ReadJSON)(nil)

// SetBinaryWithBase64 enable/disable Base64 encoding for binary.
// Note that this method is not concurrent-safe.
func (m *ReadJSON) SetBinaryWithBase64(enable bool) {
	m.binaryWithBase64 = enable
}

// SetDynamicGo ...
func (m *ReadJSON) SetDynamicGo(convOpts, convOptsWithException *conv.Options) {
	m.dynamicgoEnabled = true
	m.convOpts = *convOpts
	m.convOptsWithException = *convOptsWithException
}

// Read read data from in thrift.TProtocol and convert to json string
func (m *ReadJSON) Read(ctx context.Context, method string, isClient bool, dataLen int, in bufiox.Reader) (interface{}, error) {
	// fallback logic
	if !m.dynamicgoEnabled || dataLen <= 0 {
		return m.originalRead(ctx, method, isClient, in)
	}

	fnDsc := m.svc.DynamicGoDsc.Functions()[method]
	if fnDsc == nil {
		return nil, fmt.Errorf("missing method: %s in service: %s in dynamicgo", method, m.svc.DynamicGoDsc.Name())
	}
	tyDsc := fnDsc.Response()
	if !isClient {
		tyDsc = fnDsc.Request()
	}

	var resp interface{}
	var err error
	if tyDsc.Struct().Fields()[0].Type().Type() == dthrift.VOID {
		if err = in.Skip(voidWholeLen); err != nil {
			return nil, err
		}
		resp = descriptor.Void{}
	} else {
		transBuff := dirtmake.Bytes(dataLen, dataLen)
		if _, err = in.ReadBinary(transBuff); err != nil {
			return nil, err
		}

		// json size is usually 2 times larger than equivalent thrift data
		buf := dirtmake.Bytes(0, len(transBuff)*2)
		// thrift []byte to json []byte
		var t2jBinaryConv t2j.BinaryConv
		if isClient && !fnDsc.IsWithoutWrapping() {
			t2jBinaryConv = t2j.NewBinaryConv(m.convOptsWithException)
		} else {
			t2jBinaryConv = t2j.NewBinaryConv(m.convOpts)
		}
		if err = t2jBinaryConv.DoInto(ctx, tyDsc, transBuff, &buf); err != nil {
			return nil, err
		}
		if !fnDsc.IsWithoutWrapping() {
			buf = removePrefixAndSuffix(buf)
		}
		resp = utils.SliceByteToString(buf)
		if !fnDsc.IsWithoutWrapping() && tyDsc.Struct().Fields()[0].Type().Type() == dthrift.STRING {
			strresp := resp.(string)
			resp, err = strconv.Unquote(strresp)
			if err != nil {
				return nil, err
			}
		}
	}

	return resp, nil
}

func (m *ReadJSON) originalRead(ctx context.Context, method string, isClient bool, in bufiox.Reader) (interface{}, error) {
	fnDsc, err := m.svc.LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	tyDsc := fnDsc.Response
	if !isClient {
		tyDsc = fnDsc.Request
	}
	br := thrift.NewBufferReader(in)
	defer br.Recycle()

	if fnDsc.IsWithoutWrapping {
		return structReader(ctx, tyDsc, &readerOption{forJSON: true, binaryWithBase64: m.binaryWithBase64}, br)
	}

	resp, err := skipStructReader(ctx, br, tyDsc, &readerOption{forJSON: true, throwException: true, binaryWithBase64: m.binaryWithBase64})
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
	respNode, err := jsonSonic.Marshal(resp)
	if err != nil {
		return nil, perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("response marshal failed. err:%#v", err))
	}

	return string(respNode), nil
}

// removePrefixAndSuffix removes json []byte from prefix `{"":` and suffix `}`
func removePrefixAndSuffix(buf []byte) []byte {
	if len(buf) > structWrapLen {
		return buf[structWrapLen : len(buf)-1]
	}
	return buf
}

func jsonWriter(ctx context.Context, body *gjson.Result, typeDsc *descriptor.TypeDescriptor, opt *writerOption, bw *thrift.BufferWriter) error {
	val, writer, err := nextJSONWriter(body, typeDsc, opt)
	if err != nil {
		return fmt.Errorf("nextWriter of field[%s] error %w", typeDsc.Name, err)
	}
	if err = writer(ctx, val, bw, typeDsc, opt); err != nil {
		return fmt.Errorf("writer of field[%s] error %w", typeDsc.Name, err)
	}
	return nil
}

func structReader(ctx context.Context, typeDesc *descriptor.TypeDescriptor, opt *readerOption, br *thrift.BufferReader) (v interface{}, err error) {
	resp, err := readStruct(ctx, br, typeDesc, opt)
	if err != nil {
		return nil, err
	}
	respNode, err := jsonSonic.Marshal(resp)
	if err != nil {
		return nil, perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("streaming response marshal failed. err:%#v", err))
	}
	return string(respNode), nil
}
