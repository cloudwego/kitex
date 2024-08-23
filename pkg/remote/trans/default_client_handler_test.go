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

package trans

import (
	"context"
	"net"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/connpool"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func TestDefaultCliTransHandler(t *testing.T) {
	conn := mocks.NewIOConn()
	ext := &MockExtension{
		NewWriteByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) bufiox.Writer {
			return bufiox.NewDefaultWriter(conn)
		},
		NewReadByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) bufiox.Reader {
			return bufiox.NewDefaultReader(conn)
		},
	}

	tagEncode, tagDecode := 0, 0
	opt := &remote.ClientOption{
		Codec: &MockCodec{
			EncodeFunc: func(ctx context.Context, msg remote.Message, out bufiox.Writer) error {
				tagEncode++
				_, err := out.WriteBinary([]byte("hello world"))
				return err
			},
			DecodeFunc: func(ctx context.Context, msg remote.Message, in bufiox.Reader) error {
				tagDecode++
				hw := make([]byte, 11)
				_, err := in.ReadBinary(hw)
				if err != nil {
					return err
				}
				test.Assert(t, string(hw) == "hello world")
				return nil
			},
		},
		ConnPool: connpool.NewShortPool("opt"),
	}

	handler, err := NewDefaultCliTransHandler(opt, ext)
	test.Assert(t, err == nil)

	ctx := context.Background()
	msg := &MockMessage{
		RPCInfoFunc: func() rpcinfo.RPCInfo {
			return newMockRPCInfo()
		},
	}
	ctx, err = handler.Write(ctx, conn, msg)
	test.Assert(t, ctx != nil, ctx)
	test.Assert(t, err == nil, err)
	test.Assert(t, tagEncode == 1, tagEncode)
	test.Assert(t, tagDecode == 0, tagDecode)

	ctx, err = handler.Read(ctx, conn, msg)
	test.Assert(t, ctx != nil, ctx)
	test.Assert(t, err == nil, err)
	test.Assert(t, tagEncode == 1, tagEncode)
	test.Assert(t, tagDecode == 1, tagDecode)
}
