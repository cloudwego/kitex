package ttstream

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/kitex/pkg/remote"
	"log"
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

func EncodeFrame(ctx context.Context, writer remote.ByteBuffer, fr Frame) (err error) {
	// if there is no service in frame header, we should set it first
	if fr.header[ttheader.HeaderIDLServiceName] != fr.service {
		fr.header[ttheader.HeaderIDLServiceName] = fr.service
	}
	param := ttheader.EncodeParam{
		Flags:      ttheader.HeaderFlagsStreaming,
		SeqID:      fr.sid,
		ProtocolID: ttheader.ProtocolIDThriftStruct,
		IntInfo: map[uint16]string{
			ttheader.FrameType: frameTypeToString[fr.typ],
			ttheader.ToMethod:  fr.method,
		},
		StrInfo: fr.header,
	}
	totalLenField, err := ttheader.Encode(ctx, param, writer)
	if err != nil {
		return err
	}
	payload := fr.payload
	if len(payload) == 0 {
		payload = []byte{}
	}
	_, err = writer.WriteBinary(payload)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint32(totalLenField, uint32(writer.WrittenLen()-4))
	err = writer.Flush()
	return err
}

func DecodeFrame(ctx context.Context, reader remote.ByteBuffer) (fr Frame, err error) {
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
	case ttheader.FrameTypeData:
		fr.typ = dataFrameType
	case ttheader.FrameTypeTrailer:
		fr.typ = trailerFrameType
	default:
		err = fmt.Errorf("unexpected frame type: %v", dp.IntInfo[ttheader.FrameType])
		return
	}
	// stream meta
	fr.sid = dp.SeqID
	fr.service = dp.StrInfo[ttheader.HeaderIDLServiceName]
	fr.method = dp.IntInfo[ttheader.ToMethod]
	fr.header = dp.StrInfo

	// frame payload
	log.Printf("payload len: %d", dp.PayloadLen)
	fr.payload, err = reader.Next(dp.PayloadLen)
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
