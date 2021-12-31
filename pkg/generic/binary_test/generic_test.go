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
	"runtime"
	"runtime/debug"
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

func TestRun(t *testing.T) {
	t.Run("RawThriftBinary", rawThriftBinary)
	t.Run("RawThriftBinaryError", rawThriftBinaryError)
	t.Run("RawThriftBinaryMockReq", rawThriftBinaryMockReq)
	t.Run("RawThriftBinary2NormalServer", rawThriftBinary2NormalServer)
}

func rawThriftBinary(t *testing.T) {
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

func rawThriftBinaryError(t *testing.T) {
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

func rawThriftBinaryMockReq(t *testing.T) {
	svr := initRawThriftBinaryServer(new(GenericServiceMockImpl))
	defer svr.Stop()
	time.Sleep(500 * time.Millisecond)

	cli := initRawThriftBinaryClient()

	req := kt.NewMockReq()
	req.Msg = reqMsg
	strMap := make(map[string]string)
	strMap["aa"] = "aa"
	strMap["bb"] = "bb"
	req.StrMap = strMap
	args := kt.NewMockTestArgs()
	args.Req = req

	// encode
	rc := utils.NewThriftMessageCodec()
	buf, err := rc.Encode("Test", thrift.CALL, 100, args)
	test.Assert(t, err == nil, err)

	resp, err := cli.GenericCall(context.Background(), "Test", buf)
	test.Assert(t, err == nil, err)

	// decode
	buf = resp.([]byte)
	var result kt.MockTestResult
	method, seqID, err := rc.Decode(buf, &result)
	test.Assert(t, err == nil, err)
	test.Assert(t, method == "Test", method)
	test.Assert(t, seqID != 100, seqID)
	test.Assert(t, *result.Success == respMsg)

	seqID2, err2 := generic.GetSeqID(buf)
	test.Assert(t, err2 == nil, err2)
	test.Assert(t, seqID2 == seqID, seqID2)
}

func rawThriftBinary2NormalServer(t *testing.T) {
	svr := initMockServer(new(MockImpl))
	defer svr.Stop()
	time.Sleep(500 * time.Millisecond)

	cli := initRawThriftBinaryClient()

	req := kt.NewMockReq()
	req.Msg = reqMsg
	strMap := make(map[string]string)
	strMap["aa"] = "aa"
	strMap["bb"] = "bb"
	req.StrMap = strMap
	args := kt.NewMockTestArgs()
	args.Req = req

	// encode
	rc := utils.NewThriftMessageCodec()
	buf, err := rc.Encode("Test", thrift.CALL, 100, args)
	test.Assert(t, err == nil, err)

	resp, err := cli.GenericCall(context.Background(), "Test", buf, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)

	// decode
	buf = resp.([]byte)
	var result kt.MockTestResult
	method, seqID, err := rc.Decode(buf, &result)
	test.Assert(t, err == nil, err)
	test.Assert(t, method == "Test", method)
	// seqID会在kitex中覆盖，避免TTHeader和Payload codec 不一致问题
	test.Assert(t, seqID != 100, seqID)
	test.Assert(t, *result.Success == respMsg)
}

func initRawThriftBinaryClient() genericclient.Client {
	g := generic.BinaryThriftGeneric()
	cli := newGenericClient("destServiceName", g, "127.0.0.1:9009")
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
	buf := make([]byte, 12+len(method)+len(reqMsg))
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

func TestBinaryThriftGenericClientClose(t *testing.T) {
	debug.SetGCPercent(-1)
	defer debug.SetGCPercent(100)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	clis := make([]genericclient.Client, 10000)
	for i := 0; i < 10000; i++ {
		g := generic.BinaryThriftGeneric()
		clis[i] = newGenericClient("destServiceName", g, "127.0.0.1:9009")
	}

	runtime.ReadMemStats(&ms)
	preHeepAlloc, preHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", preHeepAlloc, preHeapObjects)

	for _, cli := range clis {
		_ = cli.Close()
	}
	runtime.GC()
	runtime.ReadMemStats(&ms)
	afterGCHeepAlloc, afterGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", afterGCHeepAlloc, afterGCHeapObjects)
	test.Assert(t, afterGCHeepAlloc < preHeepAlloc && afterGCHeapObjects < preHeapObjects)
}

func TestBinaryThriftGenericClientFinalizer(t *testing.T) {
	debug.SetGCPercent(-1)
	defer debug.SetGCPercent(100)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	clis := make([]genericclient.Client, 10000)
	for i := 0; i < 10000; i++ {
		g := generic.BinaryThriftGeneric()
		clis[i] = newGenericClient("destServiceName", g, "127.0.0.1:9009")
	}

	runtime.ReadMemStats(&ms)
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	runtime.GC()
	runtime.ReadMemStats(&ms)
	firstGCHeepAlloc, firstGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", firstGCHeepAlloc, firstGCHeapObjects)

	runtime.GC()
	runtime.ReadMemStats(&ms)
	secondGCHeepAlloc, secondGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", secondGCHeepAlloc, secondGCHeapObjects)
	test.Assert(t, secondGCHeepAlloc < firstGCHeepAlloc && secondGCHeapObjects < firstGCHeapObjects)
}

func mb(byteSize uint64) float32 {
	return float32(byteSize) / float32(1024*1024)
}
