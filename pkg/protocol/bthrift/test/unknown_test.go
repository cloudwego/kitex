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
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/pkg/protocol/bthrift/test/kitex_gen/test"
	"github.com/cloudwego/thriftgo/generator/golang/extension/unknown"
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
	if ll := fullReq.FastWriteNocopy(buf, nil); ll != l {
		t.Fatal(ll)
	}

	unknown := &test.EmptyStruct{}
	ll, err := unknown.FastRead(buf)
	if err != nil {
		t.Fatal(err)
	}
	if ll != l {
		t.Fatal(ll)
	}
	unknownL := unknown.BLength()
	if unknownL != l {
		t.Fatal(unknownL)
	}
	unknownBuf := make([]byte, unknownL)
	writeL := unknown.FastWriteNocopy(unknownBuf, nil)
	if writeL != l {
		t.Fatal(writeL)
	}

	if !bytes.Equal(buf, unknownBuf) {
		t.Fatal()
	}
}

func TestPartialUnknownField(t *testing.T) {
	l := fullReq.BLength()
	buf := make([]byte, l)
	if ll := fullReq.FastWriteNocopy(buf, nil); ll != l {
		t.Fatal(ll)
	}
	compare := &test.FullStruct{}
	if ll, err := compare.FastRead(buf); err != nil {
		t.Fatal(err)
	} else if ll != l {
		t.Fatal(ll)
	}

	unknown := &test.MixedStruct{}
	ll, err := unknown.FastRead(buf)
	if err != nil {
		t.Fatal(err)
	}
	if ll != l {
		t.Fatal(ll)
	}
	fmt.Printf("unknown: %v\n", unknownStr(unknown.GetUnknown()))
	if len(unknown.GetUnknown()) != 12 {
		t.Fatal(len(unknown.GetUnknown()))
	}
	unknownL := unknown.BLength()
	unknownBuf := make([]byte, unknownL)
	writeL := unknown.FastWriteNocopy(unknownBuf, nil)
	if writeL != unknownL {
		t.Fatal(writeL)
	}
	compare1 := &test.FullStruct{}
	if ll, err := compare1.FastRead(unknownBuf); err != nil {
		t.Fatal(err)
	} else if ll != unknownL {
		t.Fatal(ll)
	}
	if !compare1.DeepEqual(compare) {
		t.Fatal(compare1)
	}
}

func TestNoUnknownField(t *testing.T) {
	l := fullReq.BLength()
	buf := make([]byte, l)
	if ll := fullReq.FastWriteNocopy(buf, nil); ll != l {
		t.Fatal(ll)
	}

	ori := &test.FullStruct{}
	ll, err := ori.FastRead(buf)
	if err != nil {
		t.Fatal(err)
	}
	if ll != l {
		t.Fatal(ll)
	}

	if ori.DeepEqual(fullReq) {
		t.Fatal()
	}
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
		panic(e)
	}()
	_ = local.BLength()
	panic("not reachable")
}

func TestCorruptRead(t *testing.T) {
	local := &test.Local{L: 3}
	ufs := unknown.Fields{&unknown.Field{Name: "test", Type: unknown.TString, Value: "str"}}
	local.SetUnknown(ufs)
	l := local.BLength()
	buf := make([]byte, l)
	if ll := local.FastWriteNocopy(buf, nil); ll != l {
		t.Fatal(ll)
	}
	buf[7] = 200

	var local2 test.Local
	_, err := local2.FastRead(buf)
	if err == nil {
		t.Fatal()
	}
	if !strings.Contains(err.Error(), "unknown data type 200") {
		t.Fatal(err)
	}
}

func writeUF(f *unknown.Field) string {
	var val string
	switch f.Type {
	case unknown.TBool:
		val = fmt.Sprintf("%v", f.Value.(bool))
	case unknown.TByte:
		val = fmt.Sprintf("%v", f.Value.(int8))
	case unknown.TDouble:
		val = fmt.Sprintf("%v", f.Value.(float64))
	case unknown.TI16:
		val = fmt.Sprintf("%v", f.Value.(int16))
	case unknown.TI32:
		val = fmt.Sprintf("%v", f.Value.(int32))
	case unknown.TI64:
		val = fmt.Sprintf("%v", f.Value.(int64))
	case unknown.TString:
		val = fmt.Sprintf("\"%v\"", f.Value.(string))
	case unknown.TSet:
		fallthrough
	case unknown.TList:
		fallthrough
	case unknown.TMap:
		fallthrough
	case unknown.TStruct:
		val = unknownStr(f.Value.([]*unknown.Field))
	default:
		panic(fmt.Errorf("unknown data type %d", f.Type))
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("{Name: \"%v\" ID: %v Type: %v KType: %v VType: %v Value: %s}", f.Name, f.ID, f.Type, f.KeyType, f.ValType, val))
	return sb.String()
}

func unknownStr(fs unknown.Fields) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, f := range fs {
		sb.WriteString(writeUF(f))
		if i != len(fs)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString("]")
	return sb.String()
}
