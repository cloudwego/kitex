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

package generic

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

const (
	metaInfoFixLen = 8
)

var _ remote.PayloadCodec = &binaryProtobufCodec{}

type binaryPbReqType = []byte

type binaryProtobufCodec struct {
	protobufCodec remote.PayloadCodec
}

func (c *binaryProtobufCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	data := msg.Data()
	if data == nil {
		return perrors.NewProtocolErrorWithMsg("invalid marshal data in rawProtobufBinaryCodec: nil")
	}
	if msg.MessageType() == remote.Exception {
		if err := c.protobufCodec.Marshal(ctx, msg, out); err != nil {
			return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("rawProtobufBinaryCodec Marshal exception failed, err: %s", err.Error()))
		}
		return nil
	}

	methodName := msg.RPCInfo().Invocation().MethodName()
	if methodName == "" {
		return errors.New("empty methodName in protobuf Marshal")
	}
	// 3. encode metainfo
	// 3.1 magic && msgType
	if err := codec.WriteUint32(codec.ProtobufV1Magic+uint32(msg.MessageType()), out); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("protobuf marshal, write meta info failed: %s", err.Error()))
	}
	// 3.2 methodName
	if _, err := codec.WriteString(methodName, out); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("protobuf marshal, write method name failed: %s", err.Error()))
	}
	// 3.3 seqID
	if err := codec.WriteUint32(uint32(msg.RPCInfo().Invocation().SeqID()), out); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("protobuf marshal, write seqID failed: %s", err.Error()))
	}

	var transBuff []byte
	var ok bool
	if msg.RPCRole() == remote.Server {
		gResult := data.(*Result)
		transBinary := gResult.Success
		if transBuff, ok = transBinary.(binaryPbReqType); !ok {
			return perrors.NewProtocolErrorWithMsg("invalid marshal result in rawProtobufBinaryCodec: must be []byte")
		}
	} else {
		gArg := data.(*Args)
		transBinary := gArg.Request
		if transBuff, ok = transBinary.(binaryPbReqType); !ok {
			return perrors.NewProtocolErrorWithMsg("invalid marshal request in rawProtobufBinaryCodec: must be []byte")
		}
	}
	out.WriteBinary(transBuff)
	return nil
}

func (c *binaryProtobufCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	payloadLen := msg.PayloadLen()
	magicAndMsgType, err := codec.ReadUint32(in)
	if err != nil {
		return err
	}
	if magicAndMsgType&codec.MagicMask != codec.ProtobufV1Magic {
		return perrors.NewProtocolErrorWithType(perrors.BadVersion, "Bad version in protobuf Unmarshal")
	}
	msgType := magicAndMsgType & codec.FrontMask
	if err := codec.UpdateMsgType(msgType, msg); err != nil {
		return err
	}

	methodName, methodFieldLen, err := codec.ReadString(in)
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("protobuf unmarshal, read method name failed: %s", err.Error()))
	}
	if err = codec.SetOrCheckMethodName(methodName, msg); err != nil && msgType != uint32(remote.Exception) {
		return err
	}
	seqID, err := codec.ReadUint32(in)
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("protobuf unmarshal, read seqID failed: %s", err.Error()))
	}
	if err = codec.SetOrCheckSeqID(int32(seqID), msg); err != nil && msgType != uint32(remote.Exception) {
		return err
	}
	actualMsgLen := payloadLen - metaInfoFixLen - methodFieldLen
	transBuff, err := in.Next(actualMsgLen)
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("protobuf unmarshal, read message buffer failed: %s", err.Error()))
	}

	if err = codec.NewDataIfNeeded(methodName, msg); err != nil {
		return err
	}
	data := msg.Data()

	if err = codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	if msg.RPCRole() == remote.Server {
		gArg := data.(*Args)
		gArg.Method = msg.RPCInfo().Invocation().MethodName()
		gArg.Request = transBuff
	} else {
		gResult := data.(*Result)
		gResult.Success = transBuff
	}
	return nil
}

func (c *binaryProtobufCodec) Name() string {
	return "RawProtobufBinary"
}
