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
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	_ remote.PayloadCodec = &mapThriftCodec{}
	_ Closer              = &mapThriftCodec{}
)

type mapThriftCodec struct {
	svcDsc   atomic.Value // *idl
	provider DescriptorProvider
	codec    remote.PayloadCodec
	forJSON  bool
}

func newMapThriftCodec(p DescriptorProvider, codec remote.PayloadCodec) (*mapThriftCodec, error) {
	svc := <-p.Provide()
	c := &mapThriftCodec{
		codec:    codec,
		provider: p,
	}
	c.svcDsc.Store(svc)
	go c.update()
	return c, nil
}

func newMapThriftCodecForJSON(p DescriptorProvider, codec remote.PayloadCodec) (*mapThriftCodec, error) {
	c, err := newMapThriftCodec(p, codec)
	if err != nil {
		return nil, err
	}
	c.forJSON = true
	return c, nil
}

func (c *mapThriftCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcDsc.Store(svc)
	}
}

func (c *mapThriftCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	method := msg.RPCInfo().Invocation().MethodName()
	if method == "" {
		return errors.New("empty methodName in thrift Marshal")
	}
	if msg.MessageType() == remote.Exception {
		return c.codec.Marshal(ctx, msg, out)
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("get parser ServiceDescriptor failed")
	}
	wm, err := thrift.NewWriteStruct(svcDsc, method, msg.RPCRole() == remote.Client)
	if err != nil {
		return err
	}
	msg.Data().(WithCodec).SetCodec(wm)
	return c.codec.Marshal(ctx, msg, out)
}

func (c *mapThriftCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("get parser ServiceDescriptor failed")
	}
	var rm *thrift.ReadStruct
	if c.forJSON {
		rm = thrift.NewReadStructForJSON(svcDsc, msg.RPCRole() == remote.Client)
	} else {
		rm = thrift.NewReadStruct(svcDsc, msg.RPCRole() == remote.Client)
	}
	msg.Data().(WithCodec).SetCodec(rm)
	return c.codec.Unmarshal(ctx, msg, in)
}

func (c *mapThriftCodec) getMethod(req interface{}, method string) (*Method, error) {
	fnSvc, err := c.svcDsc.Load().(*descriptor.ServiceDescriptor).LookupFunctionByMethod(method)
	if err != nil {
		return nil, err
	}
	return &Method{method, fnSvc.Oneway}, nil
}

func (c *mapThriftCodec) Name() string {
	return "MapThrift"
}

func (c *mapThriftCodec) Close() error {
	return c.provider.Close()
}
