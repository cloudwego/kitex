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
	"errors"
	"fmt"
	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf/encoding"
	"io"
	"strings"

	"github.com/cloudwego/kitex/pkg/remote"
)

func getSendCompressor(ctx context.Context) (encoding.Compressor, error) {
	return getCompressor(remote.GetSendCompressor(ctx))
}

func decodeGRPCFrame(ctx context.Context, in remote.ByteBuffer) ([]byte, error) {
	compressor, err := getCompressor(remote.GetRecvCompressor(ctx))
	if err != nil {
		return nil, err
	}
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
		if compressor == nil {
			return nil, errors.New("kitex compression algorithm not found")
		}
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
	mcache.Free(data)
	return cbuf.Bytes(), nil
}

func decompress(ctx context.Context, data []byte) ([]byte, error) {
	compressor, err := getCompressor(remote.GetRecvCompressor(ctx))
	if err != nil {
		return nil, err
	}
	if compressor == nil {
		return data, err
	}
	dcReader, er := compressor.Decompress(bytes.NewReader(data))
	if er != nil {
		return nil, er
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, dcReader)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func getCompressor(cname string) (compressor encoding.Compressor, err error) {
	// if cname is empty, it means there's no compressor
	if cname == "" {
		return nil, nil
	}
	// cname can be an array, such as "identity,deflate,gzip", which means there should be at least one compressor registered.
	// found available compressors
	var hasIdentity bool
	for _, name := range strings.Split(cname, ",") {
		if name == encoding.Identity {
			hasIdentity = true
		}
		compressor = encoding.GetCompressor(name)
		if compressor != nil {
			break
		}
	}
	if compressor == nil {
		if hasIdentity {
			return nil, nil
		}
		return nil, fmt.Errorf("no kitex compressor registered found for:%v", cname)
	}
	return compressor, nil
}
