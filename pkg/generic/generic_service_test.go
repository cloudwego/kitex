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
	"strings"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/kitex/internal/test"
	gthrift "github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	codecThrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

type testImpl struct{}

// GenericCall test call
func (g *testImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	response = "test"
	if method != "test" {
		err = fmt.Errorf("method is invalid, method: %v", method)
	}
	return
}

type testWrite struct{}

// Write test write
func (m *testWrite) Write(ctx context.Context, out thrift.TProtocol, msg interface{}, requestBase *gthrift.Base) error {
	return nil
}

type testRead struct{}

// Read test read
func (m *testRead) Read(ctx context.Context, method string, in thrift.TProtocol) (interface{}, error) {
	return "test", nil
}

func TestGenericService(t *testing.T) {
	ctx := context.Background()
	method := "test"
	out := remote.NewWriterBuffer(256)
	tProto := codecThrift.NewBinaryProtocol(out)
	wInner, rInner := &testWrite{}, &testRead{}

	// Args...
	arg := newGenericServiceCallArgs()
	a, ok := arg.(*Args)
	test.Assert(t, ok == true)
	base := a.GetOrSetBase()
	test.Assert(t, base != nil)
	a.SetCodec(struct{}{})
	// write not ok
	err := a.Write(ctx, tProto)
	test.Assert(t, err.Error() == "unexpected Args writer type: struct {}")
	// write ok
	a.SetCodec(wInner)
	err = a.Write(ctx, tProto)
	test.Assert(t, err == nil)
	// read not ok
	err = a.Read(ctx, method, nil)
	test.Assert(t, strings.Contains(err.Error(), "unexpected Args reader type"))
	// read ok
	a.SetCodec(rInner)
	err = a.Read(ctx, method, tProto)
	test.Assert(t, err == nil)

	// Result...
	result := newGenericServiceCallResult()
	r, ok := result.(*Result)
	test.Assert(t, ok == true)

	// write not ok
	err = r.Write(ctx, nil)
	test.Assert(t, err.Error() == "unexpected Result writer type: <nil>")
	// write ok
	r.SetCodec(wInner)
	err = r.Write(ctx, tProto)
	test.Assert(t, err == nil)
	// read not ok
	err = r.Read(ctx, method, nil)
	test.Assert(t, err.Error() == "unexpected Result reader type: *generic.testWrite")
	// read ok
	r.SetCodec(rInner)
	err = r.Read(ctx, method, tProto)
	test.Assert(t, err == nil)

	r.SetSuccess(nil)
	success := r.GetSuccess()
	test.Assert(t, success == nil)
	r.SetSuccess(method)
	success = r.GetSuccess()
	test.Assert(t, success.(string) == method)

	// test callHandler...
	handler := &testImpl{}
	a.Method = ""
	err = callHandler(ctx, handler, arg, result)
	test.Assert(t, err.Error() == "method is invalid, method: ")
	a.Method = method
	err = callHandler(ctx, handler, arg, result)
	test.Assert(t, err == nil)
}

func TestServiceInfo(t *testing.T) {
	s := ServiceInfo(serviceinfo.Thrift)
	test.Assert(t, s.ServiceName == "$GenericService")
}
