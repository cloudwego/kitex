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

package genericclient

import (
	"context"
	dproto "github.com/cloudwego/dynamicgo/proto"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/transport"
	"testing"
)

func TestStreamingClient(t *testing.T) {
	test.PanicAt(t, func() {
		newStreamingClient("destService", generic.BinaryThriftGeneric())
	}, func(err interface{}) bool {
		return err.(string) == "binary generic streaming is not supported"
	})

	g, err := getJSONPbGeneric()
	test.Assert(t, err == nil)
	_, err = newStreamingClient("destService", g, client.WithTransportProtocol(transport.GRPC), client.WithHostPorts(test.GetLocalAddress()))
	test.Assert(t, err == nil)
}

func TestClientStreaming(t *testing.T) {
	g, err := getJSONPbGeneric()
	test.Assert(t, err == nil)
	sCli, err := newStreamingClient("destService", g, client.WithTransportProtocol(transport.GRPC), client.WithHostPorts(test.GetLocalAddress()))
	test.Assert(t, err == nil)
	_, err = newClientStreaming(context.Background(), sCli, "ClientStreamingTest")
	test.Assert(t, err != nil)
}

func TestServerStreaming(t *testing.T) {
	g, err := getJSONPbGeneric()
	test.Assert(t, err == nil)
	sCli, err := newStreamingClient("destService", g, client.WithTransportProtocol(transport.GRPC), client.WithHostPorts(test.GetLocalAddress()))
	test.Assert(t, err == nil)
	_, err = newServerStreaming(context.Background(), sCli, "ServerStreamingTest", `{"Msg": "message"}`)
	test.Assert(t, err != nil)
}

func TestBidirectionalStreaming(t *testing.T) {
	g, err := getJSONPbGeneric()
	test.Assert(t, err == nil)
	sCli, err := newStreamingClient("destService", g, client.WithTransportProtocol(transport.GRPC), client.WithHostPorts(test.GetLocalAddress()))
	test.Assert(t, err == nil)
	_, err = newBidirectionalStreaming(context.Background(), sCli, "BidirectionalStreamingTest")
	test.Assert(t, err != nil)
}

func getJSONPbGeneric() (generic.Generic, error) {
	dOpts := dproto.Options{}
	p, err := generic.NewPbFileProviderWithDynamicGo("../../pkg/generic/grpcjsonpb_test/idl/pbapi.proto", context.Background(), dOpts)
	if err != nil {
		return nil, err
	}
	return generic.JSONPbGeneric(p)
}
