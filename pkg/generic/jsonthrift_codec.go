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

	"github.com/cloudwego/dynamicgo/conv/j2t"
	"github.com/cloudwego/dynamicgo/conv/t2j"

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
	svcDsc                      atomic.Value // *idl
	provider                    DescriptorProvider
	codec                       remote.PayloadCodec
	binaryWithBase64            bool
	opts                        *Options
	j2tBinaryConv               j2t.BinaryConv // used for dynamicgo json to thrift conversion
	j2tBinaryConvWithThriftBase j2t.BinaryConv // used for dynamicgo json to thrift conversion with EnableThriftBase turned on
	t2jBinaryConv               t2j.BinaryConv // used for dynamicgo thrift to json conversion
	t2jBinaryConvWithException  t2j.BinaryConv // used for dynamicgo thrift to json conversion with ConvertException turned on
	dynamicgoEnabled            bool
}

func newJsonThriftCodec(p DescriptorProvider, codec remote.PayloadCodec, opts *Options) (*jsonThriftCodec, error) {
	svc := <-p.Provide()
	c := &jsonThriftCodec{codec: codec, provider: p, binaryWithBase64: true, opts: opts, dynamicgoEnabled: false}
	if dp, ok := p.(GetProviderOption); ok && dp.Option().DynamicGoEnabled {
		c.dynamicgoEnabled = true

		convOpts := opts.dynamicgoConvOpts
		c.j2tBinaryConv = j2t.NewBinaryConv(convOpts)
		c.t2jBinaryConv = t2j.NewBinaryConv(convOpts)

		convOptsWithThriftBase := convOpts
		convOptsWithThriftBase.EnableThriftBase = true
		c.j2tBinaryConvWithThriftBase = j2t.NewBinaryConv(convOptsWithThriftBase)

		convOptsWithException := convOpts
		convOptsWithException.ConvertException = true
		c.t2jBinaryConvWithException = t2j.NewBinaryConv(convOptsWithException)
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
		wm.SetDynamicGo(svcDsc, method, &c.j2tBinaryConv, &c.j2tBinaryConvWithThriftBase)
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
		rm.SetDynamicGo(&c.t2jBinaryConv, &c.t2jBinaryConvWithException, msg)
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
