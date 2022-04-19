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
	tt := "test"
	out := remote.NewWriterBuffer(256)
	tProto := codecThrift.NewBinaryProtocol(out)
	wInner, rInner := &testWrite{}, &testRead{}

	// Args...
	arg := newGenericServiceCallArgs()
	a, ok := arg.(*Args)
	test.Assert(t, ok == true)
	base := a.GetOrSetBase()
	test.Assert(t, base != nil)
	a.SetCodec(tt)
	// write not ok
	err := a.Write(ctx, nil)
	test.Assert(t, err != nil)
	// write ok
	a.SetCodec(wInner)
	err = a.Write(ctx, tProto)
	test.Assert(t, err == nil)
	// read not ok
	err = a.Read(ctx, tt, nil)
	test.Assert(t, err != nil)
	// read ok
	a.SetCodec(rInner)
	err = a.Read(ctx, tt, tProto)
	test.Assert(t, err == nil)

	// Result...
	result := newGenericServiceCallResult()
	r, ok := result.(*Result)
	test.Assert(t, ok == true)

	// write not ok
	err = r.Write(ctx, nil)
	test.Assert(t, err != nil)
	// write ok
	r.SetCodec(wInner)
	err = r.Write(ctx, tProto)
	test.Assert(t, err == nil)
	// read not ok
	err = r.Read(ctx, tt, nil)
	test.Assert(t, err != nil)
	// read ok
	r.SetCodec(rInner)
	err = r.Read(ctx, tt, tProto)
	test.Assert(t, err == nil)

	r.SetSuccess(nil)
	success := r.GetSuccess()
	test.Assert(t, success == nil)
	r.SetSuccess(tt)
	success = r.GetSuccess()
	test.Assert(t, success.(string) == tt)

	// test callHandler...
	handler := &testImpl{}
	a.Method = ""
	err = callHandler(ctx, handler, arg, result)
	test.Assert(t, err != nil)
	a.Method = tt
	err = callHandler(ctx, handler, arg, result)
	test.Assert(t, err == nil)
}

func TestServiceInfo(t *testing.T) {
	s := ServiceInfo(serviceinfo.Thrift)
	t.Log(s.ServiceName)
}
