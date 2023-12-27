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

package generic

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/cloudwego/dynamicgo/conv"
	dproto "github.com/cloudwego/dynamicgo/proto"

	"github.com/cloudwego/kitex/pkg/generic/proto"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	perrors "github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/jhump/protoreflect/desc"
)

var (
	_ remote.PayloadCodec = &jsonPbCodec{}
	_ Closer              = &jsonPbCodec{}
)

type jsonPbCodec struct {
	svcDsc           atomic.Value // *idl
	dynamicgoSvcDsc  *dproto.ServiceDescriptor
	provider         PbDescriptorProvider
	codec            remote.PayloadCodec
	binaryWithBase64 bool
	opts             *Options
	convOpts         conv.Options // used for dynamicgo conversion
	dynamicgoEnabled bool         // currently set to true by default
}

func newJsonPbCodec(p PbDescriptorProvider, dp *dproto.ServiceDescriptor, codec remote.PayloadCodec, opts *Options) (*jsonPbCodec, error) {
	svc := <-p.Provide()
	c := &jsonPbCodec{codec: codec, provider: p, dynamicgoSvcDsc: dp, binaryWithBase64: true, opts: opts, dynamicgoEnabled: true}
	convOpts := opts.dynamicgoConvOpts
	c.convOpts = convOpts

	c.svcDsc.Store(svc)
	go c.update()
	return c, nil
}

func (c *jsonPbCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcDsc.Store(svc)
	}
}

func (c *jsonPbCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	method := msg.RPCInfo().Invocation().MethodName()
	if method == "" {
		return perrors.NewProtocolErrorWithMsg("empty methodName in protobuf Marshal")
	}
	if msg.MessageType() == remote.Exception {
		return c.codec.Marshal(ctx, msg, out)
	}
	// pbSvc := c.svcDsc.Load().(*desc.ServiceDescriptor)
	pbSvc := c.dynamicgoSvcDsc

	wm, err := proto.NewWriteJSON(pbSvc, method, msg.RPCRole() == remote.Client, &c.convOpts)
	if err != nil {
		return err
	}

	msg.Data().(WithCodec).SetCodec(wm)

	return c.codec.Marshal(ctx, msg, out)
}

func (c *jsonPbCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}

	pbSvc := c.dynamicgoSvcDsc

	wm, err := proto.NewReadJSON(pbSvc, msg.RPCRole() == remote.Client, &c.convOpts)
	if err != nil {
		return err
	}

	msg.Data().(WithCodec).SetCodec(wm)

	return c.codec.Unmarshal(ctx, msg, in)
}

func (c *jsonPbCodec) getMethod(req interface{}, method string) (*Method, error) {
	fnSvc := c.svcDsc.Load().(*desc.ServiceDescriptor).FindMethodByName(method)
	if fnSvc == nil {
		return nil, fmt.Errorf("missing method: %s in service", method)
	}

	return &Method{method, fnSvc.IsClientStreaming() || fnSvc.IsServerStreaming()}, nil
}

func (c *jsonPbCodec) Name() string {
	return "JSONPb"
}

func (c *jsonPbCodec) Close() error {
	return c.provider.Close()
}
