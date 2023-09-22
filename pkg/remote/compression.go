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

package remote

import (
	"context"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// CompressType tells compression type for a message.
type CompressType int32

// Compression types.
// Not support now, compression will be supported at later version
const (
	NoCompress CompressType = iota
	GZip
)

func SetRecvCompressor(ctx context.Context, compressorName string) {
	ri := rpcinfo.GetRPCInfo(ctx)
	rpcinfo.AsMutableEndpointInfo(ri.From()).SetTag("recv-compressor", compressorName)
}

func SetSendCompressor(ctx context.Context, compressorName string) {
	ri := rpcinfo.GetRPCInfo(ctx)
	rpcinfo.AsMutableEndpointInfo(ri.From()).SetTag("send-compressor", compressorName)
}

func GetSendCompressor(ctx context.Context) string {
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil {
		return ""
	}
	v, exist := ri.From().Tag("send-compressor")
	if exist {
		return v
	}
	return ""
}

func GetRecvCompressor(ctx context.Context) string {
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil {
		return ""
	}
	v, exist := ri.From().Tag("recv-compressor")
	if exist {
		return v
	}
	return ""
}
