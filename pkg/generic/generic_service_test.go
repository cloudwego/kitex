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

	mocks "github.com/cloudwego/kitex/internal/mocks/generic"
	"github.com/cloudwego/kitex/internal/test"
	gthrift "github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	codecThrift "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestGenericService(t *testing.T) {
	ctx := context.Background()
	method := "test"
	out := remote.NewWriterBuffer(256)
	tProto := codecThrift.NewBinaryProtocol(out)

	argWriteInner, resultWriteInner := mocks.NewMessageWriter(t), mocks.NewMessageWriter(t)
	rInner := mocks.NewMessageReader(t)
	// Read expect
	rInner.On("Read", ctx, method, tProto).Return("test", nil)

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

	// Write expect
	argWriteInner.On("Write", ctx, tProto, a.Request, a.GetOrSetBase()).Return(nil)
	a.SetCodec(argWriteInner)
	// write ok
	err = a.Write(ctx, tProto)
	test.Assert(t, err == nil)
	// read not ok
	err = a.Read(ctx, method, tProto)
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
	err = r.Write(ctx, tProto)
	test.Assert(t, err.Error() == "unexpected Result writer type: <nil>")
	// Write expect
	resultWriteInner.On("Write", ctx, tProto, r.Success, (*gthrift.Base)(nil)).Return(nil)
	r.SetCodec(resultWriteInner)
	// write ok
	err = r.Write(ctx, tProto)
	test.Assert(t, err == nil)
	// read not ok
	err = r.Read(ctx, method, tProto)
	test.Assert(t, strings.Contains(err.Error(), "unexpected Result reader type"))
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

	// handler mock result func
	mockHandlerResultFn := func(srcMethod, expMethod string) (string, error) {
		if srcMethod != expMethod {
			return "", fmt.Errorf("srcMethod is not equal to expMethod: %v", srcMethod)
		}
		return expMethod, nil
	}
	// test callHandler...
	handler := mocks.NewService(t)
	a.Method = ""
	handler.On("GenericCall", ctx, a.Method, a.Request).Return(mockHandlerResultFn(a.Method, method))
	err = callHandler(ctx, handler, arg, result)
	test.Assert(t, strings.Contains(err.Error(), "srcMethod is not equal to expMethod"))
	a.Method = method
	handler.On("GenericCall", ctx, a.Method, a.Request).Return(mockHandlerResultFn(a.Method, method))
	err = callHandler(ctx, handler, arg, result)
	test.Assert(t, err == nil)
}

func TestServiceInfo(t *testing.T) {
	s := ServiceInfo(serviceinfo.Thrift)
	test.Assert(t, s.ServiceName == "$GenericService")
}
