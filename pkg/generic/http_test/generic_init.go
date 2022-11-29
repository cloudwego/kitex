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

// Package test ...
package test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/genericserver"
)

func newGenericClient(destService string, g generic.Generic, targetIPPort string) genericclient.Client {
	var opts []client.Option
	opts = append(opts, client.WithHostPorts(targetIPPort))
	genericCli, _ := genericclient.NewClient(destService, g, opts...)
	return genericCli
}

func newGenericServer(g generic.Generic, addr net.Addr, handler generic.Service) server.Server {
	var opts []server.Option
	opts = append(opts, server.WithServiceAddr(addr))
	svr := genericserver.NewServer(handler, g, opts...)
	go func() {
		err := svr.Run()
		if err != nil {
			panic(err)
		}
	}()
	return svr
}

// GenericServiceReadRequiredFiledImpl ...
type GenericServiceBinaryEchoImpl struct{}

const mockMyMsg = "my msg"

// GenericCall ...
func (g *GenericServiceBinaryEchoImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	req := request.(map[string]interface{})
	gotBase64 := req["got_base64"].(bool)
	fmt.Printf("Recv: (%T)%s\n", req["msg"], req["msg"])
	if !gotBase64 && req["msg"].(string) != mockMyMsg {
		return nil, errors.New("call failed, msg type mismatch")
	}
	if gotBase64 && req["msg"].(string) != base64.StdEncoding.EncodeToString([]byte(mockMyMsg)) {
		return nil, errors.New("call failed, incorrect base64 data")
	}
	num := req["num"].(string)
	if num != "0" {
		return nil, errors.New("call failed, incorrect num")
	}
	return req, nil
}
