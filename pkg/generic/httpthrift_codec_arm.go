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

package generic

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote"
)

type httpThriftCodec struct {
	svcDsc           atomic.Value // *idl
	provider         DescriptorProvider
	codec            remote.PayloadCodec
	binaryWithBase64 bool
}

func newHTTPThriftCodec(p DescriptorProvider, codec remote.PayloadCodec) (*httpThriftCodec, error) {
	svc := <-p.Provide()
	c := &httpThriftCodec{codec: codec, provider: p, binaryWithBase64: false}
	c.svcDsc.Store(svc)
	go c.update()
	return c, nil
}

func (c *httpThriftCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	return c.originalMarshal(ctx, msg, out)
}

func (c *httpThriftCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	return c.originalUnmarshal(ctx, msg, in)
}

func (c *httpThriftCodec) Name() string {
	return "HttpThrift"
}

// FromHTTPRequest parse  HTTPRequest from http.Request
func FromHTTPRequest(req *http.Request) (customReq *HTTPRequest, err error) {
	customReq = &HTTPRequest{
		Header:      req.Header,
		Query:       req.URL.Query(),
		Cookies:     descriptor.Cookies{},
		Method:      req.Method,
		Host:        req.Host,
		Path:        req.URL.Path,
		ContentType: descriptor.MIMEApplicationJson,
		Body:        map[string]interface{}{},
	}
	for _, cookie := range req.Cookies() {
		customReq.Cookies[cookie.Name] = cookie.Value
	}
	// copy the request body, maybe user used it after parse
	var b io.ReadCloser
	if req.GetBody != nil {
		// req from ServerHTTP or create by http.NewRequest
		if b, err = req.GetBody(); err != nil {
			return nil, err
		}
	} else {
		b = req.Body
	}
	if b == nil {
		// body == nil if from Get request
		return customReq, nil
	}
	if customReq.RawBody, err = ioutil.ReadAll(b); err != nil {
		return nil, err
	}
	if len(customReq.RawBody) == 0 {
		return customReq, nil
	}
	return customReq, json.Unmarshal(customReq.RawBody, &customReq.Body)
}
