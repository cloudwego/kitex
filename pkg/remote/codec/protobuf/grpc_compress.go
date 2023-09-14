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

	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf/encoding"

	"github.com/cloudwego/kitex/pkg/remote"
)

func getCompressor(ctx context.Context) (encoding.Compressor, error) {
	cname := remote.GetSendCompressor(ctx)
	if cname == "" {
		// if no compressor set, don't use compressor
		return nil, nil
	}
	compressor := encoding.GetCompressor(cname)
	if compressor == nil {
		// if comporessor set but not found, return err
		return nil, fmt.Errorf("no compressor registered for: %s", cname)
	}
	return compressor, nil
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

func compress(compressor encoding.Compressor, data []byte) ([]byte, error) {
	cbuf := &bytes.Buffer{}
	z, err := compressor.Compress(cbuf)
	if err != nil {
		return nil, err
	}
	if _, err = z.Write(data); err != nil {
		return nil, err
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	return cbuf.Bytes(), nil
}

func decompress(ctx context.Context, data []byte) ([]byte, error) {
	cname := remote.GetRecvCompressor(ctx)
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
