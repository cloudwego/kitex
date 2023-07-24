/*
 * Copyright 2023 CloudWeGo Authors
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
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/bytedance/gopkg/lang/mcache"

	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf/encoding"

	"github.com/cloudwego/kitex/pkg/remote"
)

func buildGRPCFrame(ctx context.Context, payload []byte) ([]byte, error) {
	data, isCompressed, er := compress(ctx, payload)
	if er != nil {
		return nil, er
	}
	header := mcache.Malloc(dataFrameHeaderLen)
	if isCompressed {
		header[0] = 1
	} else {
		header[0] = 0
	}
	binary.BigEndian.PutUint32(header[1:dataFrameHeaderLen], uint32(len(data)))
	frame := make([]byte, dataFrameHeaderLen+len(data))
	copy(frame, header)
	copy(frame[dataFrameHeaderLen:], data)
	return frame, nil
}

func decodeGRPCFrame(ctx context.Context, in remote.ByteBuffer) ([]byte, error) {
	hdr, err := in.Next(5)
	if err != nil {
		return nil, err
	}
	compressFlag := hdr[0]
	dLen := int(binary.BigEndian.Uint32(hdr[1:]))
	d, err := in.Next(dLen)
	if err != nil {
		return nil, err
	}
	if compressFlag == 1 {
		return decompress(ctx, d)
	}
	return d, nil
}

func compress(ctx context.Context, data []byte) ([]byte, bool, error) {
	cname := remote.GetSendCompressor(ctx)
	if cname == "" {
		return data, false, nil
	}
	compressor := encoding.GetCompressor(cname)
	if compressor == nil {
		return nil, false, fmt.Errorf("no compressor registered for: %s", cname)
	}
	cbuf := &bytes.Buffer{}
	z, err := compressor.Compress(cbuf)
	if err != nil {
		return nil, false, err
	}
	if _, err = z.Write(data); err != nil {
		return nil, false, err
	}
	if err = z.Close(); err != nil {
		return nil, false, err
	}
	return cbuf.Bytes(), true, nil
}

func decompress(ctx context.Context, data []byte) ([]byte, error) {
	cname := remote.GetSendCompressor(ctx)
	compressor := encoding.GetCompressor(cname)
	if compressor == nil {
		return nil, fmt.Errorf("no compressor registered found for:%v", cname)
	}
	dcReader, er := compressor.Decompress(bytes.NewReader(data))
	if er != nil {
		return nil, er
	}
	var buf bytes.Buffer
	_, err := io.Copy(&buf, dcReader)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
