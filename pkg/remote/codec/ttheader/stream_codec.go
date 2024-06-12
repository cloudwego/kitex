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

package ttheader

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

var (
	_ remote.Codec = (*streamCodec)(nil)

	ErrInvalidPayload = errors.New("ttheader streaming: invalid payload")

	ttheaderCodec = codec.TTHeaderCodec()
)

// streamCodec implements the ttheader codec
type streamCodec struct{}

// NewStreamCodec ttheader codec construction
func NewStreamCodec() remote.Codec {
	return &streamCodec{}
}

// Encode implements the remote.Codec interface
func (c *streamCodec) Encode(ctx context.Context, message remote.Message, out remote.ByteBuffer) (err error) {
	var payload []byte
	defer func() {
		// record send size, even when err != nil (0 is recorded to the lastSendSize)
		if rpcStats := rpcinfo.AsMutableRPCStats(message.RPCInfo().Stats()); rpcStats != nil {
			rpcStats.IncrSendSize(uint64(len(payload)))
		}
	}()

	if codec.MessageFrameType(message) == codec.FrameTypeData {
		if payload, err = c.marshalData(ctx, message); err != nil {
			return err
		}
	}

	framedSizeBuf, err := ttheaderCodec.EncodeHeader(ctx, message, out)
	if err != nil {
		return err
	}
	headerSize := out.MallocLen() - codec.Size32
	binary.BigEndian.PutUint32(framedSizeBuf, uint32(headerSize+len(payload)))
	return codec.WriteAll(out.WriteBinary, payload)
}

func (c *streamCodec) marshalData(ctx context.Context, message remote.Message) ([]byte, error) {
	if message.Data() == nil { // for non-data frames: MetaFrame, HeaderFrame, TrailerFrame(w/o err)
		return nil, nil
	}
	structCodec, err := remote.GetStructCodec(message)
	if err != nil {
		return nil, ErrInvalidPayload
	}
	return structCodec.Serialize(ctx, message.Data())
}

// Decode implements the remote.Codec interface
func (c *streamCodec) Decode(ctx context.Context, message remote.Message, in remote.ByteBuffer) (err error) {
	var payload []byte
	defer func() {
		if rpcStats := rpcinfo.AsMutableRPCStats(message.RPCInfo().Stats()); rpcStats != nil {
			// record recv size, even when err != nil (0 is recorded to the lastRecvSize)
			rpcStats.IncrRecvSize(uint64(len(payload)))
		}
	}()
	if err = ttheaderCodec.DecodeHeader(ctx, message, in); err != nil {
		return err
	}
	if codec.MessageFrameType(message) != codec.FrameTypeData {
		return nil
	}
	if message.Data() == nil {
		return in.Skip(message.PayloadLen()) // necessary for discarding dirty frames
	}
	// only data frame need to decode payload into message.Data()
	if payload, err = in.Next(message.PayloadLen()); err != nil {
		return err
	}
	structCodec, err := remote.GetStructCodec(message)
	if err != nil {
		return ErrInvalidPayload
	}
	return structCodec.Deserialize(ctx, message.Data(), payload)
}

// Name implements the remote.Codec interface
func (c *streamCodec) Name() string {
	return "ttheader streaming"
}
