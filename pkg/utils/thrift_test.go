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

package utils

import (
	"errors"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	mt "github.com/jackedelic/kitex/internal/mocks/thrift"
	"github.com/jackedelic/kitex/internal/test"
)

func TestRPCCodec(t *testing.T) {
	rc := NewThriftMessageCodec()

	req1 := mt.NewMockReq()
	req1.Msg = "Hello Kitex"
	var strMap = make(map[string]string)
	strMap["aa"] = "aa"
	strMap["bb"] = "bb"
	req1.StrMap = strMap

	args1 := mt.NewMockTestArgs()
	args1.Req = req1

	// encode
	buf, err := rc.Encode("mockMethod", thrift.CALL, 100, args1)
	test.Assert(t, err == nil, err)

	var argsDecode1 mt.MockTestArgs
	// decode
	method, seqID, err := rc.Decode(buf, &argsDecode1)

	test.Assert(t, err == nil)
	test.Assert(t, method == "mockMethod")
	test.Assert(t, seqID == 100)
	test.Assert(t, argsDecode1.Req.Msg == req1.Msg)
	test.Assert(t, len(argsDecode1.Req.StrMap) == len(req1.StrMap))
	for k := range argsDecode1.Req.StrMap {
		test.Assert(t, argsDecode1.Req.StrMap[k] == req1.StrMap[k])
	}

	// *** reuse ThriftMessageCodec
	req2 := mt.NewMockReq()
	req2.Msg = "Hello Kitex1"
	strMap = make(map[string]string)
	strMap["cc"] = "cc"
	strMap["dd"] = "dd"
	req2.StrMap = strMap
	args2 := mt.NewMockTestArgs()
	args2.Req = req2
	// encode
	buf, err = rc.Encode("mockMethod1", thrift.CALL, 101, args2)
	test.Assert(t, err == nil, err)

	// decode
	var argsDecode2 mt.MockTestArgs
	method, seqID, err = rc.Decode(buf, &argsDecode2)

	test.Assert(t, err == nil, err)
	test.Assert(t, method == "mockMethod1")
	test.Assert(t, seqID == 101)
	test.Assert(t, argsDecode2.Req.Msg == req2.Msg)
	test.Assert(t, len(argsDecode2.Req.StrMap) == len(req2.StrMap))
	for k := range argsDecode2.Req.StrMap {
		test.Assert(t, argsDecode2.Req.StrMap[k] == req2.StrMap[k])
	}
}

func TestSerializer(t *testing.T) {
	rc := NewThriftMessageCodec()

	req := mt.NewMockReq()
	req.Msg = "Hello Kitex"
	var strMap = make(map[string]string)
	strMap["aa"] = "aa"
	strMap["bb"] = "bb"
	req.StrMap = strMap

	args := mt.NewMockTestArgs()
	args.Req = req

	b, err := rc.Serialize(args)
	test.Assert(t, err == nil, err)

	var args2 mt.MockTestArgs
	err = rc.Deserialize(&args2, b)
	test.Assert(t, err == nil, err)

	test.Assert(t, args2.Req.Msg == req.Msg)
	test.Assert(t, len(args2.Req.StrMap) == len(req.StrMap))
	for k := range args2.Req.StrMap {
		test.Assert(t, args2.Req.StrMap[k] == req.StrMap[k])
	}
}

func TestException(t *testing.T) {
	errMsg := "my error"
	b := MarshalError("some method", errors.New(errMsg))
	test.Assert(t, UnmarshalError(b).Error() == errMsg)
}
