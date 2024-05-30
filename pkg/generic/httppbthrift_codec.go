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
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/jhump/protoreflect/desc"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	_ Closer                = &httpPbThriftCodec{}
	_ serviceinfo.CodecInfo = &httpPbThriftCodec{}
)

type httpPbThriftCodec struct {
	svcDsc     atomic.Value // *idl
	pbSvcDsc   atomic.Value // *pbIdl
	provider   DescriptorProvider
	pbProvider PbDescriptorProvider
	svcName    string
	method     string
}

func newHTTPPbThriftCodec(p DescriptorProvider, pbp PbDescriptorProvider) *httpPbThriftCodec {
	svc := <-p.Provide()
	pbSvc := <-pbp.Provide()
	c := &httpPbThriftCodec{provider: p, pbProvider: pbp, svcName: svc.Name}
	c.svcDsc.Store(svc)
	c.pbSvcDsc.Store(pbSvc)
	go c.update()
	return c
}

func (c *httpPbThriftCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}

		pbSvc, ok := <-c.pbProvider.Provide()
		if !ok {
			return
		}

		c.svcName = svc.Name
		c.svcDsc.Store(svc)
		c.pbSvcDsc.Store(pbSvc)
	}
}

func (c *httpPbThriftCodec) getMethod(req interface{}) (*Method, error) {
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

func (c *httpPbThriftCodec) GetMessageReaderWriter() interface{} {
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return errors.New("get parser ServiceDescriptor failed")
	}
	pbSvcDsc, ok := c.pbSvcDsc.Load().(*desc.ServiceDescriptor)
	if !ok {
		return errors.New("get parser PbServiceDescriptor failed")
	}

	return thrift.NewHTTPPbReaderWriter(svcDsc, pbSvcDsc)
}

func (c *httpPbThriftCodec) SetMethod(method string) {
	c.method = method
}

// SetIsClient for httpPb generic does nothing because httpPb generic is only for client
func (c *httpPbThriftCodec) SetIsClient(isClient bool) {
	return
}

func (c *httpPbThriftCodec) GetIDLServiceName() string {
	return c.svcName
}

func (c *httpPbThriftCodec) Name() string {
	return "HttpPbThrift"
}

func (c *httpPbThriftCodec) Close() error {
	var errs []string
	if err := c.provider.Close(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := c.pbProvider.Close(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) == 0 {
		return nil
	} else {
		return errors.New(strings.Join(errs, ";"))
	}
}

// FromHTTPPbRequest parse  HTTPRequest from http.Request
func FromHTTPPbRequest(req *http.Request) (*HTTPRequest, error) {
	customReq := &HTTPRequest{
		Request:     req,
		ContentType: descriptor.MIMEApplicationProtobuf,
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
	if len(customReq.RawBody) == 0 {
		return customReq, nil
	}

	return customReq, nil
}
