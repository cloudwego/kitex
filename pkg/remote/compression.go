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

import "context"

// CompressType tells compression type for a message.
type CompressType int32

// Compression types.
// Not support now, compression will be supported at later version
const (
	NoCompress CompressType = iota
	GZip
)

type recvCompressorKey struct{}

type sendCompressorKey struct{}

func SetRecvCompressor(ctx context.Context, compressorName string) context.Context {
	return context.WithValue(ctx, recvCompressorKey{}, compressorName)
}

func SetSendCompressor(ctx context.Context, compressorName string) context.Context {
	return context.WithValue(ctx, sendCompressorKey{}, compressorName)
}

func GetSendCompressor(ctx context.Context) string {
	if v, ok := ctx.Value(sendCompressorKey{}).(string); ok && v != "" {
		return v
	}
	return ""
}

func GetRecvCompressor(ctx context.Context) string {
	if v, ok := ctx.Value(recvCompressorKey{}).(string); ok && v != "" {
		return v
	}
	return ""
}
