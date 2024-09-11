package ttstream

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"

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
	streamMeta
	typ     int32
	payload []byte
}

func newFrame(meta streamMeta, typ int32, payload []byte) Frame {
	return Frame{
		streamMeta: meta,
		typ:        typ,
		payload:    payload,
	}
}

func EncodeFrame(ctx context.Context, writer bufiox.Writer, fr Frame) (err error) {
	param := ttheader.EncodeParam{
		Flags:      ttheader.HeaderFlagsStreaming,
		SeqID:      fr.sid,
		ProtocolID: ttheader.ProtocolIDThriftStruct,
		IntInfo: map[uint16]string{
			ttheader.FrameType: frameTypeToString[fr.typ],
			ttheader.ToMethod:  fr.method,
		},
	}
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
	frtype := dp.IntInfo[ttheader.FrameType]
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
		err = fmt.Errorf("unexpected frame type: %v", dp.IntInfo[ttheader.FrameType])
		return
	}
	// stream meta
	fr.sid = dp.SeqID
	fr.method = dp.IntInfo[ttheader.ToMethod]

	// frame payload
	if dp.PayloadLen == 0 {
		return fr, nil
	}
	log.Printf("DecodeFrame read next start")
	fr.payload = make([]byte, dp.PayloadLen)
	_, err = reader.ReadBinary(fr.payload)
	reader.Release(err)
	log.Printf("DecodeFrame read next: payload=%v", fr.payload)
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
