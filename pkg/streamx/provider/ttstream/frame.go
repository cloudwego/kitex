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

package ttstream

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/gopkg/bufiox"
	gopkgthrift "github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/pkg/streamx"
)

const (
	metaFrameType    int32 = 1
	headerFrameType  int32 = 2
	dataFrameType    int32 = 3
	trailerFrameType int32 = 4
)

var frameTypeToString = map[int32]string{
	metaFrameType:    ttheader.FrameTypeMeta,
	headerFrameType:  ttheader.FrameTypeHeader,
	dataFrameType:    ttheader.FrameTypeData,
	trailerFrameType: ttheader.FrameTypeTrailer,
}

var framePool sync.Pool

type Frame struct {
	streamFrame
	typ     int32
	payload []byte
}

func newFrame(sframe streamFrame, typ int32, payload []byte) (fr *Frame) {
	v := framePool.Get()
	if v == nil {
		fr = new(Frame)
	} else {
		fr = v.(*Frame)
	}
	fr.streamFrame = sframe
	fr.typ = typ
	fr.payload = payload
	return fr
}

func recycleFrame(frame *Frame) {
	frame.streamFrame = streamFrame{}
	frame.typ = 0
	frame.payload = nil
	framePool.Put(frame)
}

// EncodeFrame will not call Flush!
func EncodeFrame(ctx context.Context, writer bufiox.Writer, fr *Frame) (err error) {
	written := writer.WrittenLen()
	param := ttheader.EncodeParam{
		Flags:      ttheader.HeaderFlagsStreaming,
		SeqID:      fr.sid,
		ProtocolID: ttheader.ProtocolIDThriftStruct,
	}

	param.IntInfo = fr.meta
	if param.IntInfo == nil {
		param.IntInfo = make(IntHeader)
	}
	param.IntInfo[ttheader.FrameType] = frameTypeToString[fr.typ]
	param.IntInfo[ttheader.ToMethod] = fr.method

	switch fr.typ {
	case headerFrameType:
		param.StrInfo = fr.header
	case trailerFrameType:
		param.StrInfo = fr.trailer
	}

	totalLenField, err := ttheader.Encode(ctx, param, writer)
	if err != nil {
		return err
	}
	if len(fr.payload) > 0 {
		if nw, ok := writer.(gopkgthrift.NocopyWriter); ok {
			err = nw.WriteDirect(fr.payload, 0)
		} else {
			_, err = writer.WriteBinary(fr.payload)
		}
		if err != nil {
			return err
		}
	}
	written = writer.WrittenLen() - written
	binary.BigEndian.PutUint32(totalLenField, uint32(written-4))
	return nil
}

func DecodeFrame(ctx context.Context, reader bufiox.Reader) (fr *Frame, err error) {
	var dp ttheader.DecodeParam
	dp, err = ttheader.Decode(ctx, reader)
	if err != nil {
		return
	}
	if dp.Flags&ttheader.HeaderFlagsStreaming == 0 {
		err = fmt.Errorf("unexpected header flags: %d", dp.Flags)
		return
	}

	var ftype int32
	var fheader streamx.Header
	var ftrailer streamx.Trailer
	fmeta := dp.IntInfo
	switch dp.IntInfo[ttheader.FrameType] {
	case ttheader.FrameTypeMeta:
		ftype = metaFrameType
	case ttheader.FrameTypeHeader:
		ftype = headerFrameType
		fheader = dp.StrInfo
	case ttheader.FrameTypeData:
		ftype = dataFrameType
	case ttheader.FrameTypeTrailer:
		ftype = trailerFrameType
		ftrailer = dp.StrInfo
	default:
		err = fmt.Errorf("unexpected frame type: %v", dp.IntInfo[ttheader.FrameType])
		return
	}
	fmethod := dp.IntInfo[ttheader.ToMethod]
	fsid := dp.SeqID

	// frame payload
	var fpayload []byte
	if dp.PayloadLen > 0 {
		fpayload = mcache.Malloc(dp.PayloadLen)
		_, err = reader.ReadBinary(fpayload) // copy read
		_ = reader.Release(err)
		if err != nil {
			return
		}
	} else {
		_ = reader.Release(nil)
	}

	fr = newFrame(
		streamFrame{sid: fsid, method: fmethod, meta: fmeta, header: fheader, trailer: ftrailer},
		ftype, fpayload,
	)
	return fr, nil
}

func EncodePayload(ctx context.Context, msg any) ([]byte, error) {
	payload := gopkgthrift.FastMarshal(msg.(gopkgthrift.FastCodec))
	return payload, nil
}

func DecodePayload(ctx context.Context, payload []byte, msg any) error {
	return gopkgthrift.FastUnmarshal(payload, msg.(gopkgthrift.FastCodec))
}

func EncodeException(ctx context.Context, method string, seq int32, ex tException) ([]byte, error) {
	return gopkgthrift.MarshalFastMsg(method, gopkgthrift.EXCEPTION, seq, ex.(gopkgthrift.FastCodec))
}
