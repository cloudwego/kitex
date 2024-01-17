package allocator_test

import (
	"fmt"
	"reflect"
	"testing"

	tt "github.com/cloudwego/kitex/internal/test"

	"github.com/cloudwego/kitex/pkg/allocator"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift/allocator_test/kitex_gen/test"
	"github.com/cloudwego/kitex/pkg/utils/fastthrift"
)

var benchcases []benchcase

type benchcase struct {
	Name string
	req  *test.FullStruct
}

func init() {
	simpleDesc := string(make([]byte, 10))
	largeDesc := string(make([]byte, 10240))
	status := test.HTTPStatus_NOT_FOUND
	byte1 := int8(1)
	double1 := 1.3

	simpleReq := &test.FullStruct{
		Left: 32,
		InnerReq: &test.Inner{
			Num:       6,
			Desc:      &simpleDesc,
			MapOfList: map[int64][]int64{42: {1, 2}},
			Byte1:     &byte1,
			Double1:   &double1,
		},
	}
	complexReq := &test.FullStruct{
		Left:  32,
		Right: 45,
		Dummy: []byte("123"),
		InnerReq: &test.Inner{
			Num:          6,
			Desc:         &simpleDesc,
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
		I64Set: []int64{1, 2, 3},
		Int16:  98,
		IsSet:  true,
	}
	largeStringReq := &test.FullStruct{
		Left: 32,
		InnerReq: &test.Inner{
			Num:       6,
			Desc:      &largeDesc,
			MapOfList: map[int64][]int64{42: {1, 2}},
			Byte1:     &byte1,
			Double1:   &double1,
		},
	}
	benchcases = []benchcase{
		{Name: "Simple", req: simpleReq},
		{Name: "Complex", req: complexReq},
		{Name: "LargeString", req: largeStringReq},
	}
	_, _, _ = simpleReq, complexReq, largeStringReq
}

func BenchmarkNativeFastRead(b *testing.B) {
	for _, bc := range benchcases {
		var req *test.FullStruct // we should make req escape to heap
		b.Run(bc.Name, func(b *testing.B) {
			buf := fastthrift.FastMarshal(bc.req)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req = test.NewFullStruct()
				err := fastthrift.FastUnmarshal(buf, req)
				if err != nil {
					b.Fatal("unmarshal failed", err)
				}
			}
		})
		_ = req
	}
}

func BenchmarkAllocatorFastRead(b *testing.B) {
	for _, bc := range benchcases {
		var req *test.FullStruct // we should make req escape to heap
		b.Run(bc.Name, func(b *testing.B) {
			buf := fastthrift.FastMarshal(bc.req)
			var structSize = int(reflect.TypeOf(test.FullStruct{}).Size())
			b.Logf("Struct Size: %d, Buffer Size: %d", structSize, len(buf))

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ac := allocator.NewAllocator(len(buf) + structSize)
				req = allocator.New[test.FullStruct](ac)
				req.InitDefault()
				bp := bthrift.NewABinaryProtocol(ac)
				_, err := req.FastRead(bp, buf)
				if err != nil {
					b.Fatal("unmarshal failed", err)
				}
			}
		})
		_ = req
	}
}

func TestAllocatorFastRead(t *testing.T) {
	for _, bc := range benchcases {
		rawBuf := fastthrift.FastMarshal(bc.req)
		// native binary protocol
		rawReq := new(test.FullStruct)
		err := fastthrift.FastUnmarshal(rawBuf, rawReq)
		tt.Assert(t, err == nil, err)

		// allocator binary protocol
		ac := allocator.NewAllocator(len(rawBuf) + 1024)
		bp := bthrift.NewABinaryProtocol(ac)
		req := allocator.New[test.FullStruct](ac)
		_, err = req.FastRead(bp, rawBuf)
		fmt.Printf("%s %v\n", bc.Name, req)
		tt.Assert(t, err == nil, err)
		tt.DeepEqual(t, rawReq.InnerReq.Byte1, req.InnerReq.Byte1)
		tt.DeepEqual(t, *rawReq.InnerReq.Desc, *req.InnerReq.Desc)
		tt.DeepEqual(t, rawReq, req)

		//buf := fastthrift.FastMarshal(req)
		//tt.DeepEqual(t, rawBuf, buf)
	}
}
