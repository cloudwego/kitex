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
	"fmt"
	"io"
	"unsafe"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/j2t"
	dbase "github.com/cloudwego/dynamicgo/thrift/base"
	"github.com/cloudwego/gopkg/protocol/thrift"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	gthrift "github.com/cloudwego/kitex/pkg/generic/thrift"
)

// Write ...
func (w *WriteHTTPRequest) Write(ctx context.Context, out io.Writer, msg interface{}, method string, isClient bool, requestBase *gthrift.Base) error {
	// fallback logic
	if !w.dynamicgoEnabled {
		return w.originalWrite(ctx, out, msg, requestBase)
	}

	// dynamicgo logic
	req := msg.(*descriptor.HTTPRequest)

	fnDsc := w.svc.DynamicGoDsc.Functions()[method]
	if fnDsc == nil {
		return fmt.Errorf("missing method: %s in service: %s in dynamicgo", method, w.svc.DynamicGoDsc.Name())
	}
	dynamicgoTypeDsc := fnDsc.Request()

	var cv j2t.BinaryConv
	if !fnDsc.HasRequestBase() {
		requestBase = nil
	}
	if requestBase != nil {
		base := (*dbase.Base)(unsafe.Pointer(requestBase))
		ctx = context.WithValue(ctx, conv.CtxKeyThriftReqBase, base)
		cv = j2t.NewBinaryConv(w.convOptsWithThriftBase)
	} else {
		cv = j2t.NewBinaryConv(w.convOpts)
	}

	binaryWriter := thrift.NewBinaryWriter()

	ctx = context.WithValue(ctx, conv.CtxKeyHTTPRequest, req)
	body := req.GetBody()
	dbuf := mcache.Malloc(len(body))[0:0]
	defer mcache.Free(dbuf)

	for _, field := range dynamicgoTypeDsc.Struct().Fields() {
		binaryWriter.WriteFieldBegin(thrift.TType(field.Type().Type()), int16(field.ID()))

		// json []byte to thrift []byte
		if err := cv.DoInto(ctx, field.Type(), body, &dbuf); err != nil {
			return err
		}
	}
	if _, err := out.Write(binaryWriter.Bytes()); err != nil {
		return err
	}
	if _, err := out.Write(dbuf); err != nil {
		return err
	}
	binaryWriter.Reset()
	binaryWriter.WriteFieldStop()
	_, err := out.Write(binaryWriter.Bytes())
	return err
}
