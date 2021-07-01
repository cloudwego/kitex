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
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/genericclient"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/server"
)

func TestRawThriftBinary(t *testing.T) {
	svr := initRawThriftBinaryServer(new(GenericServiceImpl))
	defer svr.Stop()
	time.Sleep(500 * time.Millisecond)

	cli := initRawThriftBinaryClient()

	method := "myMethod"
	buf := genBinaryReqBuf(method)

	resp, err := cli.GenericCall(context.Background(), method, buf, callopt.WithRPCTimeout(1*time.Second))
	test.Assert(t, err == nil, err)
	respBuf, ok := resp.([]byte)
	test.Assert(t, ok)
	test.Assert(t, string(respBuf[12+len(method):]) == respMsg)
}

func TestRawThriftBinaryError(t *testing.T) {
	time.Sleep(2 * time.Second)
	svr := initRawThriftBinaryServer(new(GenericServiceErrorImpl))
	defer svr.Stop()
	time.Sleep(500 * time.Millisecond)

	cli := initRawThriftBinaryClient()

	method := "myMethod"
	buf := genBinaryReqBuf(method)

	_, err := cli.GenericCall(context.Background(), method, buf, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errResp), err.Error())
}

func TestRawThriftBinaryMockReq(t *testing.T) {
	time.Sleep(3 * time.Second)
	svr := initRawThriftBinaryServer(new(GenericServiceMockImpl))
	defer svr.Stop()
	time.Sleep(500 * time.Millisecond)

	cli := initRawThriftBinaryClient()

	req := kt.NewMockReq()
	req.Msg = reqMsg
	var strMap = make(map[string]string)
	strMap["aa"] = "aa"
	strMap["bb"] = "bb"
	req.StrMap = strMap
	args := kt.NewMockTestArgs()
	args.Req = req

	// encode
	rc := utils.NewThriftMessageCodec()
	buf, err := rc.Encode("Test", thrift.CALL, 100, args)

	resp, err := cli.GenericCall(context.Background(), "Test", buf)
	test.Assert(t, err == nil, err)

	// decode
	buf = resp.([]byte)
	var result kt.MockTestResult
	method, seqID, err := rc.Decode(buf, &result)
	test.Assert(t, method == "Test", method)
	test.Assert(t, seqID != 100, seqID)
	test.Assert(t, *result.Success == respMsg)
}

func TestRawThriftBinary2NormalServer(t *testing.T) {
	time.Sleep(4 * time.Second)
	svr := initMockServer(new(MockImpl))
	defer svr.Stop()
	time.Sleep(500 * time.Millisecond)

	cli := initRawThriftBinaryClient()

	req := kt.NewMockReq()
	req.Msg = reqMsg
	var strMap = make(map[string]string)
	strMap["aa"] = "aa"
	strMap["bb"] = "bb"
	req.StrMap = strMap
	args := kt.NewMockTestArgs()
	args.Req = req

	// encode
	rc := utils.NewThriftMessageCodec()
	buf, err := rc.Encode("Test", thrift.CALL, 100, args)

	resp, err := cli.GenericCall(context.Background(), "Test", buf, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)

	// decode
	buf = resp.([]byte)
	var result kt.MockTestResult
	method, seqID, err := rc.Decode(buf, &result)
	test.Assert(t, method == "Test", method)
	// seqID会在kitex中覆盖，避免TTHeader和Payload codec 不一致问题
	test.Assert(t, seqID != 100, seqID)
	test.Assert(t, *result.Success == respMsg)
}

func initRawThriftBinaryClient() genericclient.Client {
	g := generic.BinaryThriftGeneric()
	cli := newGenericClient("p.s.m", g, "127.0.0.1:9009")
	return cli
}

func initRawThriftBinaryServer(handler generic.Service) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", ":9009")
	g := generic.BinaryThriftGeneric()
	svr := newGenericServer(g, addr, handler)
	return svr
}

func initMockServer(handler kt.Mock) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", ":9009")
	svr := NewMockServer(handler, addr)
	return svr
}

func genBinaryReqBuf(method string) []byte {
	idx := 0
	var buf = make([]byte, 12+len(method)+len(reqMsg))
	binary.BigEndian.PutUint32(buf, thrift.VERSION_1)
	idx += 4
	binary.BigEndian.PutUint32(buf[idx:idx+4], uint32(len(method)))
	idx += 4
	copy(buf[idx:idx+len(method)], method)
	idx += len(method)
	binary.BigEndian.PutUint32(buf[idx:idx+4], 100)
	idx += 4
	copy(buf[idx:idx+len(reqMsg)], reqMsg)
	return buf
}
