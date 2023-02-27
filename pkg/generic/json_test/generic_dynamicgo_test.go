//go:build amd64 && go1.16
// +build amd64,go1.16

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

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/cloudwego/dynamicgo/conv"
	"github.com/tidwall/gjson"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/transport"
)

func initDynamicgoThriftClient(tp transport.Protocol, t *testing.T, addr, idl string, opts generic.DynamicgoOptions) genericclient.Client {
	p, err := generic.NewThriftFileProvider(idl)
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p, opts)
	test.Assert(t, err == nil)
	cli := newGenericDynamicgoClient(tp, "destServiceName", g, addr)
	test.Assert(t, err == nil)
	return cli
}

func initDynamicgoThriftServer(t *testing.T, address string, handler generic.Service, idlPath string, opts generic.DynamicgoOptions) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewThriftFileProvider(idlPath)
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p, opts)
	test.Assert(t, err == nil)
	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func initOriginalThriftClient(t *testing.T, addr, idl string, base64binary *bool) genericclient.Client {
	p, err := generic.NewThriftFileProvider(idl)
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	if base64binary != nil {
		generic.SetBinaryWithBase64(g, *base64binary)
	}
	cli := newGenericClient("destServiceName", g, addr)
	test.Assert(t, err == nil)
	return cli
}

func initOriginalThriftServer(t *testing.T, address string, handler generic.Service, idlPath string, base64Binary *bool) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewThriftFileProvider(idlPath)
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	if base64Binary != nil {
		generic.SetBinaryWithBase64(g, *base64Binary)
	}
	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func TestDynamicgoThrift(t *testing.T) {
	dOpts := generic.DynamicgoOptions{}
	time.Sleep(1 * time.Second)
	svr := initDynamicgoThriftServer(t, ":8128", new(GenericServiceImpl), "./idl/example.thrift", dOpts)
	time.Sleep(500 * time.Millisecond)

	cli := initDynamicgoThriftClient(transport.TTHeader, t, "127.0.0.1:8128", "./idl/example.thrift", dOpts)

	resp, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(respStr, "Msg").String(), "world"), "world")

	cli = initDynamicgoThriftClient(transport.PurePayload, t, "127.0.0.1:8128", "./idl/example.thrift", dOpts)
	resp, err = cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok = resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(respStr, "Msg").String(), "world"), "world")

	svr.Stop()
}

func BenchmarkCompareKitexAndDynamicgo_Small(b *testing.B) {
	// small data
	sobj := GetSimpleValue()
	sout, err := json.Marshal(sobj)
	if err != nil {
		panic(err)
	}
	simpleJSON := string(sout)
	fmt.Println("small data size: ", len(simpleJSON))

	t := testing.T{}
	b.Run("thrift_small", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initOriginalThriftServer(&t, ":8121", new(GenericServiceBenchmarkImpl), "./idl/baseline.thrift", nil)
		time.Sleep(500 * time.Millisecond)
		cli := initOriginalThriftClient(&t, "127.0.0.1:8121", "./idl/baseline.thrift", nil)

		resp, err := cli.GenericCall(context.Background(), "SimpleMethod", simpleJSON, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		respStr, ok := resp.(string)
		test.Assert(&t, ok)
		test.Assert(&t, gjson.Get(respStr, "I64Field").Int() == math.MaxInt64, math.MaxInt64)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "SimpleMethod", simpleJSON, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})

	b.Run("dynamicgoThrift_small", func(b *testing.B) {
		convOpts := conv.Options{NoBase64Binary: true, EnableValueMapping: true}
		dOpts := generic.DynamicgoOptions{ConvOpts: convOpts}
		time.Sleep(1 * time.Second)
		svr := initDynamicgoThriftServer(&t, ":8128", new(GenericServiceBenchmarkImpl), "./idl/baseline.thrift", dOpts)
		time.Sleep(500 * time.Millisecond)
		cli := initDynamicgoThriftClient(transport.TTHeader, &t, "127.0.0.1:8128", "./idl/baseline.thrift", dOpts)

		resp, err := cli.GenericCall(context.Background(), "SimpleMethod", simpleJSON, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		respStr, ok := resp.(string)
		test.Assert(&t, ok)
		test.Assert(&t, gjson.Get(respStr, "I64Field").Int() == math.MaxInt64, math.MaxInt64)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "SimpleMethod", simpleJSON, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})
}

func BenchmarkCompareKitexAndDynamicgo_Medium(b *testing.B) {
	// medium data
	nobj := GetNestingValue()
	nout, err := json.Marshal(nobj)
	if err != nil {
		panic(err)
	}
	nestingJSON := string(nout)
	fmt.Println("medium data size: ", len(nestingJSON))

	t := testing.T{}

	b.Run("thrift_medium", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initOriginalThriftServer(&t, ":8121", new(GenericServiceBenchmarkImpl), "./idl/baseline.thrift", nil)
		time.Sleep(500 * time.Millisecond)
		cli := initOriginalThriftClient(&t, "127.0.0.1:8121", "./idl/baseline.thrift", nil)
		resp, err := cli.GenericCall(context.Background(), "NestingMethod", nestingJSON, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		respStr, ok := resp.(string)
		test.Assert(&t, ok)
		test.Assert(&t, gjson.Get(respStr, "Double").Float() == math.MaxFloat64, math.MaxFloat64)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "NestingMethod", nestingJSON, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})

	b.Run("dynamicgoThrift_medium", func(b *testing.B) {
		convOpts := conv.Options{NoBase64Binary: true}
		dOpts := generic.DynamicgoOptions{ConvOpts: convOpts}
		time.Sleep(1 * time.Second)
		svr := initDynamicgoThriftServer(&t, ":8128", new(GenericServiceBenchmarkImpl), "./idl/baseline.thrift", dOpts)
		time.Sleep(500 * time.Millisecond)
		cli := initDynamicgoThriftClient(transport.TTHeader, &t, "127.0.0.1:8128", "./idl/baseline.thrift", dOpts)

		resp, err := cli.GenericCall(context.Background(), "NestingMethod", nestingJSON, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		respStr, ok := resp.(string)
		test.Assert(&t, ok)
		test.Assert(&t, gjson.Get(respStr, "Double").Float() == math.MaxFloat64, math.MaxFloat64)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "NestingMethod", nestingJSON, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})
}
