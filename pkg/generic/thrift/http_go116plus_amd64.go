//go:build amd64 && go1.16
// +build amd64,go1.16

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

package thrift

import (
	"context"
	"unsafe"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/j2t"
	"github.com/cloudwego/dynamicgo/thrift/base"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	thrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
)

// Write ...
func (w *WriteHTTPRequest) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	// fallback logic
	if !w.dynamicgoEnabled {
		return w.originalWrite(ctx, out, msg, requestBase)
	}

	// dynamicgo logic
	req := msg.(*descriptor.HTTPRequest)

	var cv j2t.BinaryConv
	if !w.hasRequestBase {
		requestBase = nil
	}
	if requestBase != nil {
		base := (*base.Base)(unsafe.Pointer(requestBase))
		ctx = context.WithValue(ctx, conv.CtxKeyThriftReqBase, base)
		cv = j2t.NewBinaryConv(w.convOptsWithThriftBase)
	} else {
		cv = j2t.NewBinaryConv(w.convOpts)
	}

	if err := out.WriteStructBegin(w.dynamicgoTypeDsc.Struct().Name()); err != nil {
		return err
	}

	ctx = context.WithValue(ctx, conv.CtxKeyHTTPRequest, req)
	body := req.GetBody()
	dbuf := mcache.Malloc(len(body))[0:0]
	defer mcache.Free(dbuf)

	for _, field := range w.dynamicgoTypeDsc.Struct().Fields() {
		if err := out.WriteFieldBegin(field.Name(), field.Type().Type().ToThriftTType(), int16(field.ID())); err != nil {
			return err
		}

		// json []byte to thrift []byte
		if err := cv.DoInto(ctx, field.Type(), body, &dbuf); err != nil {
			return err
		}

		// WriteFieldEnd has no content
		// if err := out.WriteFieldEnd(); err != nil {
		// 	return err
		// }
	}

	tProt, ok := out.(*cthrift.BinaryProtocol)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
	}
	buf, err := tProt.ByteBuffer().Malloc(len(dbuf))
	if err != nil {
		return err
	}
	// TODO: implement MallocAck() to achieve zero copy
	copy(buf, dbuf)

	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	if err := out.WriteStructEnd(); err != nil {
		return err
	}
	return nil
}
