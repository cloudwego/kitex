//go:build amd64
// +build amd64

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
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/j2t"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	cthrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
)

// Write ...
func (w *WriteHTTPRequest) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	if w.enableDynamicgo {
		req := msg.(*descriptor.HTTPRequest)
		fn, err := w.svc.Router.Lookup(req)
		if err != nil {
			return err
		}
		if w.svc.DynamicgoDsc == nil {
			return perrors.NewProtocolErrorWithMsg("svcDsc.DynamicgoDsc is nil")
		}
		tyDsc := w.svc.DynamicgoDsc.Functions()[fn.Name].Request().Struct().Fields()[0].Type()

		body := req.GetBody()
		buf := make([]byte, fieldBeginLen, len(body)+structWrapLen)
		offset := 0
		offset += bthrift.Binary.WriteStructBegin(buf[offset:], "")
		bthrift.Binary.WriteFieldBegin(buf[offset:], "", thrift.STRUCT, 1)
		ctx = context.WithValue(ctx, conv.CtxKeyHTTPRequest, req)
		cv := j2t.NewBinaryConv(w.opts)
		if err = cv.DoInto(ctx, tyDsc, body, &buf); err != nil {
			return err
		}
		offset = len(buf)
		offset += bthrift.Binary.WriteStructEnd(buf[offset:])
		buf = append(buf, 0)
		offset += bthrift.Binary.WriteFieldStop(buf[offset:])
		bthrift.Binary.WriteStructEnd(buf[offset:])

		tProt, ok := out.(*cthrift.BinaryProtocol)
		if !ok {
			return perrors.NewProtocolErrorWithMsg("TProtocol should be BinaryProtocol")
		}
		_, err = tProt.ByteBuffer().WriteBinary(buf)
		return err
	}
	return w.originalWrite(ctx, out, msg, requestBase)
}
