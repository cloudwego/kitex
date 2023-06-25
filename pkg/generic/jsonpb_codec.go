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
	"fmt"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	_ remote.PayloadCodec = &jsonPbCodec{}
	_ Closer              = &jsonPbCodec{}
)

type jsonPbCodec struct {
	svcDsc           atomic.Value // *idl
	provider         PbDescriptorProvider
	codec            remote.PayloadCodec
	binaryWithBase64 bool
}

func newJsonPbCodec(p PbDescriptorProvider, codec remote.PayloadCodec) (*jsonPbCodec, error) {
	svc := <-p.Provide()
	c := &jsonPbCodec{codec: codec, provider: p, binaryWithBase64: true}
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

	c.MarshalJSONToDynamicMessage(msg)

	return c.codec.Marshal(ctx, msg, out)
}

func (c *jsonPbCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	//svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	//if !ok {
	//	return perrors.NewProtocolErrorWithMsg("get parser ServiceDescriptor failed")
	//}
	//rm := thrift.NewReadJSON(svcDsc, msg.RPCRole() == remote.Client)
	//rm.SetBinaryWithBase64(c.binaryWithBase64)
	//msg.Data().(WithCodec).SetCodec(rm)
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

// Helper Functions
// Parses the json payload into a dynamic message and stores it in the inner field of msg.Data
func (c *jsonPbCodec) MarshalJSONToDynamicMessage(msg remote.Message) error {
	//jsonStr string, mtname string, pbSvc *desc.ServiceDescriptor
	//instantiate variables
	pbSvc := c.svcDsc.Load().(*desc.ServiceDescriptor)
	msgArgs := msg.Data().(*Args)
	mtname := msgArgs.Method
	mtBody, ok := msgArgs.Request.(string)
	if !ok {
		return perrors.NewProtocolErrorWithType(perrors.InvalidData, "request payload is not string")
	}
	mt := pbSvc.FindMethodByName(mtname)

	//create dynamic message
	inputDescriptor := mt.GetInputType()
	inputMessage := dynamic.NewMessage(inputDescriptor)

	var jsonMap map[string]interface{}
	err := json.Unmarshal([]byte(mtBody), &jsonMap)
	if err != nil {
		return perrors.NewProtocolErrorWithMsg("Incorrect JSON format")
	}

	//map data into inputMessage
	for fieldName, fieldValue := range jsonMap {
		err = inputMessage.TrySetFieldByName(fieldName, fieldValue)
		if err != nil {
			return perrors.NewProtocolErrorWithMsg("Incorrect JSON format")
		}
	}

	//set the inner field of msg.Data to be the inputMessage
	msg.SetData(inputMessage)

	return nil
}
