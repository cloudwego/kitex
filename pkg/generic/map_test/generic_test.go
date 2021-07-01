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
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/genericclient"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/server"
)

func TestThrift(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9021", new(GenericServiceImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9021")

	resp, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respMap, ok := resp.(map[string]interface{})
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(respMap, respMsg), respMap)
	svr.Stop()
}

func TestThriftPingMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9022", new(GenericServicePingImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9022")

	resp, err := cli.GenericCall(context.Background(), "Ping", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respMap, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(respMap, "hello"), respMap)
	svr.Stop()
}

func TestThriftError(t *testing.T) {
	time.Sleep(2 * time.Second)
	svr := initThriftServer(t, ":9023", new(GenericServiceErrorImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9023")

	_, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errResp), err.Error())
	svr.Stop()
}

func TestThriftOnewayMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9024", new(GenericServiceOnewayImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9024")

	resp, err := cli.GenericCall(context.Background(), "Oneway", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == nil)
	svr.Stop()
}

func TestThriftVoidMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9025", new(GenericServiceVoidImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9025")

	resp, err := cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})
	svr.Stop()
}

func TestThriftReadRequiredField(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9026", new(GenericServiceReadRequiredFiledImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:9026", "./idl/example_check_read_required.thrift")

	_, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil, err)
	test.Assert(t, strings.Contains(err.Error(), "required field (2/required_field) missing"), err.Error())
	svr.Stop()
}

func TestThriftWriteRequiredField(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9027", new(GenericServiceReadRequiredFiledImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:9027", "./idl/example_check_write_required.thrift")

	_, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil, err)
	test.Assert(t, strings.Contains(err.Error(), "required field (2/Foo) missing"), err.Error())
	svr.Stop()
}

func TestThrift2NormalServer(t *testing.T) {
	time.Sleep(4 * time.Second)
	svr := initMockServer(t, new(mockImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftMockClient(t)

	_, err := cli.GenericCall(context.Background(), "Test", mockReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	svr.Stop()
}

func initThriftMockClient(t *testing.T) genericclient.Client {
	p, err := generic.NewThriftFileProvider("./idl/mock.thrift")
	test.Assert(t, err == nil)
	g, err := generic.MapThriftGeneric(p)
	test.Assert(t, err == nil)
	cli := newGenericClient("p.s.m", g, "127.0.0.1:9019")
	test.Assert(t, err == nil)
	return cli
}

func initThriftClient(t *testing.T, addr string) genericclient.Client {
	return initThriftClientByIDL(t, addr, "./idl/example.thrift")
}

func initThriftClientByIDL(t *testing.T, addr, idl string) genericclient.Client {
	p, err := generic.NewThriftFileProvider(idl)
	test.Assert(t, err == nil)
	g, err := generic.MapThriftGeneric(p)
	test.Assert(t, err == nil)
	cli := newGenericClient("p.s.m", g, addr)
	test.Assert(t, err == nil)
	return cli
}

func initThriftServer(t *testing.T, address string, handler generic.Service) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewThriftFileProvider("./idl/example.thrift")
	test.Assert(t, err == nil)
	g, err := generic.MapThriftGeneric(p)
	test.Assert(t, err == nil)
	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func initMockServer(t *testing.T, handler kt.Mock) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", ":9019")
	svr := newMockServer(handler, addr)
	return svr
}
