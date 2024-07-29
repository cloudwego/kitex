/*
 * Copyright 2023 CloudWeGo Authors
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

package thrift

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/protocol/thrift"

	mocks "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	athrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
	"github.com/cloudwego/kitex/pkg/remote"
)

var (
	mockReq = &mocks.MockReq{
		Msg: "hello",
	}
	mockReqThrift = []byte{
		11 /* string */, 0, 1 /* id=1 */, 0, 0, 0, 5 /* length=5 */, 104, 101, 108, 108, 111, /* "hello" */
		13 /* map */, 0, 2 /* id=2 */, 11 /* key:string*/, 11 /* value:string */, 0, 0, 0, 0, /* map size=0 */
		15 /* list */, 0, 3 /* id=3 */, 11 /* item:string */, 0, 0, 0, 0, /* list size=0 */
		0, /* end of struct */
	}
)

func TestMarshalBasicThriftData(t *testing.T) {
	t.Run("invalid-data", func(t *testing.T) {
		err := marshalBasicThriftData(nil, 0)
		test.Assert(t, err == errEncodeMismatchMsgType, err)
	})
	t.Run("valid-data", func(t *testing.T) {
		transport := athrift.NewTMemoryBufferLen(1024)
		tProt := athrift.NewTBinaryProtocol(transport, true, true)
		err := marshalBasicThriftData(tProt, mocks.ToApacheCodec(mockReq))
		test.Assert(t, err == nil, err)
		result := transport.Bytes()
		test.Assert(t, reflect.DeepEqual(result, mockReqThrift), result)
	})
}

func TestMarshalThriftData(t *testing.T) {
	t.Run("NoCodec(=FastCodec)", func(t *testing.T) {
		buf, err := MarshalThriftData(context.Background(), nil, mockReq)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(buf, mockReqThrift), buf)
	})
	t.Run("FastCodec", func(t *testing.T) {
		buf, err := MarshalThriftData(context.Background(), NewThriftCodecWithConfig(FastRead|FastWrite), mockReq)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(buf, mockReqThrift), buf)
	})
	t.Run("BasicCodec", func(t *testing.T) {
		buf, err := MarshalThriftData(context.Background(), NewThriftCodecWithConfig(Basic), mocks.ToApacheCodec(mockReq))
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(buf, mockReqThrift), buf)
	})
	// FrugalCodec: in thrift_frugal_amd64_test.go: TestMarshalThriftDataFrugal
}

func Test_decodeBasicThriftData(t *testing.T) {
	t.Run("empty-input", func(t *testing.T) {
		req := &mocks.MockReq{}
		tProt := NewBinaryProtocol(remote.NewReaderBuffer([]byte{}))
		err := decodeBasicThriftData(tProt, mocks.ToApacheCodec(req))
		test.Assert(t, err != nil, err)
	})
	t.Run("invalid-input", func(t *testing.T) {
		req := &mocks.MockReq{}
		tProt := NewBinaryProtocol(remote.NewReaderBuffer([]byte{0xff}))
		err := decodeBasicThriftData(tProt, mocks.ToApacheCodec(req))
		test.Assert(t, err != nil, err)
	})
	t.Run("normal-input", func(t *testing.T) {
		req := &mocks.MockReq{}
		tProt := NewBinaryProtocol(remote.NewReaderBuffer(mockReqThrift))
		err := decodeBasicThriftData(tProt, mocks.ToApacheCodec(req))
		checkDecodeResult(t, err, req)
	})
}

func checkDecodeResult(t *testing.T, err error, req *mocks.MockReq) {
	test.Assert(t, err == nil, err)
	test.Assert(t, req.Msg == mockReq.Msg, req.Msg, mockReq.Msg)
	test.Assert(t, len(req.StrMap) == 0, req.StrMap)
	test.Assert(t, len(req.StrList) == 0, req.StrList)
}

func TestUnmarshalThriftData(t *testing.T) {
	t.Run("NoCodec(=FastCodec)", func(t *testing.T) {
		req := &mocks.MockReq{}
		err := UnmarshalThriftData(context.Background(), nil, "mock", mockReqThrift, req)
		checkDecodeResult(t, err, req)
	})
	t.Run("FastCodec", func(t *testing.T) {
		req := &mocks.MockReq{}
		err := UnmarshalThriftData(context.Background(), NewThriftCodecWithConfig(FastRead|FastWrite), "mock", mockReqThrift, req)
		checkDecodeResult(t, err, req)
	})
	t.Run("BasicCodec", func(t *testing.T) {
		req := &mocks.MockReq{}
		err := UnmarshalThriftData(context.Background(), NewThriftCodecWithConfig(Basic), "mock", mockReqThrift, mocks.ToApacheCodec(req))
		checkDecodeResult(t, err, req)
	})
	// FrugalCodec: in thrift_frugal_amd64_test.go: TestUnmarshalThriftDataFrugal
}

func TestThriftCodec_unmarshalThriftData(t *testing.T) {
	t.Run("FastCodec with SkipDecoder enabled", func(t *testing.T) {
		req := &mocks.MockReq{}
		codec := &thriftCodec{FastRead | EnableSkipDecoder}
		tProt := NewBinaryProtocol(remote.NewReaderBuffer(mockReqThrift))
		defer tProt.Recycle()
		// specify dataLen with 0 so that skipDecoder works
		err := codec.unmarshalThriftData(tProt, req, 0)
		checkDecodeResult(t, err, &mocks.MockReq{
			Msg:     req.Msg,
			StrList: req.StrList,
			StrMap:  req.StrMap,
		})
	})

	t.Run("FastCodec with SkipDecoder enabled, failed in using SkipDecoder Buffer", func(t *testing.T) {
		req := &mocks.MockReq{}
		codec := &thriftCodec{FastRead | EnableSkipDecoder}
		// these bytes are mapped to
		//  Msg     string            `thrift:"Msg,1" json:"Msg"`
		//	StrMap  map[string]string `thrift:"strMap,2" json:"strMap"`
		//	I16List []int16           `thrift:"I16List,3" json:"i16List"`
		faultMockReqThrift := []byte{
			11 /* string */, 0, 1 /* id=1 */, 0, 0, 0, 5 /* length=5 */, 104, 101, 108, 108, 111, /* "hello" */
			13 /* map */, 0, 2 /* id=2 */, 11 /* key:string*/, 11 /* value:string */, 0, 0, 0, 0, /* map size=0 */
			15 /* list */, 0, 3 /* id=3 */, 6 /* item:I16 */, 0, 0, 0, 1 /* length=1 */, 0, 1, /* I16=1 */
			0, /* end of struct */
		}
		tProt := NewBinaryProtocol(remote.NewReaderBuffer(faultMockReqThrift))
		defer tProt.Recycle()
		// specify dataLen with 0 so that skipDecoder works
		err := codec.unmarshalThriftData(tProt, req, 0)
		test.Assert(t, err != nil, err)
		test.Assert(t, strings.Contains(err.Error(), "caught in FastCodec using SkipDecoder Buffer"))
	})
}

func TestUnmarshalThriftException(t *testing.T) {
	// prepare exception thrift binary
	errMessage := "test: invalid protocol"
	exc := thrift.NewApplicationException(thrift.INVALID_PROTOCOL, errMessage)
	b := make([]byte, exc.BLength())
	n := exc.FastWrite(b)
	test.Assert(t, n == len(b), n)

	// unmarshal
	tProtRead := NewBinaryProtocol(remote.NewReaderBuffer(b))
	err := UnmarshalThriftException(tProtRead)
	transErr, ok := err.(*remote.TransError)
	test.Assert(t, ok, err)
	test.Assert(t, transErr.TypeID() == athrift.INVALID_PROTOCOL, transErr)
	test.Assert(t, transErr.Error() == errMessage, transErr)
}

func Test_getSkippedStructBuffer(t *testing.T) {
	// string length is 6 but only got "hello"
	faultThrift := []byte{
		11 /* string */, 0, 1 /* id=1 */, 0, 0, 0, 6 /* length=6 */, 104, 101, 108, 108, 111, /* "hello" */
	}
	tProt := NewBinaryProtocol(remote.NewReaderBuffer(faultThrift))
	_, err := getSkippedStructBuffer(tProt)
	test.Assert(t, err != nil, err)
	test.Assert(t, strings.Contains(err.Error(), "caught in SkipDecoder Next phase"))
}
