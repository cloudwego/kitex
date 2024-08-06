package ttstream

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"
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

type Frame struct {
	streamFrame
	meta    IntHeader
	typ     int32
	payload []byte
}

func newFrame(meta streamFrame, typ int32, payload []byte) Frame {
	return Frame{
		streamFrame: meta,
		typ:         typ,
		payload:     payload,
	}
}

func EncodeFrame(ctx context.Context, writer bufiox.Writer, fr Frame) (err error) {
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
		_, err = writer.WriteBinary(fr.payload)
		if err != nil {
			return err
		}
	}
	binary.BigEndian.PutUint32(totalLenField, uint32(writer.WrittenLen()-4))
	err = writer.Flush()
	return err
}

func DecodeFrame(ctx context.Context, reader bufiox.Reader) (fr Frame, err error) {
	var dp ttheader.DecodeParam
	dp, err = ttheader.Decode(ctx, reader)
	if err != nil {
		return
	}
	if dp.Flags != ttheader.HeaderFlagsStreaming {
		err = fmt.Errorf("unexpected header flags: %d", dp.Flags)
		return
	}

	fr.meta = dp.IntInfo
	frtype := fr.meta[ttheader.FrameType]
	switch frtype {
	case ttheader.FrameTypeMeta:
		fr.typ = metaFrameType
	case ttheader.FrameTypeHeader:
		fr.typ = headerFrameType
		fr.header = dp.StrInfo
	case ttheader.FrameTypeData:
		fr.typ = dataFrameType
	case ttheader.FrameTypeTrailer:
		fr.typ = trailerFrameType
		fr.trailer = dp.StrInfo
	default:
		err = fmt.Errorf("unexpected frame type: %v", fr.meta[ttheader.FrameType])
		return
	}
	// stream meta
	fr.sid = dp.SeqID
	fr.method = fr.meta[ttheader.ToMethod]

	// frame payload
	if dp.PayloadLen == 0 {
		return fr, nil
	}
	fr.payload = make([]byte, dp.PayloadLen)
	_, err = reader.ReadBinary(fr.payload)
	reader.Release(err)
	if err != nil {
		return
	}
	return fr, nil
}

func EncodePayload(ctx context.Context, msg any) ([]byte, error) {
	return thrift.FastMarshal(msg.(thrift.FastCodec)), nil
}

func DecodePayload(ctx context.Context, payload []byte, msg any) error {
	err := thrift.FastUnmarshal(payload, msg.(thrift.FastCodec))
	return err
}
