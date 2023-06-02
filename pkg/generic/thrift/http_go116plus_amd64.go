//go:build amd64 && go1.16
// +build amd64,go1.16

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
	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/j2t"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
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
		cv = j2t.NewBinaryConv(*w.dyConvOptsWithThriftBase)
	} else {
		cv = j2t.NewBinaryConv(*w.dyConvOpts)
	}

	body := req.GetBody()

	ctx = context.WithValue(ctx, conv.CtxKeyHTTPRequest, req)

	tProt, ok := out.(*cthrift.BinaryProtocol)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
	}

	if err := out.WriteStructBegin(w.ty.Struct().Name()); err != nil {
		return err
	}

	dbuf := mcache.Malloc(len(body))[0:0]

	for _, field := range w.ty.Struct().Fields() {
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

	buf, err := tProt.ByteBuffer().Malloc(len(dbuf))
	if err != nil {
		return err
	} else {
		// TODO: implement MallocAck() to achieve zero copy
		copy(buf, dbuf)
	}
	mcache.Free(dbuf)

	if err := out.WriteFieldStop(); err != nil {
		return err
	}
	if err := out.WriteStructEnd(); err != nil {
		return err
	}
	return nil
}
