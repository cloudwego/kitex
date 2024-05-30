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
	"github.com/pkg/errors"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	_ serviceinfo.CodecInfo = &mapThriftCodec{}
	_ Closer                = &mapThriftCodec{}
)

type mapThriftCodec struct {
	svcDsc                  atomic.Value // *idl
	provider                DescriptorProvider
	forJSON                 bool
	binaryWithBase64        bool
	binaryWithByteSlice     bool
	setFieldsForEmptyStruct uint8
	svcName                 string
	method                  string
	isClient                bool
}

func newMapThriftCodec(p DescriptorProvider) *mapThriftCodec {
	svc := <-p.Provide()
	c := &mapThriftCodec{
		provider:            p,
		binaryWithBase64:    false,
		binaryWithByteSlice: false,
		svcName:             svc.Name,
	}
	c.svcDsc.Store(svc)
	go c.update()
	return c
}

func newMapThriftCodecForJSON(p DescriptorProvider) *mapThriftCodec {
	c := newMapThriftCodec(p)
	c.forJSON = true
	return c
}

func (c *mapThriftCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcName = svc.Name
		c.svcDsc.Store(svc)
	}
}

func (c *mapThriftCodec) GetMessageReaderWriter() interface{} {
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return errors.New("get parser ServiceDescriptor failed")
	}
	var rw *thrift.StructReaderWriter
	var err error
	if c.forJSON {
		rw, err = thrift.NewStructReaderWriterForJSON(svcDsc, c.method, c.isClient)
	} else {
		rw, err = thrift.NewStructReaderWriter(svcDsc, c.method, c.isClient)
	}
	if err != nil {
		return err
	}
	c.configureStructWriter(rw.WriteStruct)
	c.configureStructReader(rw.ReadStruct)
	return rw
}

func (c *mapThriftCodec) configureStructWriter(writer *thrift.WriteStruct) {
	writer.SetBinaryWithBase64(c.binaryWithBase64)
}

func (c *mapThriftCodec) configureStructReader(reader *thrift.ReadStruct) {
	reader.SetBinaryOption(c.binaryWithBase64, c.binaryWithByteSlice)
	reader.SetSetFieldsForEmptyStruct(c.setFieldsForEmptyStruct)
}

func (c *mapThriftCodec) SetMethod(method string) {
	c.method = method
}

func (c *mapThriftCodec) SetIsClient(isClient bool) {
	c.isClient = isClient
}

func (c *mapThriftCodec) GetIDLServiceName() string {
	return c.svcName
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
