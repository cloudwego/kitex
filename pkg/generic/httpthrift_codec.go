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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync/atomic"

	"github.com/cloudwego/dynamicgo/conv"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	_ remote.PayloadCodec = &httpThriftCodec{}
	_ Closer              = &httpThriftCodec{}
)

// HTTPRequest alias of descriptor HTTPRequest
type HTTPRequest = descriptor.HTTPRequest

// HTTPResponse alias of descriptor HTTPResponse
type HTTPResponse = descriptor.HTTPResponse

type httpThriftCodec struct {
	svcDsc                 atomic.Value // *idl
	provider               DescriptorProvider
	codec                  remote.PayloadCodec
	binaryWithBase64       bool
	convOpts               conv.Options // used for dynamicgo conversion
	convOptsWithThriftBase conv.Options // used for dynamicgo conversion with EnableThriftBase turned on
	dynamicgoEnabled       bool
	useRawBodyForHTTPResp  bool
}

func newHTTPThriftCodec(p DescriptorProvider, codec remote.PayloadCodec, opts *Options) (*httpThriftCodec, error) {
	svc := <-p.Provide()
	c := &httpThriftCodec{codec: codec, provider: p, binaryWithBase64: false, dynamicgoEnabled: false, useRawBodyForHTTPResp: opts.useRawBodyForHTTPResp}
	if dp, ok := p.(GetProviderOption); ok && dp.Option().DynamicGoEnabled {
		c.dynamicgoEnabled = true

		convOpts := opts.dynamicgoConvOpts
		c.convOpts = convOpts

		convOpts.EnableThriftBase = true
		c.convOptsWithThriftBase = convOpts
	}
	c.svcDsc.Store(svc)
	go c.update()
	return c, nil
}

func (c *httpThriftCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcDsc.Store(svc)
	}
}

func (c *httpThriftCodec) getMethod(req interface{}) (*Method, error) {
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return nil, errors.New("get method name failed, no ServiceDescriptor")
	}
	r, ok := req.(*HTTPRequest)
	if !ok {
		return nil, errors.New("req is invalid, need descriptor.HTTPRequest")
	}
	function, err := svcDsc.Router.Lookup(r)
	if err != nil {
		return nil, err
	}
	return &Method{function.Name, function.Oneway}, nil
}

func (c *httpThriftCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("get parser ServiceDescriptor failed")
	}

	inner := thrift.NewWriteHTTPRequest(svcDsc)
	inner.SetBinaryWithBase64(c.binaryWithBase64)
	if c.dynamicgoEnabled {
		if err := inner.SetDynamicGo(&c.convOpts, &c.convOptsWithThriftBase, msg.RPCInfo().Invocation().MethodName()); err != nil {
			return err
		}
	}

	msg.Data().(WithCodec).SetCodec(inner)
	return c.codec.Marshal(ctx, msg, out)
}

func (c *httpThriftCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("get parser ServiceDescriptor failed")
	}

	inner := thrift.NewReadHTTPResponse(svcDsc)
	inner.SetBase64Binary(c.binaryWithBase64)
	inner.SetUseRawBodyForHTTPResp(c.useRawBodyForHTTPResp)
	if c.dynamicgoEnabled && c.useRawBodyForHTTPResp {
		inner.SetDynamicGo(&c.convOpts, msg)
	}

	msg.Data().(WithCodec).SetCodec(inner)
	return c.codec.Unmarshal(ctx, msg, in)
}

func (c *httpThriftCodec) Name() string {
	return "HttpThrift"
}

func (c *httpThriftCodec) Close() error {
	return c.provider.Close()
}

// FromHTTPRequest parse HTTPRequest from http.Request
func FromHTTPRequest(req *http.Request) (*HTTPRequest, error) {
	customReq := &HTTPRequest{
		Request:     req,
		ContentType: descriptor.MIMEApplicationJson,
	}
	var b io.ReadCloser
	var err error
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
	return customReq, nil
}
