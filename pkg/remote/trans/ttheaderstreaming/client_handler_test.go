/*
 * Copyright 2024 CloudWeGo Authors
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

package ttheaderstreaming

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/ttheader"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

func Test_getClientFrameMetaHandler(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		h := getClientFrameMetaHandler(nil)
		_, ok := h.(*clientTTHeaderFrameHandler)
		test.Assert(t, ok, h)
	})
	t.Run("non-nil", func(t *testing.T) {
		h := getClientFrameMetaHandler(&mockFrameMetaHandler{})
		_, ok := h.(*mockFrameMetaHandler)
		test.Assert(t, ok, h)
	})
}

func Test_ttheaderStreamingClientTransHandler_newHeaderMessage(t *testing.T) {
	svcInfo := &serviceinfo.ServiceInfo{
		ServiceName:  "xxx",
		PayloadCodec: serviceinfo.Protobuf,
	}

	t.Run("newHeaderMessage", func(t *testing.T) {
		metaHandler := &mockFrameMetaHandler{
			writeHeader: func(ctx context.Context, message remote.Message) error {
				message.TransInfo().TransStrInfo()["xxx"] = "1"
				return nil
			},
			onConnectStream: func(ctx context.Context) (context.Context, error) {
				return context.WithValue(ctx, "yyy", "2"), nil
			},
		}
		opt := &remote.ClientOption{
			Option: remote.Option{
				TTHeaderFrameMetaHandler: metaHandler,
				StreamingMetaHandlers:    []remote.StreamingMetaHandler{metaHandler},
			},
		}
		h, err := NewCliTransHandler(opt, nil)
		test.Assert(t, err == nil, err)

		clientHandler := h.(*ttheaderStreamingClientTransHandler)

		ivk := rpcinfo.NewInvocation("idl-service-name", "to-method")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		ctx, msg, err := clientHandler.newHeaderMessage(ctx, svcInfo)
		test.Assert(t, err == nil, err)
		test.Assert(t, msg.MessageType() == remote.Stream, msg.MessageType())
		test.Assert(t, msg.RPCRole() == remote.Client, msg.RPCRole())
		test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeader, msg.ProtocolInfo())
		test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Protobuf, msg.ProtocolInfo())
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, msg.TransInfo().TransIntInfo())

		// WriteHeader check
		test.Assert(t, msg.TransInfo().TransStrInfo()["xxx"] == "1", msg.TransInfo().TransStrInfo())

		// StreamingMetaHandler check
		test.Assert(t, ctx.Value("yyy") == "2", ctx.Value("yyy"))
	})

	t.Run("write-header-err", func(t *testing.T) {
		metaHandler := &mockFrameMetaHandler{
			writeHeader: func(ctx context.Context, message remote.Message) error {
				return errors.New("xxx")
			},
		}
		opt := &remote.ClientOption{
			Option: remote.Option{
				TTHeaderFrameMetaHandler: metaHandler,
			},
		}
		h, err := NewCliTransHandler(opt, nil)
		test.Assert(t, err == nil, err)

		clientHandler := h.(*ttheaderStreamingClientTransHandler)

		ivk := rpcinfo.NewInvocation("idl-service-name", "to-method")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		_, _, err = clientHandler.newHeaderMessage(ctx, svcInfo)
		test.Assert(t, err != nil, err)
	})

	t.Run("OnConnectStream-err", func(t *testing.T) {
		metaHandler := &mockFrameMetaHandler{
			writeHeader: func(ctx context.Context, message remote.Message) error {
				return nil
			},
			onConnectStream: func(ctx context.Context) (context.Context, error) {
				return nil, errors.New("xxx")
			},
		}
		opt := &remote.ClientOption{
			Option: remote.Option{
				TTHeaderFrameMetaHandler: metaHandler,
				StreamingMetaHandlers:    []remote.StreamingMetaHandler{metaHandler},
			},
		}
		h, err := NewCliTransHandler(opt, nil)
		test.Assert(t, err == nil, err)

		clientHandler := h.(*ttheaderStreamingClientTransHandler)

		ivk := rpcinfo.NewInvocation("idl-service-name", "to-method")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		_, _, err = clientHandler.newHeaderMessage(ctx, svcInfo)
		test.Assert(t, err != nil, err)
	})
}

func Test_ttheaderStreamingClientTransHandler_NewStream(t *testing.T) {
	svcInfo := &serviceinfo.ServiceInfo{
		ServiceName:  "xxx",
		PayloadCodec: serviceinfo.Protobuf,
	}

	t.Run("new-header-err", func(t *testing.T) {
		metaHandler := &mockFrameMetaHandler{
			writeHeader: func(ctx context.Context, message remote.Message) error {
				return errors.New("xxx")
			},
		}
		opt := &remote.ClientOption{
			Option: remote.Option{
				TTHeaderFrameMetaHandler: metaHandler,
			},
		}
		h, err := NewCliTransHandler(opt, nil)
		test.Assert(t, err == nil, err)

		clientHandler := h.(*ttheaderStreamingClientTransHandler)

		ivk := rpcinfo.NewInvocation("idl-service-name", "to-method")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		_, err = clientHandler.NewStream(ctx, svcInfo, nil, nil)
		test.Assert(t, err != nil, err)
	})

	t.Run("normal-with-grpc-meta-and-wait-meta-frame", func(t *testing.T) {
		opt := &remote.ClientOption{
			TTHeaderStreamingGRPCCompatible: true,
			TTHeaderStreamingWaitMetaFrame:  true,
			Option: remote.Option{
				TTHeaderFrameMetaHandler: &mockFrameMetaHandler{
					readMeta: func(ctx context.Context, msg remote.Message) error {
						to := rpcinfo.AsMutableEndpointInfo(msg.RPCInfo().To())
						_ = to.SetTag("meta_key", msg.TransInfo().TransStrInfo()["meta_key"])
						return nil
					},
					writeHeader: func(ctx context.Context, message remote.Message) error {
						return nil
					},
				},
			},
		}
		ext := &mockExtension{}
		h, err := NewCliTransHandler(opt, ext)
		test.Assert(t, err == nil, err)
		clientHandler := h.(*ttheaderStreamingClientTransHandler)

		writeChan := make(chan []byte, 3)
		conn := &mockConn{
			readBuf: func() []byte {
				ivk := rpcinfo.NewInvocation("idl-service-name", "to-method")
				ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
				msg := remote.NewMessage(nil, svcInfo, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeMeta
				msg.TransInfo().TransStrInfo()["meta_key"] = "meta_value"
				out := remote.NewWriterBuffer(1024)
				ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
				if err = ttheader.NewStreamCodec().Encode(ctx, msg, out); err != nil {
					panic(err)
				}
				buf, _ := out.Bytes()
				return buf
			}(),
			write: func(b []byte) (int, error) {
				writeBuf := make([]byte, len(b))
				n := copy(writeBuf, b)
				writeChan <- writeBuf
				return n, nil
			},
		}

		ivk := rpcinfo.NewInvocation("idl-service-name", "to-method")
		to := rpcinfo.NewMutableEndpointInfo("to-service", "to-instance", nil, nil)
		ri := rpcinfo.NewRPCInfo(nil, to.ImmutableView(), ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx = metadata.AppendToOutgoingContext(ctx, "k1", "v1", "k1", "v2")

		st, err := clientHandler.NewStream(ctx, svcInfo, conn, nil)
		test.Assert(t, err == nil, err)
		_ = st.Close()

		buf := <-writeChan
		test.Assert(t, len(buf) > 0, buf)
		msg := remote.NewMessage(nil, svcInfo, ri, remote.Stream, remote.Client)
		err = codec.TTHeaderCodec().DecodeHeader(ctx, msg, remote.NewReaderBuffer(buf))
		test.Assert(t, err == nil, err)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, msg.TransInfo().TransIntInfo())
		meta := msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData]
		test.Assert(t, meta == `{"k1":["v1","v2"]}`, meta)

		v, exists := ri.To().Tag("meta_key")
		test.Assert(t, exists)
		test.Assert(t, v == "meta_value")
	})

	t.Run("read-meta-frame-err", func(t *testing.T) {
		opt := &remote.ClientOption{
			TTHeaderStreamingWaitMetaFrame: true,
		}
		ext := &mockExtension{}
		h, err := NewCliTransHandler(opt, ext)
		test.Assert(t, err == nil, err)
		clientHandler := h.(*ttheaderStreamingClientTransHandler)

		writeChan := make(chan []byte, 3)
		expectedErr := errors.New("read-meta-err")
		conn := &mockConn{
			read: func(b []byte) (int, error) {
				return -1, expectedErr
			},
			write: func(b []byte) (int, error) {
				writeBuf := make([]byte, len(b))
				n := copy(writeBuf, b)
				writeChan <- writeBuf
				return n, nil
			},
		}

		ivk := rpcinfo.NewInvocation("idl-service-name", "to-method")
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		st, err := clientHandler.NewStream(ctx, svcInfo, conn, nil)
		test.Assert(t, expectedErr.Error() == err.Error(), err)
		test.Assert(t, st == nil, st)
	})
}
