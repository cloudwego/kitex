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

package protobuf

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/fastpb"
	"github.com/cloudwego/kitex/pkg/remote"
	"google.golang.org/grpc/encoding"
	"google.golang.org/protobuf/proto"
	"io"
)

const dataFrameHeaderLen = 5

// gogoproto generate
type marshaler interface {
	MarshalTo(data []byte) (n int, err error)
	Size() int
}

type protobufV2MsgCodec interface {
	XXX_Unmarshal(b []byte) error
	XXX_Marshal(b []byte, deterministic bool) ([]byte, error)
}

type grpcCodec struct{}

// NewGRPCCodec create grpc and protobuf codec
func NewGRPCCodec() remote.Codec {
	return new(grpcCodec)
}

func mallocWithFirstByteZeroed(size int) []byte {
	data := mcache.Malloc(size)
	data[0] = 0 // compressed flag = false
	return data
}

func doGzip(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}
	writer.Close()
	compressedData, err := io.ReadAll(&buffer)
	return compressedData, nil
}

// todo 实现自己的注册模块
func EncodingCompress(ctx context.Context, data []byte) ([]byte, bool, error) {

	compressorName, ok := ctx.Value(sendCompressorKey{}).(string)
	if ok && compressorName != "" {
		compressor := encoding.GetCompressor(compressorName)
		if compressor == nil {
			return nil, false, errors.New("no compressor registered for:" + compressorName)
		}
		cd, err := doGzip(data)
		return cd, true, err
	}
	return nil, false, nil
}

func GetSendCompressor(ctx context.Context) string {
	compressorName, ok := ctx.Value(sendCompressorKey{}).(string)
	if ok && compressorName != "" {
		return compressorName
	}
	return ""
}

func (c *grpcCodec) Encode(ctx context.Context, message remote.Message, out remote.ByteBuffer) (err error) {
	writer, ok := out.(remote.FrameWrite)
	if !ok {
		return fmt.Errorf("output buffer must implement FrameWrite")
	}

	var data []byte
	switch t := message.Data().(type) {
	case fastpb.Writer:
		// TODO: reuse data buffer when we can free it safely
		data = mcache.Malloc(t.Size())
		t.FastWrite(data)
		data, doCompress, er := EncodingCompress(ctx, data)
		if er != nil {
			return er
		}
		compressedData := mcache.Malloc(dataFrameHeaderLen)
		if doCompress {
			compressedData[0] = 1
		} else {
			compressedData[0] = 0
		}
		binary.BigEndian.PutUint32(compressedData[1:dataFrameHeaderLen], uint32(len(data)))
		compressedData = append(compressedData, data...)

		return writer.WriteData(compressedData)
	case marshaler:
		// TODO: reuse data buffer when we can free it safely
		size := t.Size()
		data = mallocWithFirstByteZeroed(size + dataFrameHeaderLen)
		if _, err = t.MarshalTo(data[dataFrameHeaderLen:]); err != nil {
			return err
		}
		binary.BigEndian.PutUint32(data[1:dataFrameHeaderLen], uint32(size))
		_, _, er := EncodingCompress(ctx, data)
		if er != nil {
			return er
		}
		return writer.WriteData(data)
	case protobufV2MsgCodec:
		data, err = t.XXX_Marshal(nil, true)
	case proto.Message:
		data, err = proto.Marshal(t)
	case protobufMsgCodec:
		data, err = t.Marshal(nil)
	}
	if err != nil {
		return err
	}
	if err = writer.WriteData(data); err != nil {
		return err
	}
	var header [5]byte
	binary.BigEndian.PutUint32(header[1:5], uint32(len(data)))
	return writer.WriteHeader(header[:])
}

type recvCompressorKey struct {
}
type sendCompressorKey struct {
}

func SetRecvCompressor(ctx context.Context, compressorName string) context.Context {
	return context.WithValue(ctx, recvCompressorKey{}, compressorName)
}

func SetSendCompressor(ctx context.Context, compressorName string) context.Context {
	return context.WithValue(ctx, sendCompressorKey{}, compressorName)
}

func getCompressor(ctx context.Context, key interface{}) (encoding.Compressor, error) {
	compressorName, ok := ctx.Value(key).(string)
	if !ok || compressorName == "" {
		return nil, errors.New("no compressor info from stream context")
	}
	compressor := encoding.GetCompressor(compressorName)
	if compressor == nil {
		return nil, errors.New("no compressor registered for:" + compressorName)
	}
	return compressor, nil
}

func (c *grpcCodec) Decode(ctx context.Context, message remote.Message, in remote.ByteBuffer) (err error) {
	hdr, err := in.Next(5)
	if err != nil {
		return err
	}
	compressFlag := hdr[0]
	dLen := int(binary.BigEndian.Uint32(hdr[1:]))
	d, err := in.Next(dLen)
	if err != nil {
		return err
	}
	if compressFlag == 1 {
		compressor, er := getCompressor(ctx, recvCompressorKey{})
		if er != nil {
			return er
		}
		dcReader, er := compressor.Decompress(bytes.NewReader(d))
		if er != nil {
			return er
		}
		d, er = io.ReadAll(dcReader)
		if er != nil {
			return er
		}
	}
	message.SetPayloadLen(dLen)
	data := message.Data()
	if t, ok := data.(fastpb.Reader); ok {
		if len(d) == 0 {
			// if all fields of a struct is default value, data will be nil
			// In the implementation of fastpb, if data is nil, then fastpb will skip creating this struct, as a result user will get a nil pointer which is not expected.
			// So, when data is nil, use default protobuf unmarshal method to decode the struct.
			// todo: fix fastpb
		} else {
			_, err = fastpb.ReadMessage(d, fastpb.SkipTypeCheck, t)
			return err
		}
	}
	switch t := data.(type) {
	case protobufV2MsgCodec:
		return t.XXX_Unmarshal(d)
	case proto.Message:
		return proto.Unmarshal(d, t)
	case protobufMsgCodec:
		return t.Unmarshal(d)
	}
	return nil
}

func (c *grpcCodec) Name() string {
	return "grpc"
}
