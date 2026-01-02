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

package grpc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/gopkg/bufiox"

	icodec "github.com/cloudwego/kitex/internal/codec"
	"github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/internal/utils/safemcache"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

const dataFrameHeaderLen = 5

var (
	ErrInvalidPayload          = errors.New("grpc invalid payload")
	errWrongGRPCImplementation = errors.New("KITEX: grpc client streaming protocol violation: get <nil>, want <EOF>")
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

type grpcCodec struct {
	ThriftCodec remote.PayloadCodec
}

type CodecOption func(c *grpcCodec)

func WithThriftCodec(t remote.PayloadCodec) CodecOption {
	return func(c *grpcCodec) {
		c.ThriftCodec = t
	}
}

// NewGRPCCodec create grpc and protobuf codec
func NewGRPCCodec(opts ...CodecOption) remote.Codec {
	codec := &grpcCodec{}
	for _, opt := range opts {
		opt(codec)
	}
	if !thrift.IsThriftCodec(codec.ThriftCodec) {
		codec.ThriftCodec = thrift.NewThriftCodec()
	}
	return codec
}

func mallocWithFirstByteZeroed(size int) []byte {
	data := safemcache.Malloc(size)
	data[0] = 0 // compressed flag = false
	return data
}

func (c *grpcCodec) Encode(ctx context.Context, message remote.Message, out remote.ByteBuffer) (err error) {
	var payload []byte
	defer func() {
		// record send size, even when err != nil (0 is recorded to the lastSendSize)
		if rpcStats := rpcinfo.AsMutableRPCStats(message.RPCInfo().Stats()); rpcStats != nil {
			rpcStats.IncrSendSize(uint64(len(payload)))
		}
	}()

	writer, ok := out.(remote.FrameWrite)
	if !ok {
		return fmt.Errorf("output buffer must implement FrameWrite")
	}
	compressor, err := getSendCompressor(ctx)
	if err != nil {
		return err
	}
	isCompressed := compressor != nil

	switch message.RPCInfo().Config().PayloadCodec() {
	case serviceinfo.Thrift:
		switch msg := message.Data().(type) {
		case generic.ThriftWriter:
			methodName := message.RPCInfo().Invocation().MethodName()
			if methodName == "" {
				return errors.New("empty methodName in grpc generic streaming Encode")
			}
			var bs []byte
			bw := bufiox.NewBytesWriter(&bs)
			if err = msg.Write(ctx, methodName, bw); err != nil {
				return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("generic thrift streaming marshal, Write failed: %s", err.Error()))
			}
			if err = bw.Flush(); err != nil {
				return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("generic thrift streaming marshal, Flush failed: %s", err.Error()))
			}
			payload = bs
		default:
			payload, err = thrift.MarshalThriftData(ctx, c.ThriftCodec, message.Data())
		}
	case serviceinfo.Protobuf:
		var res icodec.EncodeResult
		if !isCompressed {
			res, err = icodec.GRPCEncodeProtobufStruct(ctx, message.RPCInfo(), message.Data(), false)
			if err != nil {
				return err
			}
			payload = res.Payload
			if res.PreAllocate {
				payload[0] = 0
				binary.BigEndian.PutUint32(payload[1:dataFrameHeaderLen], uint32(res.PreAllocateSize))
				return writer.WriteData(payload)
			}
		} else {
			res, err = icodec.GRPCEncodeProtobufStruct(ctx, message.RPCInfo(), message.Data(), true)
			if err != nil {
				return err
			}
			payload = res.Payload
		}
	default:
		return ErrInvalidPayload
	}

	if err != nil {
		return err
	}
	header := safemcache.Malloc(dataFrameHeaderLen)
	if isCompressed {
		payload, err = compress(compressor, payload) // compress will `Free` payload
		if err != nil {
			return err
		}
		header[0] = 1
	} else {
		header[0] = 0
	}
	binary.BigEndian.PutUint32(header[1:dataFrameHeaderLen], uint32(len(payload)))
	err = writer.WriteHeader(header)
	if err != nil {
		return err
	}
	return writer.WriteData(payload)
}

func (c *grpcCodec) Decode(ctx context.Context, message remote.Message, in remote.ByteBuffer) (err error) {
	d, err := decodeGRPCFrame(ctx, in)
	if rpcStats := rpcinfo.AsMutableRPCStats(message.RPCInfo().Stats()); rpcStats != nil {
		// record recv size, even when err != nil (0 is recorded to the lastRecvSize)
		rpcStats.IncrRecvSize(uint64(len(d)))
	}
	if err != nil {
		return err
	}
	// For ClientStreaming and Unary, server may return an err(e.g. status) as trailer frame.
	// We need to receive this trailer frame.
	if message.RPCRole() == remote.Client && isNonServerStreaming(message.RPCInfo().Invocation().StreamingMode()) {
		// Receive trailer frame
		// If err == nil, wrong gRPC protocol implementation.
		// If err == io.EOF, it means server returns nil, just ignore io.EOF.
		// If err != io.EOF, it means server returns status err or BizStatusErr, or other gRPC transport error came out,
		// we need to throw it to users.
		_, err = decodeGRPCFrame(ctx, in)
		if err == nil {
			return errWrongGRPCImplementation
		}
		if err != io.EOF {
			return err
		}
		// ignore io.EOF
	}
	message.SetPayloadLen(len(d))
	data := message.Data()
	switch message.RPCInfo().Config().PayloadCodec() {
	case serviceinfo.Thrift:
		switch t := data.(type) {
		case generic.ThriftReader:
			methodName := message.RPCInfo().Invocation().MethodName()
			if methodName == "" {
				return errors.New("empty methodName in grpc Decode")
			}
			return t.Read(ctx, methodName, len(d), remote.NewReaderBuffer(d))
		default:
			return thrift.UnmarshalThriftData(ctx, c.ThriftCodec, "", d, message.Data())
		}
	case serviceinfo.Protobuf:
		return icodec.DecodeProtobufStruct(ctx, message.RPCInfo(), d, data)
	default:
		return ErrInvalidPayload
	}
}

func (c *grpcCodec) Name() string {
	return "grpc"
}

func isNonServerStreaming(mode serviceinfo.StreamingMode) bool {
	if mode == serviceinfo.StreamingClient || mode == serviceinfo.StreamingUnary || mode == serviceinfo.StreamingNone {
		return true
	}
	// BidiStreaming has the ability of ServerStreaming, is also considered as ServerStreaming
	return false
}
