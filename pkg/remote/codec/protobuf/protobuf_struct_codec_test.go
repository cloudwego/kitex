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
	"reflect"
	"testing"

	"github.com/cloudwego/fastpb"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/httppb_test/idl"
)

// The following structs are just for test, not conforming to the protobuf specification
var (
	_ fastpb.Writer      = (*fastStruct)(nil)
	_ fastpb.Reader      = (*fastStruct)(nil)
	_ marshaler          = (*marshalerStruct)(nil)
	_ ProtobufMsgCodec   = (*pbMsgStruct)(nil)
	_ ProtobufV2MsgCodec = (*pbMsgV2Struct)(nil)
)

type pbMsgV2Struct struct {
	buf []byte
}

func (p *pbMsgV2Struct) XXX_Unmarshal(b []byte) error {
	p.buf = make([]byte, len(b))
	copy(p.buf, b)
	return nil
}

func (p *pbMsgV2Struct) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return p.buf, nil
}

type pbMsgStruct struct {
	buf []byte
}

func (p *pbMsgStruct) Marshal(out []byte) ([]byte, error) {
	return p.buf, nil
}

func (p *pbMsgStruct) Unmarshal(in []byte) error {
	p.buf = make([]byte, len(in))
	copy(p.buf, in)
	return nil
}

type fastStruct struct {
	size int
	buf  []byte
}

func (p *fastStruct) FastRead(buf []byte, _type int8, number int32) (n int, err error) {
	p.buf = make([]byte, len(buf))
	return copy(buf, p.buf), nil
}

func (p *fastStruct) Size() (n int) {
	return p.size
}

func (p *fastStruct) FastWrite(buf []byte) (n int) {
	return copy(buf, p.buf)
}

type marshalerStruct struct {
	size int
	buf  []byte
}

func (p *marshalerStruct) Size() (n int) {
	return p.size
}

func (p *marshalerStruct) MarshalTo(data []byte) (n int, err error) {
	p.buf = make([]byte, len(data))
	return copy(data, p.buf), nil
}

func Test_protobufCodec_Serialize(t *testing.T) {
	c := &protobufCodec{}
	t.Run("fastpb.Writer", func(t *testing.T) {
		data := &fastStruct{
			size: 4,
			buf:  []byte{1, 2, 3, 4},
		}

		buf, err := c.Serialize(context.Background(), data)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(buf, data.buf), buf)
	})
	t.Run("marshaler", func(t *testing.T) {
		data := &marshalerStruct{
			size: 4,
			buf:  []byte{1, 2, 3, 4},
		}
		buf, err := c.Serialize(context.Background(), data)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(buf, data.buf), buf)
	})
	t.Run("ProtobufMsgCodec", func(t *testing.T) {
		data := &pbMsgStruct{
			buf: []byte{1, 2, 3, 4},
		}
		buf, err := c.Serialize(context.Background(), data)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(buf, data.buf), buf)
	})
	t.Run("ProtobufV2MsgCodec", func(t *testing.T) {
		data := &idl.Message{Tiny: 1}
		buf, err := c.Serialize(context.Background(), data)
		test.Assert(t, err == nil, err)
		test.Assert(t, len(buf) != 0, buf)
	})
	t.Run("invalid-data-type", func(t *testing.T) {
		data := struct{}{}
		_, err := c.Serialize(context.Background(), data)
		test.Assert(t, errors.Is(err, ErrDataUnmarshalable), err)
	})
}

func Test_protobufCodec_Deserialize(t *testing.T) {
	c := &protobufCodec{}
	t.Run("fastpb.Reader:non-zero-payload", func(t *testing.T) {
		buf := []byte{byte(protowire.EncodeTag(1, 1)), 1, 2, 3, 4}
		data := &fastStruct{}
		err := c.Deserialize(context.Background(), data, buf)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(data.buf, buf[1:]), data.buf)
	})
	t.Run("fastpb.Reader:zero-payload", func(t *testing.T) {
		buf := []byte{}
		data := &fastStruct{}
		err := c.Deserialize(context.Background(), data, buf)
		test.Assert(t, errors.Is(err, ErrInvalidPayload), err) // no fallback
	})
	t.Run("ProtobufV2MsgCodec", func(t *testing.T) {
		buf := []byte{1, 2, 3, 4}
		data := &pbMsgV2Struct{}
		err := c.Deserialize(context.Background(), data, buf)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(data.buf, buf), data.buf)
	})
	t.Run("proto.Message", func(t *testing.T) {
		data := &idl.Message{Tiny: 1}
		buf, err := c.Serialize(context.Background(), data)
		test.Assert(t, err == nil, err)
		test.Assert(t, len(buf) != 0, buf)

		rData := &idl.Message{}
		err = c.Deserialize(context.Background(), rData, buf)
		test.Assert(t, err == nil, err)
		test.Assert(t, rData.Tiny == 1, rData.Tiny)
	})
	t.Run("ProtobufMsgCodec", func(t *testing.T) {
		buf := []byte{1, 2, 3, 4}
		data := &pbMsgStruct{}
		err := c.Deserialize(context.Background(), data, buf)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(data.buf, buf), data.buf)
	})
	t.Run("invalid-data-type", func(t *testing.T) {
		data := struct{}{}
		err := c.Deserialize(context.Background(), data, []byte{})
		test.Assert(t, errors.Is(err, ErrInvalidPayload), err)
	})
}
