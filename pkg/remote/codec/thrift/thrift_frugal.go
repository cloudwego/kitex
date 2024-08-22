//go:build (amd64 || arm64) && !windows && go1.16 && !go1.23 && !disablefrugal
// +build amd64 arm64
// +build !windows
// +build go1.16
// +build !go1.23
// +build !disablefrugal

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

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/frugal"
	"github.com/cloudwego/gopkg/protocol/thrift"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

// TODO(xiaost): rename hyper -> frugal after v0.11.0 is released

// hyperMarshalAvailable indicates that if high priority message codec is available.
func hyperMarshalAvailable(data interface{}) bool {
	dt := reflect.TypeOf(data).Elem()
	if dt.NumField() > 0 && dt.Field(0).Tag.Get("frugal") == "" {
		return false
	}
	return true
}

// hyperMessageUnmarshalAvailable indicates that if high priority message codec is available.
func (c thriftCodec) hyperMessageUnmarshalAvailable(data interface{}, payloadLen int) bool {
	if payloadLen == 0 && c.CodecType&EnableSkipDecoder == 0 {
		return false
	}
	dt := reflect.TypeOf(data).Elem()
	if dt.NumField() > 0 && dt.Field(0).Tag.Get("frugal") == "" {
		return false
	}
	return true
}

func (c thriftCodec) hyperMarshal(out remote.ByteBuffer, methodName string, msgType remote.MessageType,
	seqID int32, data interface{},
) error {
	// calculate and malloc message buffer
	msgBeginLen := thrift.Binary.MessageBeginLength(methodName)
	objectLen := frugal.EncodedSize(data)
	buf, err := out.Malloc(msgBeginLen + objectLen)
	if err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, Malloc failed: %s", err.Error()))
	}
	mallocLen := out.MallocLen()

	// encode message
	offset := thrift.Binary.WriteMessageBegin(buf, methodName, thrift.TMessageType(msgType), seqID)
	nw, _ := out.(remote.NocopyWrite)
	_, err = frugal.EncodeObject(buf[offset:], nw, data)
	if err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, Encode failed: %s", err.Error()))
	}
	if nw != nil {
		return nw.MallocAck(mallocLen)
	}
	return nil
}

func (c thriftCodec) hyperMarshalBody(data interface{}) (buf []byte, err error) {
	objectLen := frugal.EncodedSize(data)
	buf = mcache.Malloc(objectLen)
	_, err = frugal.EncodeObject(buf, nil, data)
	return buf, err
}

func (c thriftCodec) hyperMessageUnmarshal(buf []byte, data interface{}) error {
	_, err := frugal.DecodeObject(buf, data)
	if err != nil {
		return err
	}
	return nil
}
