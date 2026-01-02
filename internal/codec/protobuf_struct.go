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

package codec

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/fastpb"
	"google.golang.org/protobuf/proto"

	"github.com/cloudwego/kitex/internal/utils/safemcache"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

var (
	errEncodePbGenericEmptyMethod = errors.New("empty methodName in generic pb Encode")
	errDecodePbGenericEmptyMethod = errors.New("empty methodName in generic pb Decode")
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

type EncodeResult struct {
	Payload         []byte // encoded byte slice
	PreAllocate     bool   // the struct encoded can pre-allocate memory
	PreAllocateSize int    // pre-allocate size for the struct encoded
}

const GRPCDataFrameHeaderLen = 5

func GRPCEncodeProtobufStruct(ctx context.Context, ri rpcinfo.RPCInfo, msg any, isCompress bool) (EncodeResult, error) {
	prefixLen := GRPCDataFrameHeaderLen
	if isCompress {
		prefixLen = 0
	}
	return encodeProtobufStruct(ctx, ri, msg, safemcache.Malloc, safemcache.Free, prefixLen)
}

func TTStreamEncodeProtobufStruct(ctx context.Context, ri rpcinfo.RPCInfo, msg any) (EncodeResult, error) {
	return encodeProtobufStruct(ctx, ri, msg, mcacheMalloc, mcache.Free, 0)
}

func mcacheMalloc(size int) []byte {
	return mcache.Malloc(size)
}

func encodeProtobufStruct(ctx context.Context, ri rpcinfo.RPCInfo, msg any,
	mallocFunc func(int) []byte, freeFunc func([]byte), prefixLen int,
) (res EncodeResult, err error) {
	var payload []byte
	switch t := msg.(type) {
	// Deprecated: fastpb is no longer used
	case fastpb.Writer:
		res.PreAllocate = true
		res.PreAllocateSize = t.Size()
		payload = mallocFunc(res.PreAllocateSize + prefixLen)
		t.FastWrite(payload[prefixLen:])
	case marshaler:
		size := t.Size()
		payload = mallocFunc(size + prefixLen)
		if _, err = t.MarshalTo(payload[prefixLen:]); err != nil {
			freeFunc(payload)
			return res, err
		}
		res.PreAllocate = true
		res.PreAllocateSize = size
	case protobufV2MsgCodec:
		payload, err = t.XXX_Marshal(nil, true)
	case proto.Message:
		payload, err = proto.Marshal(t)
	case protobuf.ProtobufMsgCodec:
		payload, err = t.Marshal(nil)
	case protobuf.MessageWriterWithContext:
		payload, err = encodeProtobufGeneric(ctx, ri, t)
	default:
		err = fmt.Errorf("invalid payload %T in EncodeProtobufStruct", t)
	}

	if err != nil {
		return res, err
	}
	res.Payload = payload
	return res, nil
}

func encodeProtobufGeneric(ctx context.Context, ri rpcinfo.RPCInfo, w protobuf.MessageWriterWithContext) ([]byte, error) {
	methodName := ri.Invocation().MethodName()
	if methodName == "" {
		return nil, errEncodePbGenericEmptyMethod
	}
	actualMsg, err := w.WritePb(ctx, methodName)
	if err != nil {
		return nil, err
	}
	payload, ok := actualMsg.([]byte)
	if !ok {
		return nil, fmt.Errorf("encodePbGeneric failed, got %T", actualMsg)
	}
	return payload, nil
}

func DecodeProtobufStruct(ctx context.Context, ri rpcinfo.RPCInfo, payload []byte, msg any) (err error) {
	// Deprecated: fastpb is no longer used
	if t, ok := msg.(fastpb.Reader); ok {
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
	switch t := msg.(type) {
	case protobufV2MsgCodec:
		return t.XXX_Unmarshal(payload)
	case proto.Message:
		return proto.Unmarshal(payload, t)
	case protobuf.ProtobufMsgCodec:
		return t.Unmarshal(payload)
	case protobuf.MessageReaderWithMethodWithContext:
		methodName := ri.Invocation().MethodName()
		if methodName == "" {
			return errDecodePbGenericEmptyMethod
		}
		return t.ReadPb(ctx, methodName, payload)
	default:
		return fmt.Errorf("invalid payload %T in DecodeProtobufStruct", t)
	}
}
