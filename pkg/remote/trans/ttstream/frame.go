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
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

const (
	metaFrameType    int32 = 1
	headerFrameType  int32 = 2
	dataFrameType    int32 = 3
	trailerFrameType int32 = 4
	rstFrameType     int32 = 5
)

var frameTypeToString = map[int32]string{
	metaFrameType:    ttheader.FrameTypeMeta,
	headerFrameType:  ttheader.FrameTypeHeader,
	dataFrameType:    ttheader.FrameTypeData,
	trailerFrameType: ttheader.FrameTypeTrailer,
	rstFrameType:     ttheader.FrameTypeRst,
}

var framePool sync.Pool

var (
	errNoRPCInfo      = errors.New("no rpcinfo in context")
	errInvalidMessage = errors.New("ttheaderstreaming invalid message")
)

// Frame define a TTHeader Streaming Frame
type Frame struct {
	streamFrame
	typ            int32
	payload        []byte
	recyclePayload bool
}

func (f *Frame) String() string {
	return fmt.Sprintf("[sid=%d ftype=%d fmethod=%s]", f.sid, f.typ, f.method)
}

func newFrame(sframe streamFrame, typ int32, payload []byte, recyclePayload bool) (fr *Frame) {
	v := framePool.Get()
	if v == nil {
		fr = new(Frame)
	} else {
		fr = v.(*Frame)
	}
	fr.streamFrame = sframe
	fr.typ = typ
	fr.payload = payload
	fr.recyclePayload = recyclePayload
	return fr
}

func recycleFrame(frame *Frame) {
	frame.streamFrame = streamFrame{}
	frame.typ = 0
	if frame.recyclePayload {
		mcache.Free(frame.payload)
		frame.recyclePayload = false
	}
	frame.payload = nil
	frame.protocolID = 0
	framePool.Put(frame)
}

// EncodeFrame will not call Flush!
func EncodeFrame(ctx context.Context, writer bufiox.Writer, fr *Frame) (err error) {
	written := writer.WrittenLen()
	param := ttheader.EncodeParam{
		Flags:      ttheader.HeaderFlagsStreaming,
		SeqID:      fr.sid,
		ProtocolID: fr.protocolID,
	}

	param.IntInfo = fr.meta
	if param.IntInfo == nil {
		param.IntInfo = make(IntHeader)
	}
	param.IntInfo[ttheader.FrameType] = frameTypeToString[fr.typ]
	param.IntInfo[ttheader.ToMethod] = fr.method

	switch fr.typ {
	case headerFrameType, metaFrameType, rstFrameType:
		param.StrInfo = fr.header
	case trailerFrameType:
		param.StrInfo = fr.trailer
	}

	totalLenField, err := ttheader.Encode(ctx, param, writer)
	if err != nil {
		return errIllegalFrame.newBuilder().withCause(err)
	}
	if len(fr.payload) > 0 {
		if nw, ok := writer.(gopkgthrift.NocopyWriter); ok {
			err = nw.WriteDirect(fr.payload, 0)
		} else {
			_, err = writer.WriteBinary(fr.payload)
		}
		if err != nil {
			return errTransport.newBuilder().withCause(err)
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
		return nil, errIllegalFrame.newBuilder().withCause(err)
	}
	if dp.Flags&ttheader.HeaderFlagsStreaming == 0 {
		return nil, errIllegalFrame.newBuilder().withCause(fmt.Errorf("unexpected header flags: %d", dp.Flags))
	}

	var ftype int32
	var fheader streaming.Header
	var ftrailer streaming.Trailer
	fmeta := dp.IntInfo
	switch dp.IntInfo[ttheader.FrameType] {
	case ttheader.FrameTypeMeta:
		ftype = metaFrameType
		fheader = dp.StrInfo
	case ttheader.FrameTypeHeader:
		ftype = headerFrameType
		fheader = dp.StrInfo
	case ttheader.FrameTypeData:
		ftype = dataFrameType
	case ttheader.FrameTypeTrailer:
		ftype = trailerFrameType
		ftrailer = dp.StrInfo
	case ttheader.FrameTypeRst:
		ftype = rstFrameType
		fheader = dp.StrInfo
	default:
		return nil, errIllegalFrame.newBuilder().withCause(fmt.Errorf("unexpected frame type: %v", dp.IntInfo[ttheader.FrameType]))
	}
	fmethod := dp.IntInfo[ttheader.ToMethod]
	fsid := dp.SeqID
	protocolID := dp.ProtocolID

	// frame payload
	var fpayload []byte
	if dp.PayloadLen > 0 {
		fpayload = mcache.Malloc(dp.PayloadLen)
		_, err = reader.ReadBinary(fpayload) // copy read
		_ = reader.Release(err)
		if err != nil {
			return nil, errTransport.newBuilder().withCause(err)
		}
	} else {
		_ = reader.Release(nil)
	}

	fr = newFrame(
		streamFrame{sid: fsid, method: fmethod, meta: fmeta, header: fheader, trailer: ftrailer, protocolID: protocolID},
		ftype, fpayload, false,
	)
	return fr, nil
}

func EncodePayload(ctx context.Context, msg any) ([]byte, error) {
	switch t := msg.(type) {
	case gopkgthrift.FastCodec:
		return gopkgthrift.FastMarshal(t), nil
	case *generic.Args:
		ri := rpcinfo.GetRPCInfo(ctx)
		if ri == nil {
			return nil, errNoRPCInfo
		}
		methodName := ri.Invocation().MethodName()
		var buf []byte
		w := bufiox.NewBytesWriter(&buf)
		err := t.Write(ctx, methodName, w)
		if err != nil {
			return nil, err
		}
		w.Flush()
		return buf, nil
	case *generic.Result:
		ri := rpcinfo.GetRPCInfo(ctx)
		if ri == nil {
			return nil, errNoRPCInfo
		}
		methodName := ri.Invocation().MethodName()
		var buf []byte
		w := bufiox.NewBytesWriter(&buf)
		err := t.Write(ctx, methodName, w)
		if err != nil {
			return nil, err
		}
		w.Flush()
		return buf, nil
	default:
		return nil, errInvalidMessage
	}
}

func DecodePayload(ctx context.Context, payload []byte, msg any) error {
	switch t := msg.(type) {
	case gopkgthrift.FastCodec:
		return gopkgthrift.FastUnmarshal(payload, msg.(gopkgthrift.FastCodec))
	case *generic.Args:
		ri := rpcinfo.GetRPCInfo(ctx)
		if ri == nil {
			return errNoRPCInfo
		}
		methodName := ri.Invocation().MethodName()
		r := bufiox.NewBytesReader(payload)
		return t.Read(ctx, methodName, len(payload), r)
	case *generic.Result:
		ri := rpcinfo.GetRPCInfo(ctx)
		if ri == nil {
			return errNoRPCInfo
		}
		methodName := ri.Invocation().MethodName()
		r := bufiox.NewBytesReader(payload)
		return t.Read(ctx, methodName, len(payload), r)
	default:
		return errInvalidMessage
	}
}

func EncodeException(ctx context.Context, method string, seq int32, ex error) ([]byte, error) {
	var appEx gopkgthrift.FastCodec
	switch et := ex.(type) {
	case *Exception:
		appEx = gopkgthrift.NewApplicationException(et.TypeId(), et.getMessage())
	case gopkgthrift.FastCodec:
		appEx = et
	default:
		appEx = gopkgthrift.NewApplicationException(remote.InternalError, ex.Error())
	}

	return gopkgthrift.MarshalFastMsg(method, gopkgthrift.EXCEPTION, seq, appEx)
}

func getThriftMessageTypeStr(typ gopkgthrift.TMessageType) string {
	switch typ {
	case gopkgthrift.INVALID_TMESSAGE_TYPE:
		return "InvalidTMessageType"
	case gopkgthrift.CALL:
		return "Call"
	case gopkgthrift.REPLY:
		return "Reply"
	case gopkgthrift.EXCEPTION:
		return "Exception"
	case gopkgthrift.ONEWAY:
		return "Oneway"
	default:
		return fmt.Sprintf("Unknown ThriftMessageType: %d", typ)
	}
}

func decodeException(buf []byte) (*gopkgthrift.ApplicationException, error) {
	_, msgType, _, i, err := gopkgthrift.Binary.ReadMessageBegin(buf)
	if err != nil {
		return nil, err
	}
	if msgType != gopkgthrift.EXCEPTION {
		return nil, fmt.Errorf("thrift message type want Exception, but got %s", getThriftMessageTypeStr(msgType))
	}
	buf = buf[i:]

	ex := gopkgthrift.NewApplicationException(gopkgthrift.UNKNOWN_APPLICATION_EXCEPTION, "")
	_, err = ex.FastRead(buf)
	if err != nil {
		return nil, err
	}
	return ex, nil
}
