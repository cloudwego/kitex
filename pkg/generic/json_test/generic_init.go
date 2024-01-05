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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/genericclient"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/genericserver"
	"github.com/cloudwego/kitex/transport"
)

var reqMsg = `{"Msg":"hello","InnerBase":{"Base":{"LogID":"log_id_inner"}},"Base":{"LogID":"log_id"}}`

var reqRegression = `{"Msg":"hello","InnerBase":{"Base":{"LogID":"log_id_inner"}},"Base":{"LogID":"log_id"},"I8":"8","I16":"16","I32":"32","I64":"64","Double":"12.3"}`

var respMsgWithExtra = `{"Msg":"world","required_field":"required_field","extra_field":"extra_field"}`

var reqExtendMsg = `{"Msg":123}`

var errResp = "Test Error"

type Simple struct {
	ByteField   int8    `thrift:"ByteField,1" json:"ByteField"`
	I64Field    int64   `thrift:"I64Field,2" json:"I64Field"`
	DoubleField float64 `thrift:"DoubleField,3" json:"DoubleField"`
	I32Field    int32   `thrift:"I32Field,4" json:"I32Field"`
	StringField string  `thrift:"StringField,5" json:"StringField"`
	BinaryField []byte  `thrift:"BinaryField,6" json:"BinaryField"`
}

type Nesting struct {
	String_         string             `thrift:"String,1" json:"String"`
	ListSimple      []*Simple          `thrift:"ListSimple,2" json:"ListSimple"`
	Double          float64            `thrift:"Double,3" json:"Double"`
	I32             int32              `thrift:"I32,4" json:"I32"`
	ListI32         []int32            `thrift:"ListI32,5" json:"ListI32"`
	I64             int64              `thrift:"I64,6" json:"I64"`
	MapStringString map[string]string  `thrift:"MapStringString,7" json:"MapStringString"`
	SimpleStruct    *Simple            `thrift:"SimpleStruct,8" json:"SimpleStruct"`
	MapI32I64       map[int32]int64    `thrift:"MapI32I64,9" json:"MapI32I64"`
	ListString      []string           `thrift:"ListString,10" json:"ListString"`
	Binary          []byte             `thrift:"Binary,11" json:"Binary"`
	MapI64String    map[int64]string   `thrift:"MapI64String,12" json:"MapI64String"`
	ListI64         []int64            `thrift:"ListI64,13" json:"ListI64"`
	Byte            int8               `thrift:"Byte,14" json:"Byte"`
	MapStringSimple map[string]*Simple `thrift:"MapStringSimple,15" json:"MapStringSimple"`
}

func getString() string {
	return strings.Repeat("你好,\b\n\r\t世界", 2)
}

func getBytes() []byte {
	return bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, 2)
}

func getSimpleValue() *Simple {
	return &Simple{
		ByteField:   math.MaxInt8,
		I64Field:    math.MaxInt64,
		DoubleField: math.MaxFloat64,
		I32Field:    math.MaxInt32,
		StringField: getString(),
		BinaryField: getBytes(),
	}
}

func getNestingValue() *Nesting {
	ret := &Nesting{
		String_:         getString(),
		ListSimple:      []*Simple{},
		Double:          math.MaxFloat64,
		I32:             math.MaxInt32,
		ListI32:         []int32{},
		I64:             math.MaxInt64,
		MapStringString: map[string]string{},
		SimpleStruct:    getSimpleValue(),
		MapI32I64:       map[int32]int64{},
		ListString:      []string{},
		Binary:          getBytes(),
		MapI64String:    map[int64]string{},
		ListI64:         []int64{},
		Byte:            math.MaxInt8,
		MapStringSimple: map[string]*Simple{},
	}

	for i := 0; i < 16; i++ {
		ret.ListSimple = append(ret.ListSimple, getSimpleValue())
		ret.ListI32 = append(ret.ListI32, math.MinInt32)
		ret.ListI64 = append(ret.ListI64, math.MinInt64)
		ret.ListString = append(ret.ListString, getString())
	}

	for i := 0; i < 16; i++ {
		ret.MapStringString[strconv.Itoa(i)] = getString()
		ret.MapI32I64[int32(i)] = math.MinInt64
		ret.MapI64String[int64(i)] = getString()
		ret.MapStringSimple[strconv.Itoa(i)] = getSimpleValue()
	}

	return ret
}

func newGenericClient(tp transport.Protocol, destService string, g generic.Generic, targetIPPort string) genericclient.Client {
	var opts []client.Option
	opts = append(opts, client.WithHostPorts(targetIPPort), client.WithTransportProtocol(tp))
	genericCli, _ := genericclient.NewClient(destService, g, opts...)
	return genericCli
}

func newGenericServer(g generic.Generic, addr net.Addr, handler generic.Service) server.Server {
	var opts []server.Option
	opts = append(opts, server.WithServiceAddr(addr), server.WithExitWaitTime(time.Microsecond*10))
	svr := genericserver.NewServer(handler, g, opts...)
	go func() {
		err := svr.Run()
		if err != nil {
			panic(err)
		}
	}()
	test.WaitServerStart(addr.String())
	return svr
}

// GenericServiceImpl ...
type GenericServiceImpl struct{}

// GenericCall ...
func (g *GenericServiceImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	if method == "ExtendMethod" {
		return request, nil
	}
	buf := request.(string)
	rpcinfo := rpcinfo.GetRPCInfo(ctx)
	fmt.Printf("Method from Ctx: %s\n", rpcinfo.Invocation().MethodName())
	fmt.Printf("Recv: %v\n", buf)
	fmt.Printf("Method: %s\n", method)
	return respMsgWithExtra, nil
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

// GenericServiceVoidWithStringImpl ...
type GenericServiceVoidWithStringImpl struct{}

// GenericCall ...
func (g *GenericServiceVoidWithStringImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	msg := request.(string)
	fmt.Printf("Recv: %v\n", msg)
	return "Void", nil
}

// GenericServiceBinaryEchoImpl ...
type GenericServiceBinaryEchoImpl struct{}

const mockMyMsg = "my msg"

type BinaryEcho struct {
	Msg       string `json:"msg"`
	GotBase64 bool   `json:"got_base64"`
}

// GenericCall ...
func (g *GenericServiceBinaryEchoImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	req := &BinaryEcho{}
	json.Unmarshal([]byte(request.(string)), req)
	fmt.Printf("Recv: %s\n", req.Msg)
	if !req.GotBase64 && req.Msg != mockMyMsg {
		return nil, errors.New("call failed, msg type mismatch")
	}
	if req.GotBase64 && req.Msg != base64.StdEncoding.EncodeToString([]byte(mockMyMsg)) {
		return nil, errors.New("call failed, incorrect base64 data")
	}
	return request, nil
}

// GenericServiceImpl ...
type GenericServiceBenchmarkImpl struct{}

// GenericCall ...
func (g *GenericServiceBenchmarkImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	return request, nil
}

var (
	mockReq  = `{"Msg":"hello","strMap":{"mk1":"mv1","mk2":"mv2"},"strList":["lv1","lv2"]} `
	mockResp = "this is response"
)

// normal server
func newMockServer(handler kt.Mock, addr net.Addr, opts ...server.Option) server.Server {
	var options []server.Option
	opts = append(opts, server.WithServiceAddr(addr), server.WithExitWaitTime(time.Microsecond*10))
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

//type MockImpl struct{}
//
//func (g *MockImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
//	req := &kt.MockReq{}
//	json.Unmarshal([]byte(request.(string)), req)
//	fmt.Printf("Recv: %s\n", req.Msg)
//	return mockResp, nil
//}

func serviceInfo() *serviceinfo.ServiceInfo {
	destService := "Mock"
	handlerType := (*kt.Mock)(nil)
	methods := map[string]serviceinfo.MethodInfo{
		"Test":          serviceinfo.NewMethodInfo(testHandler, newMockTestArgs, newMockTestResult, false),
		"ExceptionTest": serviceinfo.NewMethodInfo(exceptionHandler, newMockExceptionTestArgs, newMockExceptionTestResult, false),
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

func newMockExceptionTestArgs() interface{} {
	return kt.NewMockExceptionTestArgs()
}

func newMockExceptionTestResult() interface{} {
	return &kt.MockExceptionTestResult{}
}

func exceptionHandler(ctx context.Context, handler, args, result interface{}) error {
	a := args.(*kt.MockExceptionTestArgs)
	r := result.(*kt.MockExceptionTestResult)
	reply, err := handler.(kt.Mock).ExceptionTest(ctx, a.Req)
	if err != nil {
		switch v := err.(type) {
		case *kt.Exception:
			r.Err = v
		default:
			return err
		}
	} else {
		r.Success = &reply
	}
	return nil
}

type mockImpl struct{}

// Test ...
func (m *mockImpl) Test(ctx context.Context, req *kt.MockReq) (r string, err error) {
	msg := gjson.Get(mockReq, "Msg")
	if req.Msg != msg.String() {
		return "", fmt.Errorf("msg is not %s", mockReq)
	}
	strMap := gjson.Get(mockReq, "strMap")
	if len(strMap.Map()) == 0 {
		return "", fmt.Errorf("strmsg is not map[interface{}]interface{}")
	}
	for k, v := range strMap.Map() {
		if req.StrMap[k] != v.String() {
			return "", fmt.Errorf("strMsg is not %s", req.StrMap)
		}
	}

	strList := gjson.Get(mockReq, "strList")
	array := strList.Array()
	if len(array) == 0 {
		return "", fmt.Errorf("strlist is not %v", strList)
	}
	for idx := range array {
		if array[idx].Value() != req.StrList[idx] {
			return "", fmt.Errorf("strlist is not %s", mockReq)
		}
	}
	return mockResp, nil
}

// ExceptionTest ...
func (m *mockImpl) ExceptionTest(ctx context.Context, req *kt.MockReq) (r string, err error) {
	msg := gjson.Get(mockReq, "Msg")
	if req.Msg != msg.String() {
		return "", fmt.Errorf("msg is not %s", mockReq)
	}
	strMap := gjson.Get(mockReq, "strMap")
	if len(strMap.Map()) == 0 {
		return "", fmt.Errorf("strmsg is not map[interface{}]interface{}")
	}
	for k, v := range strMap.Map() {
		if req.StrMap[k] != v.String() {
			return "", fmt.Errorf("strMsg is not %s", req.StrMap)
		}
	}

	strList := gjson.Get(mockReq, "strList")
	array := strList.Array()
	if len(array) == 0 {
		return "", fmt.Errorf("strlist is not %v", strList)
	}
	for idx := range array {
		if array[idx].Value() != req.StrList[idx] {
			return "", fmt.Errorf("strlist is not %s", mockReq)
		}
	}
	return "", &kt.Exception{Code: 400, Msg: "this is an exception"}
}

// GenericRegressionImpl ...
type GenericRegressionImpl struct{}

// GenericCall ...
func (g *GenericRegressionImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	buf := request.(string)
	rpcinfo := rpcinfo.GetRPCInfo(ctx)
	fmt.Printf("Method from Ctx: %s\n", rpcinfo.Invocation().MethodName())
	fmt.Printf("Recv: %v\n", buf)
	fmt.Printf("Method: %s\n", method)
	respMsg := `{"Msg":"world","required_field":"required_field","num":64,"I8":"8","I16":"16","I32":"32","I64":"64","Double":"12.3"}`
	return respMsg, nil
}
