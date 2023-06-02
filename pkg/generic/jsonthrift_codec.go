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
	"sync/atomic"

	"github.com/cloudwego/dynamicgo/conv"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	_ remote.PayloadCodec = &jsonThriftCodec{}
	_ Closer              = &jsonThriftCodec{}
)

// JSONRequest alias of string
type JSONRequest = string

type jsonThriftCodec struct {
	svcDsc                   atomic.Value // *idl
	provider                 DescriptorProvider
	codec                    remote.PayloadCodec
	binaryWithBase64         bool
	dyConvOpts               conv.Options
	dyConvOptsWithThriftBase conv.Options
	dyConvOptsWithException  conv.Options
	dynamicgoEnabled         bool
}

func newJsonThriftCodec(p DescriptorProvider, codec remote.PayloadCodec, opts *Options) (*jsonThriftCodec, error) {
	svc := <-p.Provide()
	c := &jsonThriftCodec{codec: codec, provider: p, binaryWithBase64: true, dynamicgoEnabled: false}
	if dp, ok := p.(GetProviderOption); ok && dp.Option().DynamicGoExpected {
		c.dynamicgoEnabled = true
		if !opts.isSetDynamicGoConvOpts {
			// set default dynamicgo conv.Options if it is not specified
			opts.dynamicgoConvOpts = defaultJSONDynamicGoConvOpts
			opts.isSetDynamicGoConvOpts = true
		}
		c.dyConvOpts = opts.dynamicgoConvOpts

		convOptsWithThriftBase := opts.dynamicgoConvOpts
		convOptsWithThriftBase.EnableThriftBase = true
		c.dyConvOptsWithThriftBase = convOptsWithThriftBase

		convOptsWithException := opts.dynamicgoConvOpts
		convOptsWithException.ConvertException = true
		c.dyConvOptsWithException = convOptsWithException
	}
	c.svcDsc.Store(svc)
	go c.update()
	return c, nil
}

func (c *jsonThriftCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcDsc.Store(svc)
	}
}

func (c *jsonThriftCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	method := msg.RPCInfo().Invocation().MethodName()
	if method == "" {
		return perrors.NewProtocolErrorWithMsg("empty methodName in thrift Marshal")
	}
	if msg.MessageType() == remote.Exception {
		return c.codec.Marshal(ctx, msg, out)
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}

	wm, err := thrift.NewWriteJSON(svcDsc, method, msg.RPCRole() == remote.Client)
	if err != nil {
		return err
	}
	wm.SetBase64Binary(c.binaryWithBase64)
	if c.dynamicgoEnabled {
		wm.SetDynamicGo(svcDsc, method, &c.dyConvOpts, &c.dyConvOptsWithThriftBase)
	}

	msg.Data().(WithCodec).SetCodec(wm)
	return c.codec.Marshal(ctx, msg, out)
}

func (c *jsonThriftCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	}

	rm := thrift.NewReadJSON(svcDsc, msg.RPCRole() == remote.Client)
	rm.SetBinaryWithBase64(c.binaryWithBase64)
	// Transport protocol should be TTHeader, Framed, or TTHeaderFramed to enable dynamicgo
	if c.dynamicgoEnabled && msg.PayloadLen() != 0 {
		rm.SetDynamicGo(&c.dyConvOpts, &c.dyConvOptsWithException, msg)
	}

	msg.Data().(WithCodec).SetCodec(rm)
	return c.codec.Unmarshal(ctx, msg, in)
}

func (c *jsonThriftCodec) getMethod(req interface{}, method string) (*Method, error) {
	fnSvc, err := c.svcDsc.Load().(*descriptor.ServiceDescriptor).LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	return &Method{method, fnSvc.Oneway}, nil
}

func (c *jsonThriftCodec) Name() string {
	return "JSONThrift"
}

func (c *jsonThriftCodec) Close() error {
	return c.provider.Close()
}
