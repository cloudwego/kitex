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

package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cloudwego/thriftgo/generator/golang/extension/unknown"

	tt "github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift/test/kitex_gen/test"
)

var fullReq *test.FullStruct

func init() {
	desc := "aa"
	status := test.HTTPStatus_NOT_FOUND
	byte1 := int8(1)
	double1 := 1.3
	fullReq = &test.FullStruct{
		Left:  32,
		Right: 45,
		Dummy: []byte("test"),
		InnerReq: &test.Inner{
			Num:          6,
			Desc:         &desc,
			MapOfList:    map[int64][]int64{42: {1, 2}},
			MapOfEnumKey: map[test.AEnum]int64{test.AEnum_A: 1, test.AEnum_B: 2},
			Byte1:        &byte1,
			Double1:      &double1,
		},
		Status:   test.HTTPStatus_OK,
		Str:      "str",
		EnumList: []test.HTTPStatus{test.HTTPStatus_NOT_FOUND, test.HTTPStatus_OK},
		Strmap: map[int32]string{
			10: "aa",
			11: "bb",
		},
		Int64:     5,
		IntList:   []int32{11, 22, 33},
		LocalList: []*test.Local{{L: 33}, nil},
		StrLocalMap: map[string]*test.Local{
			"bbb": {
				L: 22,
			},
			"ccc": {
				L: 11,
			},
			"ddd": nil,
		},
		NestList: [][]int32{{3, 4}, {5, 6}},
		RequiredIns: &test.Local{
			L: 55,
		},
		NestMap:  map[string][]string{"aa": {"cc", "bb"}, "bb": {"xx", "yy"}},
		NestMap2: []map[string]test.HTTPStatus{{"ok": test.HTTPStatus_OK}},
		EnumMap: map[int32]test.HTTPStatus{
			0: test.HTTPStatus_NOT_FOUND,
			1: test.HTTPStatus_OK,
		},
		Strlist:   []string{"mm", "nn"},
		OptStatus: &status,
		Complex: map[test.HTTPStatus][]map[string]*test.Local{
			test.HTTPStatus_OK: {
				{"": &test.Local{L: 3}},
				{"c": nil, "d": &test.Local{L: 42}},
				nil,
			},
			test.HTTPStatus_NOT_FOUND: nil,
		},
	}
}

func TestOnlyUnknownField(t *testing.T) {
	l := fullReq.BLength()
	buf := make([]byte, l)
	ll := fullReq.FastWriteNocopy(buf, nil)
	tt.Assert(t, ll == l)

	unknown := &test.EmptyStruct{}
	ll, err := unknown.FastRead(buf)
	tt.Assert(t, err == nil)
	tt.Assert(t, ll == l)
	unknownL := unknown.BLength()
	tt.Assert(t, unknownL == l)
	unknownBuf := make([]byte, unknownL)
	writeL := unknown.FastWriteNocopy(unknownBuf, nil)
	tt.Assert(t, writeL == l)
	tt.Assert(t, bytes.Equal(buf, unknownBuf))
}

func TestPartialUnknownField(t *testing.T) {
	l := fullReq.BLength()
	buf := make([]byte, l)
	ll := fullReq.FastWriteNocopy(buf, nil)
	tt.Assert(t, ll == l)
	compare := &test.FullStruct{}
	ll, err := compare.FastRead(buf)
	tt.Assert(t, err == nil)
	tt.Assert(t, ll == l)

	unknown := &test.MixedStruct{}
	ll, err = unknown.FastRead(buf)
	tt.Assert(t, err == nil)
	tt.Assert(t, ll == l)
	tt.Assert(t, len(unknown.GetUnknown()) == 12)
	unknownL := unknown.BLength()
	unknownBuf := make([]byte, unknownL)
	writeL := unknown.FastWriteNocopy(unknownBuf, nil)
	tt.Assert(t, writeL == unknownL)
	compare1 := &test.FullStruct{}
	ll, err = compare1.FastRead(unknownBuf)
	tt.Assert(t, err == nil)
	tt.Assert(t, ll == unknownL)
	tt.Assert(t, compare1.DeepEqual(compare))
}

func TestNoUnknownField(t *testing.T) {
	l := fullReq.BLength()
	buf := make([]byte, l)
	ll := fullReq.FastWriteNocopy(buf, nil)
	tt.Assert(t, ll == l)

	ori := &test.FullStruct{}
	ll, err := ori.FastRead(buf)
	tt.Assert(t, err == nil)
	tt.Assert(t, ll == l)

	// required fields
	tt.Assert(t, ori.Field11DeepEqual([]*test.Local{{L: 33}, test.NewLocal()}))
	tt.Assert(t, ori.Field12DeepEqual(map[string]*test.Local{
		"bbb": {L: 22}, "ccc": {L: 11}, "ddd": {},
	}))
	tt.Assert(t, ori.Field21DeepEqual(test.NewInner()))
	tt.Assert(t, ori.Field28DeepEqual(map[test.HTTPStatus][]map[string]*test.Local{
		test.HTTPStatus_OK: {
			{"": &test.Local{L: 3}},
			{"c": {}, "d": &test.Local{L: 42}},
			nil,
		},
		test.HTTPStatus_NOT_FOUND: nil,
	}))
	ori.LocalList[1] = nil
	ori.StrLocalMap["ddd"] = nil
	ori.AnotherInner = nil
	ori.Complex[test.HTTPStatus_OK][1]["c"] = nil

	tt.Assert(t, ori.Field1DeepEqual(fullReq.Left))
	tt.Assert(t, ori.Field2DeepEqual(fullReq.Right))
	tt.Assert(t, ori.Field3DeepEqual(fullReq.Dummy))
	tt.Assert(t, ori.Field4DeepEqual(fullReq.InnerReq))
	tt.Assert(t, ori.Field5DeepEqual(fullReq.Status))
	tt.Assert(t, ori.Field6DeepEqual(fullReq.Str))
	tt.Assert(t, ori.Field7DeepEqual(fullReq.EnumList))
	tt.Assert(t, ori.Field8DeepEqual(fullReq.Strmap))
	tt.Assert(t, ori.Field9DeepEqual(fullReq.Int64))
	tt.Assert(t, ori.Field10DeepEqual(fullReq.IntList))
	tt.Assert(t, ori.Field11DeepEqual(fullReq.LocalList))
	tt.Assert(t, ori.Field12DeepEqual(fullReq.StrLocalMap))
	tt.Assert(t, ori.Field13DeepEqual(fullReq.NestList))
	tt.Assert(t, ori.Field14DeepEqual(fullReq.RequiredIns))
	tt.Assert(t, ori.Field16DeepEqual(fullReq.NestMap))
	tt.Assert(t, ori.Field17DeepEqual(fullReq.NestMap2))
	tt.Assert(t, ori.Field18DeepEqual(fullReq.EnumMap))
	tt.Assert(t, ori.Field19DeepEqual(fullReq.Strlist))
	tt.Assert(t, ori.Field20DeepEqual(fullReq.OptionalIns))
	tt.Assert(t, ori.Field21DeepEqual(fullReq.AnotherInner))
	tt.Assert(t, ori.Field22DeepEqual(fullReq.OptNilList))
	tt.Assert(t, ori.Field23DeepEqual(fullReq.NilList))
	tt.Assert(t, ori.Field24DeepEqual(fullReq.OptNilInsList))
	tt.Assert(t, ori.Field25DeepEqual(fullReq.NilInsList))
	tt.Assert(t, ori.Field26DeepEqual(fullReq.OptStatus))
	tt.Assert(t, ori.Field27DeepEqual(fullReq.EnumKeyMap))
	tt.Assert(t, ori.Field28DeepEqual(fullReq.Complex))
}

func TestCorruptWrite(t *testing.T) {
	local := &test.Local{L: 3}
	ufs := unknown.Fields{&unknown.Field{Type: 1000}}
	local.SetUnknown(ufs)

	defer func() {
		e := recover()
		if strings.Contains(e.(error).Error(), "unknown data type 1000") {
			return
		}
		tt.Assert(t, false, e)
	}()
	_ = local.BLength()
	tt.Assert(t, false)
}

func TestCorruptRead(t *testing.T) {
	local := &test.Local{L: 3}
	ufs := unknown.Fields{&unknown.Field{Name: "test", Type: unknown.TString, Value: "str"}}
	local.SetUnknown(ufs)
	l := local.BLength()
	buf := make([]byte, l)
	ll := local.FastWriteNocopy(buf, nil)
	tt.Assert(t, ll == l)
	buf[7] = 200

	var local2 test.Local
	_, err := local2.FastRead(buf)
	tt.Assert(t, err != nil)
	tt.Assert(t, strings.Contains(err.Error(), "unknown data type 200"))
}
