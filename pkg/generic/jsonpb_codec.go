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
	"fmt"
	"sync/atomic"

	"github.com/cloudwego/dynamicgo/conv"
	dproto "github.com/cloudwego/dynamicgo/proto"

	"github.com/cloudwego/kitex/pkg/generic/proto"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	//_ remote.PayloadCodec   = &jsonPbCodec{}
	_ Closer                = &jsonPbCodec{}
	_ serviceinfo.CodecInfo = &jsonPbCodec{}
)

type jsonPbCodec struct {
	svcDsc           atomic.Value // *idl
	provider         PbDescriptorProviderDynamicGo
	codec            remote.PayloadCodec
	opts             *Options
	convOpts         conv.Options // used for dynamicgo conversion
	dynamicgoEnabled bool         // currently set to true by default
	svcName          string
	method           string
	isClient         bool
}

func newJsonPbCodec(p PbDescriptorProviderDynamicGo, codec remote.PayloadCodec, opts *Options) *jsonPbCodec {
	svc := <-p.Provide()
	c := &jsonPbCodec{codec: codec, provider: p, opts: opts, dynamicgoEnabled: true, svcName: svc.Name()}
	convOpts := opts.dynamicgoConvOpts
	c.convOpts = convOpts

	c.svcDsc.Store(svc)
	go c.update()
	return c
}

func (c *jsonPbCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcName = svc.Name()
		c.svcDsc.Store(svc)
	}
}

func (c *jsonPbCodec) GetMessageReaderWriter() interface{} {
	pbSvc := c.svcDsc.Load().(*dproto.ServiceDescriptor)

	rw, err := proto.NewJsonReaderWriter(pbSvc, c.method, c.isClient, &c.convOpts)
	if err != nil {
		return nil
		// return nil, err
	}
	return rw
}

func (c *jsonPbCodec) SetMethod(method string) {
	c.method = method
}

func (c *jsonPbCodec) SetIsClient(isClient bool) {
	c.isClient = isClient
}

func (c *jsonPbCodec) GetIDLServiceName() string {
	return c.svcName
}

func (c *jsonPbCodec) getMethod(req interface{}, method string) (*Method, error) {
	fnSvc := c.svcDsc.Load().(*dproto.ServiceDescriptor).LookupMethodByName(method)
	if fnSvc == nil {
		return nil, fmt.Errorf("missing method: %s in service", method)
	}

	// protobuf does not have oneway methods
	return &Method{method, false}, nil
}

func (c *jsonPbCodec) Name() string {
	return "JSONPb"
}

func (c *jsonPbCodec) Close() error {
	return c.provider.Close()
}
