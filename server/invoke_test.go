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

package server

import (
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/jackedelic/kitex/internal/mocks"
	"github.com/jackedelic/kitex/internal/pkg/remote/trans/invoke"
	"github.com/jackedelic/kitex/internal/pkg/utils"
	"github.com/jackedelic/kitex/internal/test"
)

func TestInvokerCall(t *testing.T) {
	var opts []Option
	opts = append(opts, WithMetaHandler(noopMetahandler{}))
	invoker := NewInvoker(opts...)

	err := invoker.RegisterService(mocks.ServiceInfo(), mocks.MyServiceHandler())
	if err != nil {
		t.Fatal(err)
	}
	err = invoker.Init()
	if err != nil {
		t.Fatal(err)
	}
	args := mocks.NewMockArgs()
	codec := utils.NewThriftMessageCodec()
	b, _ := codec.Encode("mock", thrift.CALL, 0, args.(thrift.TStruct))

	msg := invoke.NewMessage(nil, nil)
	msg.SetRequestBytes(b)

	err = invoker.Call(msg)
	if err != nil {
		t.Fatal(err)
	}
	b, err = msg.GetResponseBytes()
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, len(b) > 0)
}
