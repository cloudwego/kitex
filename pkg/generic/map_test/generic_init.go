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
	"errors"
	"fmt"
	"net"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/genericclient"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/genericserver"
)

var reqMsg = map[string]interface{}{
	"Msg": "hello",
	"InnerBase": map[string]interface{}{
		"Base": map[string]interface{}{
			"LogID": "log_id_inner",
		},
	},
	"Base": map[string]interface{}{
		"LogID": "log_id",
	},
}

var errResp = "Test Error"

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

// GenericServiceImpl ...
type GenericServiceImpl struct{}

// GenericCall ...
func (g *GenericServiceImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	buf := request.(map[string]interface{})
	rpcinfo := rpcinfo.GetRPCInfo(ctx)
	fmt.Printf("Method from Ctx: %s\n", rpcinfo.Invocation().MethodName())
	fmt.Printf("Recv: %v\n", buf)
	fmt.Printf("Method: %s\n", method)
	return buf, nil
}

// GenericServiceErrorImpl ...
type GenericServiceErrorImpl struct{}

// GenericCall ...
func (g *GenericServiceErrorImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	return response, errors.New(errResp)
}

// GenericServicePingImpl ...
type GenericServicePingImpl struct{}

// GenericCall ...
func (g *GenericServicePingImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	msg := request.(string)
	fmt.Printf("Recv: %v\n", msg)
	return request, nil
}

// GenericServiceOnewayImpl ...
type GenericServiceOnewayImpl struct{}

// GenericCall ...
func (g *GenericServiceOnewayImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	msg := request.(string)
	fmt.Printf("Recv: %v\n", msg)
	return descriptor.Void{}, nil
}

// GenericServiceVoidImpl ...
type GenericServiceVoidImpl struct{}

// GenericCall ...
func (g *GenericServiceVoidImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	msg := request.(string)
	fmt.Printf("Recv: %v\n", msg)
	return descriptor.Void{}, nil
}

var (
	mockReq = map[string]interface{}{
		"Msg": "hello",
		"strMap": map[interface{}]interface{}{
			"mk1": "mv1",
			"mk2": "mv2",
		},
		"strList": []interface{}{
			"lv1", "lv2",
		},
	}
	mockResp = "this is response"
)

// normal server
func newMockServer(handler kt.Mock, addr net.Addr, opts ...server.Option) server.Server {
	var options []server.Option
	opts = append(opts, server.WithServiceAddr(addr))
	options = append(options, opts...)

	svr := server.NewServer(options...)
	if err := svr.RegisterService(serviceInfo(), handler); err != nil {
		panic(err)
	}
	go func() {
		err := svr.Run()
		if err != nil {
			panic(err)
		}
	}()
	return svr
}

func serviceInfo() *serviceinfo.ServiceInfo {
	destService := "Mock"
	handlerType := (*kt.Mock)(nil)
	methods := map[string]serviceinfo.MethodInfo{
		"Test": serviceinfo.NewMethodInfo(testHandler, newMockTestArgs, newMockTestResult, false),
	}
	svcInfo := &serviceinfo.ServiceInfo{
		ServiceName: destService,
		HandlerType: handlerType,
		Methods:     methods,
		Extra:       make(map[string]interface{}),
	}
	return svcInfo
}

func newMockTestArgs() interface{} {
	return kt.NewMockTestArgs()
}

func newMockTestResult() interface{} {
	return kt.NewMockTestResult()
}

func testHandler(ctx context.Context, handler, arg, result interface{}) error {
	realArg := arg.(*kt.MockTestArgs)
	realResult := result.(*kt.MockTestResult)
	success, err := handler.(kt.Mock).Test(ctx, realArg.Req)
	if err != nil {
		return err
	}
	realResult.Success = &success
	return nil
}

type mockImpl struct{}

// Test ...
func (m *mockImpl) Test(ctx context.Context, req *kt.MockReq) (r string, err error) {
	if req.Msg != mockReq["Msg"] {
		return "", fmt.Errorf("msg is not %s", mockReq)
	}
	sm, ok := mockReq["strMap"].(map[interface{}]interface{})
	if !ok {
		return "", fmt.Errorf("strmsg is not map[interface{}]interface{}")
	}
	for k, v := range sm {
		if req.StrMap[k.(string)] != v.(string) {
			return "", fmt.Errorf("strMsg is not %s", req.StrMap)
		}
	}
	sl, ok := mockReq["strList"].([]interface{})
	if !ok {
		return "", fmt.Errorf("strlist is not %s", mockReq["strList"])
	}
	for idx := range sl {
		if sl[idx].(string) != req.StrList[idx] {
			return "", fmt.Errorf("strlist is not %s", mockReq)
		}
	}
	return mockResp, nil
}
