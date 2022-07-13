package testutil

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
)

func MarshalAny(message proto.Message) *any.Any {
	a, _ := ptypes.MarshalAny(message)
	return a
}
