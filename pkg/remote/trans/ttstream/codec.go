/*
 * Copyright 2026 CloudWeGo Authors
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

package ttstream

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/gopkg/bufiox"
	gopkgthrift "github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	icodec "github.com/cloudwego/kitex/internal/codec"
	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

var (
	defaultThriftCodec   = thriftCodec{}
	defaultProtobufCodec = protobufCodec{}
)

func getCodec(protocolId ttheader.ProtocolID) codec {
	if protocolId == ttheader.ProtocolIDProtobufStruct {
		return defaultProtobufCodec
	}
	return defaultThriftCodec
}

// codec is used to encode/decode payload of ttstream Data Frame.
// now supports thrift and protobuf
type codec interface {
	encode(ctx context.Context, ri rpcinfo.RPCInfo, msg any) (payload []byte, needRecycle bool, err error)
	decode(ctx context.Context, ri rpcinfo.RPCInfo, payload []byte, msg any) error
}

var (
	errEncodeThriftGenericEmptyMethod = errors.New("empty methodName in ttstream thrift generic Encode")
	errDecodeThriftGenericEmptyMethod = errors.New("empty methodName in ttstream thrift generic Decode")
)

type thriftCodec struct{}

func (c thriftCodec) encode(ctx context.Context, ri rpcinfo.RPCInfo, msg any) (payload []byte, needRecycle bool, err error) {
	switch t := msg.(type) {
	case gopkgthrift.FastCodec:
		return gopkgthrift.FastMarshal(t), false, nil
	case *generic.Args:
		return encodeThriftGeneric(ctx, ri, t)
	case *generic.Result:
		return encodeThriftGeneric(ctx, ri, t)
	default:
		return nil, false, fmt.Errorf("invalid payload type %T in ttstream thrift Encode", t)
	}
}

func encodeThriftGeneric(ctx context.Context, ri rpcinfo.RPCInfo, writer igeneric.ThriftWriter) (payload []byte, needRecycle bool, err error) {
	methodName := ri.Invocation().MethodName()
	if methodName == "" {
		return nil, false, errEncodeThriftGenericEmptyMethod
	}
	var buf []byte
	w := bufiox.NewBytesWriter(&buf)
	err = writer.Write(ctx, methodName, w)
	if err != nil {
		return nil, false, err
	}
	w.Flush()
	return buf, false, nil
}

func (c thriftCodec) decode(ctx context.Context, ri rpcinfo.RPCInfo, payload []byte, msg any) error {
	switch t := msg.(type) {
	case gopkgthrift.FastCodec:
		return gopkgthrift.FastUnmarshal(payload, t)
	case *generic.Args:
		return decodeThriftGeneric(ctx, ri, payload, t)
	case *generic.Result:
		return decodeThriftGeneric(ctx, ri, payload, t)
	default:
		return fmt.Errorf("invalid payload type %T in ttstream thrift Decode", t)
	}
}

func decodeThriftGeneric(ctx context.Context, ri rpcinfo.RPCInfo, payload []byte, reader igeneric.ThriftReader) error {
	methodName := ri.Invocation().MethodName()
	if methodName == "" {
		return errDecodeThriftGenericEmptyMethod
	}
	r := bufiox.NewBytesReader(payload)
	return reader.Read(ctx, methodName, len(payload), r)
}

type protobufCodec struct{}

func (p protobufCodec) encode(ctx context.Context, ri rpcinfo.RPCInfo, msg any) ([]byte, bool, error) {
	res, err := icodec.TTStreamEncodeProtobufStruct(ctx, ri, msg)
	if err != nil {
		return nil, false, err
	}

	return res.Payload, res.PreAllocate, nil
}

func (p protobufCodec) decode(ctx context.Context, ri rpcinfo.RPCInfo, payload []byte, msg any) error {
	return icodec.DecodeProtobufStruct(ctx, ri, payload, msg)
}
