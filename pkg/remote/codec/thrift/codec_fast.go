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

package thrift

import (
	"fmt"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"

	"github.com/cloudwego/kitex/internal/utils/safemcache"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

const (
	adaptiveFastMarshalMinLength      = 16 << 10
	adaptiveFastMarshalMinDirectCount = 32
	adaptiveFastMarshalMinDirectBytes = 256 << 10
)

type adaptiveNocopyInfo struct {
	Length      int
	DirectCount int
	DirectBytes int
}

type streamingFastCodec interface {
	thrift.FastCodec
	FastNocopyInfo() (length int, directCount int, directBytes int)
	FastWriteTo(out bufiox.Writer) error
}

// ThriftMsgFastCodec ...
// Deprecated: use `github.com/cloudwego/gopkg/protocol/thrift.FastCodec`
type ThriftMsgFastCodec = thrift.FastCodec

func fastCodecAvailable(data interface{}) bool {
	_, ok := data.(thrift.FastCodec)
	return ok
}

// encodeFastThrift encode with the FastCodec way
func fastMarshal(out bufiox.Writer, methodName string, msgType remote.MessageType, seqID int32, msg thrift.FastCodec) error {
	nw, _ := out.(remote.NocopyWrite)
	if nw != nil {
		if smsg, ok := msg.(streamingFastCodec); ok {
			length, directCount, directBytes := smsg.FastNocopyInfo()
			info := adaptiveNocopyInfo{
				Length:      length,
				DirectCount: directCount,
				DirectBytes: directBytes,
			}
			if shouldUseStreamingFastMarshal(info) {
				return streamingFastMarshal(out, methodName, msgType, seqID, smsg)
			}
			payloadLen := info.Length
			if payloadLen <= 0 {
				payloadLen = msg.BLength()
			}
			return legacyFastMarshal(out, nw, methodName, msgType, seqID, msg, payloadLen)
		}
	}
	return legacyFastMarshal(out, nw, methodName, msgType, seqID, msg, msg.BLength())
}

func shouldUseStreamingFastMarshal(info adaptiveNocopyInfo) bool {
	return info.Length >= adaptiveFastMarshalMinLength &&
		info.DirectCount >= adaptiveFastMarshalMinDirectCount &&
		info.DirectBytes >= adaptiveFastMarshalMinDirectBytes
}

func legacyFastMarshal(out bufiox.Writer, nw remote.NocopyWrite, methodName string, msgType remote.MessageType, seqID int32, msg thrift.FastCodec, payloadLen int) error {
	// nocopy write is a special implementation of linked buffer, only bytebuffer implement NocopyWrite do FastWrite
	msgBeginLen := thrift.Binary.MessageBeginLength(methodName)
	buf, err := out.Malloc(msgBeginLen + payloadLen)
	if err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, Malloc failed: %s", err.Error()))
	}
	// If fast write enabled, the underlying buffer maybe large than the correct buffer,
	// so we need to save the mallocLen before fast write and correct the real mallocLen after codec
	mallocLen := out.WrittenLen()
	offset := thrift.Binary.WriteMessageBegin(buf, methodName, thrift.TMessageType(msgType), seqID)
	_ = msg.FastWriteNocopy(buf[offset:], nw)
	if nw == nil {
		// if nw is nil, FastWrite will act in Copy mode.
		return nil
	}
	return nw.MallocAck(mallocLen)
}

func streamingFastMarshal(out bufiox.Writer, methodName string, msgType remote.MessageType, seqID int32, msg streamingFastCodec) error {
	buf, err := out.Malloc(thrift.Binary.MessageBeginLength(methodName))
	if err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, write message begin failed: %s", err.Error()))
	}
	thrift.Binary.WriteMessageBegin(buf, methodName, thrift.TMessageType(msgType), seqID)
	if err := msg.FastWriteTo(out); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, fast write failed: %s", err.Error()))
	}
	return nil
}

func fastUnmarshal(trans bufiox.Reader, data interface{}, dataLen int) error {
	msg := data.(thrift.FastCodec)
	if dataLen > 0 {
		buf, err := trans.Next(dataLen)
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		_, err = msg.FastRead(buf)
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		return nil
	}
	buf, err := skipThriftStruct(trans)
	if err != nil {
		return err
	}
	_, err = msg.FastRead(buf)
	if err != nil {
		return remote.NewTransError(remote.ProtocolError, err).AppendMessage("caught in FastCodec using SkipDecoder Buffer")
	}
	return err
}

// NOTE: only used by `marshalThriftData`
func fastMarshalData(data interface{}) (buf []byte, err error) {
	msg := data.(thrift.FastCodec)
	payloadSize := msg.BLength()
	payload := safemcache.Malloc(payloadSize)
	msg.FastWriteNocopy(payload, nil)
	return payload, nil
}
