//go:build !amd64 || !go1.16
// +build !amd64 !go1.16

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

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
)

// SetWriteHTTPRequest ...
func (w *WriteHTTPRequest) SetWriteHTTPRequest(opts *conv.Options, method string) error {
	w.dyOpts = opts
	return nil
}

// Write ...
func (w *WriteHTTPRequest) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *Base) error {
	req := msg.(*descriptor.HTTPRequest)
	if req.Body == nil && len(req.RawBody) != 0 {
		if err := customJson.Unmarshal(req.RawBody, &req.Body); err != nil {
			return err
		}
	}
	fn, err := w.svc.Router.Lookup(req)
	if err != nil {
		return err
	}
	if !fn.HasRequestBase {
		requestBase = nil
	}
	return wrapStructWriter(ctx, req, out, fn.Request, &writerOption{requestBase: requestBase, binaryWithBase64: !w.dyOpts.NoBase64Binary})
}
