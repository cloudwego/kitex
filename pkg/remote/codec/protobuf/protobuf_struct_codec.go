/*
 * Copyright 2024 CloudWeGo Authors
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

package protobuf

import (
	"context"
	"errors"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/fastpb"
	"google.golang.org/protobuf/proto"
)

// This is for streaming transports including grpc and ttheader

var (
	ErrInvalidPayload    = errors.New("protobuf: invalid payload")
	ErrDataChanged       = errors.New("protobuf: data changed between size calculation and serialization")
	ErrDataUnmarshalable = errors.New("protobuf: data unmarshalable")
)

type marshaler interface {
	MarshalTo(data []byte) (n int, err error)
	Size() int
}

func (c protobufCodec) Serialize(ctx context.Context, data interface{}) (payload []byte, err error) {
	switch t := data.(type) {
	case fastpb.Writer:
		size := t.Size()
		payload = mcache.Malloc(size)
		if n := t.FastWrite(payload); n != size {
			return nil, ErrDataChanged
		}
		return payload, nil
	case marshaler:
		size := t.Size()
		payload = mcache.Malloc(size)
		_, err = t.MarshalTo(payload)
		return payload, err
	case ProtobufMsgCodec:
		return t.Marshal(nil)
	case ProtobufV2MsgCodec:
		return t.XXX_Marshal(nil, true)
	case proto.Message:
		return proto.Marshal(t)
	default:
		return nil, ErrDataUnmarshalable
	}
}

// Deserialize implements the remote.StructCodec interface.
// It deserializes the protobuf struct in to message.Data().
func (c protobufCodec) Deserialize(ctx context.Context, data interface{}, payload []byte) (err error) {
	if t, ok := data.(fastpb.Reader); ok {
		if len(payload) == 0 {
			// if all fields of a struct is default value, data will be nil
			// In the implementation of fastpb, if data is nil, then fastpb will skip creating this struct, as a result user will get a nil pointer which is not expected.
			// So, when data is nil, use default protobuf unmarshal method to decode the struct.
			// todo: fix fastpb
		} else {
			_, err = fastpb.ReadMessage(payload, fastpb.SkipTypeCheck, t)
			return err
		}
	}
	switch t := data.(type) {
	case ProtobufV2MsgCodec:
		return t.XXX_Unmarshal(payload)
	case proto.Message:
		return proto.Unmarshal(payload, t)
	case ProtobufMsgCodec:
		return t.Unmarshal(payload)
	default:
		return ErrInvalidPayload
	}
}
