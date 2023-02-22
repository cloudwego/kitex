//go:build amd64 && !windows && go1.16
// +build amd64,!windows,go1.16

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

package thrift

import (
	"fmt"
	"reflect"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/frugal"

	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

const (
	// 0x1 0x10 used for FastWrite and FastRead, so Frugal starts from 0x100
	FrugalWrite CodecType = 1 << (iota + 2)
	FrugalRead
)

// hyperMarshalEnabled indicates that if there are high priority message codec for current platform.
func (c thriftCodec) hyperMarshalEnabled() bool {
	return c.CodecType&FrugalWrite != 0
}

// hyperMarshalAvailable indicates that if high priority message codec is available.
func hyperMarshalAvailable(data interface{}) bool {
	dt := reflect.TypeOf(data).Elem()
	if dt.NumField() > 0 && dt.Field(0).Tag.Get("frugal") == "" {
		return false
	}
	return true
}

// hyperMessageUnmarshalEnabled indicates that if there are high priority message codec for current platform.
func (c thriftCodec) hyperMessageUnmarshalEnabled() bool {
	return c.CodecType&FrugalRead != 0
}

// hyperMessageUnmarshalAvailable indicates that if high priority message codec is available.
func hyperMessageUnmarshalAvailable(data interface{}, message remote.Message) bool {
	if message.PayloadLen() == 0 {
		return false
	}
	dt := reflect.TypeOf(data).Elem()
	if dt.NumField() > 0 && dt.Field(0).Tag.Get("frugal") == "" {
		return false
	}
	return true
}

func (c thriftCodec) hyperMarshal(data interface{}, message remote.Message, out remote.ByteBuffer) error {
	// prepare message info
	methodName := message.RPCInfo().Invocation().MethodName()
	msgType := message.MessageType()
	seqID := message.RPCInfo().Invocation().SeqID()

	// calculate and malloc message buffer
	msgBeginLen := bthrift.Binary.MessageBeginLength(methodName, thrift.TMessageType(msgType), seqID)
	msgEndLen := bthrift.Binary.MessageEndLength()
	objectLen := frugal.EncodedSize(data)
	buf, err := out.Malloc(msgBeginLen + objectLen + msgEndLen)
	if err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, Malloc failed: %s", err.Error()))
	}

	// encode message
	offset := bthrift.Binary.WriteMessageBegin(buf, methodName, thrift.TMessageType(msgType), seqID)
	var writeLen int
	writeLen, err = frugal.EncodeObject(buf[offset:], nil, data)
	if err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, Encode failed: %s", err.Error()))
	}
	offset += writeLen
	bthrift.Binary.WriteMessageEnd(buf[offset:])
	return nil
}

func (c thriftCodec) hyperMessageUnmarshal(buf []byte, data interface{}) error {
	_, err := frugal.DecodeObject(buf, data)
	if err != nil {
		return remote.NewTransError(remote.ProtocolError, err)
	}
	return nil
}
