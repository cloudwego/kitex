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
	"encoding/binary"
	"fmt"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

var _ remote.PayloadCodec = &binaryPbCodec{}

// TODO: common
const dataFrameHeaderLen int = 5

type binaryPbCodec struct {
	pbCodec remote.PayloadCodec
}

func (c *binaryPbCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	fmt.Println("GENERIC MARSHAL")
	// TODO: 共通部分抽出 (binaryCodec)
	data := msg.Data()
	if data == nil {
		return perrors.NewProtocolErrorWithMsg("invalid marshal data in binaryGRPCCodec: nil")
	}
	// TODO: is there remote.Exception for grpc?
	//if msg.MessageType() == remote.Exception {
	//	if err := c.pbCodec.Marshal(ctx, msg, out); err != nil {
	//		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("binaryGRPCCodec Marshal exception failed, err: %s", err.Error()))
	//	}
	//	return nil
	//}
	var transBuff []byte
	var ok bool
	//if msg.RPCRole() == remote.Server {
	//	gResult := data.(*Result)
	//	transBinary := gResult.Success
	//	if transBuff, ok = transBinary.(binaryReqType); !ok {
	//		return perrors.NewProtocolErrorWithMsg("invalid marshal result in binaryGRPCCodec: must be []byte")
	//	}
	//} else {
	// TODO: only client now. will implement svr later
	gArg := data.(*Args)
	transBinary := gArg.Request
	if transBuff, ok = transBinary.(binaryReqType); !ok {
		return perrors.NewProtocolErrorWithMsg("invalid marshal request in binaryGRPCCodec: must be []byte")
	}
	//if err := SetSeqID(msg.RPCInfo().Invocation().SeqID(), transBuff); err != nil {
	//	return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("binaryGRPCCodec set seqID failed, err: %s", err.Error()))
	//}
	writer, ok := out.(remote.FrameWrite)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("output buffer must implement FrameWrite")
	}
	writer.WriteHeader(createHeader(len(transBuff)))
	writer.WriteData(transBuff)
	return nil
}

func createHeader(payloadLen int) []byte {
	var header [dataFrameHeaderLen]byte
	header[0] = 0
	binary.BigEndian.PutUint32(header[1:dataFrameHeaderLen], uint32(payloadLen))
	return header[:]
}

func (c *binaryPbCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	//buf, err := in.Next(codec.Size32)
	//if err != nil {
	//	return err
	//}
	//magicAndMsgType := binary.BigEndian.Uint32(buf)
	//// TODO: should implement Peek() etc.?
	////magicAndMsgType, err := codec.PeekUint32(in)
	////if err != nil {
	////	return err
	////}
	//msgType := magicAndMsgType & codec.FrontMask
	//if msgType == uint32(remote.Exception) {
	//	return c.pbCodec.Unmarshal(ctx, msg, in)
	//}
	//payloadLen := msg.PayloadLen()
	hdr, err := in.Next(5) // TODO: header len
	payloadLen := int(binary.BigEndian.Uint32(hdr[1:]))
	if err != nil {
		return err
	}
	fmt.Println("payloadLen", payloadLen)

	transBuff, err := in.Next(payloadLen)
	//transBuff, err := in.ReadBinary(payloadLen)
	if err != nil {
		return err
	}
	//if err := readBinaryMethod(transBuff, msg); err != nil {
	//	return err
	//}
	//
	//if err = codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
	//	return err
	//}
	data := msg.Data()
	//if msg.RPCRole() == remote.Server {
	//	gArg := data.(*Args)
	//	gArg.Method = msg.RPCInfo().Invocation().MethodName()
	//	gArg.Request = transBuff
	//} else {
	// TODO: only client now. will implement svr later
	gResult := data.(*Result)
	gResult.Success = transBuff
	//}
	return nil
}

func (c *binaryPbCodec) Name() string {
	return "RawGRPCBinary"
}
