/*
 * Copyright 2024 CloudWeGo Authors
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

package ttstream

import (
	"context"
	"sync"
	"testing"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/server/streamxserver"
	"github.com/cloudwego/netpoll"
)

type testRequest struct {
	Message string
}

func (t *testRequest) BLength() int {
	return len(t.Message)
}

func (t *testRequest) FastWriteNocopy(buf []byte, bw thrift.NocopyWriter) int {
	copy(buf, t.Message)
	return len(t.Message)
}

func (t *testRequest) FastRead(buf []byte) (int, error) {
	t.Message = string(buf)
	return len(buf), nil
}

type testResponse = testRequest

var _ thrift.FastCodec = (*testRequest)(nil)

var testServiceInfo = &serviceinfo.ServiceInfo{
	ServiceName: "kitex.service.streaming",
	Methods: map[string]serviceinfo.MethodInfo{
		"Bidi": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return streamxserver.InvokeStream[testRequest, testResponse](
					ctx, serviceinfo.StreamingBidirectional, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
		),
	},
}

func TestTransport(t *testing.T) {
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)

	ctrans := newTransport(clientTransport, testServiceInfo, cconn)
	ss, err := ctrans.newStream(context.Background(), "Bidi", make(IntHeader), make(streamx.Header))
	test.Assert(t, err == nil, err)
	strans := newTransport(serverTransport, testServiceInfo, sconn)
	cs, err := strans.readStream(context.Background())
	test.Assert(t, err == nil, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := new(testRequest)
		req.Message = "hello"
		err = cs.SendMsg(context.Background(), req)
		test.Assert(t, err == nil, err)
	}()

	req := new(testRequest)
	err = ss.RecvMsg(context.Background(), req)
	test.Assert(t, err == nil, err)
	t.Logf("req: %v", req)
	wg.Wait()
}
