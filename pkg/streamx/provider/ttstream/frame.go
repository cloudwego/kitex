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
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/gopkg/bufiox"
	gopkgthrift "github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/terrors"
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

// Frame define a TTHeader Streaming Frame
type Frame struct {
	streamFrame
	typ     int32
	payload []byte
}

func (f *Frame) String() string {
	return fmt.Sprintf("[sid=%d ftype=%d fmethod=%s]", f.sid, f.typ, f.method)
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
		return terrors.ErrIllegalFrame.WithCause(err)
	}
	if len(fr.payload) > 0 {
		if nw, ok := writer.(gopkgthrift.NocopyWriter); ok {
			err = nw.WriteDirect(fr.payload, 0)
		} else {
			_, err = writer.WriteBinary(fr.payload)
		}
		if err != nil {
			return terrors.ErrTransport.WithCause(err)
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
		if errors.Is(err, io.EOF) {
			return nil, err
		}
		return nil, terrors.ErrIllegalFrame.WithCause(err)
	}
	if dp.Flags&ttheader.HeaderFlagsStreaming == 0 {
		return nil, terrors.ErrIllegalFrame.WithCause(fmt.Errorf("unexpected header flags: %d", dp.Flags))
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
		return nil, terrors.ErrIllegalFrame.WithCause(fmt.Errorf("unexpected frame type: %v", dp.IntInfo[ttheader.FrameType]))
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
			return nil, terrors.ErrTransport.WithCause(err)
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

func EncodePayload(ctx context.Context, msg any, method string) ([]byte, error) {
	switch msg.(type) {
	case *generic.Args:
		return EncodeGenericPayload(ctx, msg, method)
	default:
		payload := gopkgthrift.FastMarshal(msg.(gopkgthrift.FastCodec))
		return payload, nil
	}
}

func EncodeGenericPayload(ctx context.Context, msg any, method string) (payload []byte, err error) {
	switch m := msg.(type) {
	case *generic.Args:
		if method == "" {
			return nil, terrors.ErrIllegalFrame.WithCause(errors.New("empty methodName in ttheader generic Encode"))
		}
		bw := bufiox.NewBytesWriter(&payload)
		if err = m.Write(ctx, m.Method, bw); err != nil {
			return nil, terrors.ErrInvalidPayload.WithCause(err)
		}
		if err = bw.Flush(); err != nil {
			return nil, terrors.ErrInvalidPayload.WithCause(err)
		}
		return
	default:
		return nil, terrors.ErrInvalidPayload.WithCause(fmt.Errorf("unsupported type: %v", m))
	}
}

func DecodePayload(ctx context.Context, payload []byte, msg any, method string) error {
	switch msg.(type) {
	case *generic.Result:
		return DecodeGenericPayload(ctx, method, payload, msg)
	default:
		return gopkgthrift.FastUnmarshal(payload, msg.(gopkgthrift.FastCodec))
	}
}

func DecodeGenericPayload(ctx context.Context, method string, payload []byte, msg any) error {
	switch m := msg.(type) {
	case *generic.Result:
		if method == "" {
			return terrors.ErrIllegalFrame.WithCause(errors.New("empty methodName in ttheader generic Decode"))
		}
		if err := m.Read(ctx, method, len(payload), remote.NewReaderBuffer(payload)); err != nil {
			return terrors.ErrInvalidPayload.WithCause(err)
		}
		return nil
	default:
		return terrors.ErrInvalidPayload.WithCause(fmt.Errorf("unsupported type: %v", m))
	}
}

func EncodeException(ctx context.Context, method string, seq int32, ex error) ([]byte, error) {
	exception, ok := ex.(gopkgthrift.FastCodec)
	if !ok {
		exception = gopkgthrift.NewApplicationException(remote.InternalError, ex.Error())
	}
	return gopkgthrift.MarshalFastMsg(method, gopkgthrift.EXCEPTION, seq, exception)
}
