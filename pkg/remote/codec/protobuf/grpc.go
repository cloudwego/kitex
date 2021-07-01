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

package protobuf

import (
	"context"
	"encoding/binary"

	"google.golang.org/protobuf/proto"

	"github.com/cloudwego/kitex/pkg/remote"
)

// gogoproto generate
type marshaler interface {
	MarshalTo(data []byte) (n int, err error)
	Size() int
}

type protobufV2MsgCodec interface {
	XXX_Unmarshal(b []byte) error
	XXX_Marshal(b []byte, deterministic bool) ([]byte, error)
}

// NewGRPCCodec create grpc and protobuf codec
func NewGRPCCodec() remote.Codec {
	return new(grpcCodec)
}

type grpcCodec struct {
}

func (c *grpcCodec) Encode(ctx context.Context, message remote.Message, out remote.ByteBuffer) (err error) {
	// 5byte + body
	data := message.Data()
	var (
		size int
		buf  []byte
	)
	switch t := data.(type) {
	case marshaler:
		size = t.Size()
		buf, err = out.Malloc(5 + size)
		if err != nil {
			return err
		}
		if _, err = t.MarshalTo(buf[5:]); err != nil {
			return err
		}
	case protobufV2MsgCodec:
		d, err := t.XXX_Marshal(nil, true)
		if err != nil {
			return err
		}
		size = len(d)
		buf, err = out.Malloc(5 + size)
		if err != nil {
			return err
		}
		copy(buf[5:], d)
	case proto.Message:
		d, err := proto.Marshal(t)
		if err != nil {
			return err
		}
		size = len(d)
		buf, err = out.Malloc(5 + size)
		if err != nil {
			return err
		}
		copy(buf[5:], d)
	case protobufMsgCodec:
		d, err := t.Marshal(nil)
		if err != nil {
			return err
		}
		size = len(d)
		buf, err = out.Malloc(5 + size)
		if err != nil {
			return err
		}
		copy(buf[5:], d)
	}
	// reuse buffer need clean data
	buf[0] = 0
	binary.BigEndian.PutUint32(buf[1:5], uint32(size))
	return nil
}

func (c *grpcCodec) Decode(ctx context.Context, message remote.Message, in remote.ByteBuffer) (err error) {
	hdr, err := in.Next(5)
	if err != nil {
		return err
	}
	dLen := int(binary.BigEndian.Uint32(hdr[1:]))
	d, err := in.Next(dLen)
	if err != nil {
		return err
	}
	data := message.Data()
	switch t := data.(type) {
	case protobufV2MsgCodec:
		return t.XXX_Unmarshal(d)
	case proto.Message:
		return proto.Unmarshal(d, t)
	case protobufMsgCodec:
		return t.Unmarshal(d)
	}
	return nil
}

func (c *grpcCodec) Name() string {
	return "grpc"
}
