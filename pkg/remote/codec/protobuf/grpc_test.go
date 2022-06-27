package protobuf

import (
	"github.com/cloudwego/kitex/internal/test"
	"reflect"
	"testing"
)

var (
	cop = &grpcCodec{}
)

func Test_grpcCodec_Name(t *testing.T) {
	newGrpcCodec := &grpcCodec{}
	test.Assert(t, newGrpcCodec.Name() == "grpc")
}

func TestNewGRPCCodec(t *testing.T) {
	codec := NewGRPCCodec()
	test.Assert(t, reflect.DeepEqual(codec, cop))
}
