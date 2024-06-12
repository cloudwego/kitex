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
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	ktest "github.com/cloudwego/kitex/pkg/protocol/bthrift/test/kitex_gen/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/remote/codec/ttheader"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

func mockRPCInfo(args ...interface{}) rpcinfo.RPCInfo {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8888")
	from := rpcinfo.NewEndpointInfo("from", "from", nil, map[string]string{})
	to := rpcinfo.NewEndpointInfo("to", "method", addr, map[string]string{})
	var ivk rpcinfo.Invocation = rpcinfo.NewInvocation("idl-service", "method")
	cfg := rpcinfo.NewRPCConfig()
	cfg.(rpcinfo.MutableRPCConfig).SetPayloadCodec(serviceinfo.Protobuf)
	stat := rpcinfo.NewRPCStats()
	for _, obj := range args {
		switch t := obj.(type) {
		case rpcinfo.RPCConfig:
			cfg = t
		case rpcinfo.Invocation:
			ivk = t
		case rpcinfo.EndpointInfo:
			if _, exists := t.Tag("from"); exists {
				from = t
			} else if _, exists := t.Tag("to"); exists {
				to = t
			} else {
				panic("invalid endpoint info for mockRPCInfo")
			}
		case rpcinfo.RPCStats:
			stat = t
		}
	}
	return rpcinfo.NewRPCInfo(from, to, ivk, cfg, stat)
}

func Test_newTTHeaderStream(t *testing.T) {
	ri := mockRPCInfo()
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	writeCnt := 0
	conn := &mockConn{
		write: func(b []byte) (int, error) {
			msg, err := decodeMessage(context.Background(), b, nil, remote.Client)
			test.Assert(t, err == nil, err)
			if writeCnt == 0 {
				test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
			} else if writeCnt == 1 {
				test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, codec.MessageFrameType(msg))
			}
			writeCnt += 1
			return len(b), nil
		},
	}
	streamCodec := ttheader.NewStreamCodec()
	ext := &mockExtension{}
	handler := getClientFrameMetaHandler(nil)

	st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
	test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

	err := st.closeSend(nil)
	test.Assert(t, err == nil, err)
	test.Assert(t, writeCnt == 2, writeCnt)
}

func Test_ttheaderStream_SetHeader(t *testing.T) {
	t.Run("client", func(t *testing.T) {
		var st *ttheaderStream
		defer func() {
			panicInfo := recover()
			test.Assert(t, panicInfo != nil, panicInfo)
			_ = st.closeSend(nil) // avoid goroutine leak
		}()
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := metadata.MD{
			"key": []string{"v1", "v2"},
		}
		_ = st.SetHeader(md)
		test.Assert(t, false, "should panic")
	})

	t.Run("server:header-sent", func(t *testing.T) {
		var st *ttheaderStream
		defer func() {
			panicInfo := recover()
			test.Assert(t, panicInfo == nil, panicInfo)
			_ = st.closeSend(nil) // avoid goroutine leak
		}()
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := metadata.MD{
			"key": []string{"v1", "v2"},
		}

		err := st.SendHeader(nil)
		test.Assert(t, err == nil, err)

		err = st.SetHeader(md)
		test.Assert(t, errors.Is(err, ErrIllegalHeaderWrite), err)
	})

	t.Run("server:closed", func(t *testing.T) {
		var st *ttheaderStream
		defer func() {
			panicInfo := recover()
			test.Assert(t, panicInfo == nil, panicInfo)
			_ = st.closeSend(nil) // avoid goroutine leak
		}()
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := metadata.MD{
			"key": []string{"v1", "v2"},
		}

		st.sendState.setClosed()

		err := st.SetHeader(md)
		test.Assert(t, errors.Is(err, ErrIllegalHeaderWrite), err)
	})

	t.Run("normal", func(t *testing.T) {
		var st *ttheaderStream
		defer func() {
			panicInfo := recover()
			test.Assert(t, panicInfo == nil, panicInfo)
			_ = st.closeSend(nil) // avoid goroutine leak
		}()
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := metadata.MD{
			"key": []string{"v1", "v2"},
		}

		err := st.SetHeader(md)
		test.Assert(t, err == nil, err)

		test.Assert(t, reflect.DeepEqual(st.grpcMetadata.headersToSend, md), st.grpcMetadata.headersToSend)
	})
}

func Test_ttheaderStream_SendHeader(t *testing.T) {
	t.Run("client", func(t *testing.T) {
		var st *ttheaderStream
		defer func() {
			panicInfo := recover()
			test.Assert(t, panicInfo != nil, panicInfo)
			_ = st.closeSend(nil) // avoid goroutine leak
		}()
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		writeCount := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Server)
				test.Assert(t, err == nil, err)
				if writeCount == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
				}
				writeCount += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := metadata.MD{
			"key": []string{"v1", "v2"},
		}
		_ = st.SendHeader(md)
		test.Assert(t, false, "should panic")
	})

	t.Run("server:normal", func(t *testing.T) {
		var st *ttheaderStream
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		writeCnt := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Server)
				test.Assert(t, err == nil, err)
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
				} else if writeCnt == 1 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, codec.MessageFrameType(msg))
				}
				writeCnt += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := metadata.MD{
			"key": []string{"v1", "v2"},
		}
		err := st.SendHeader(md)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(st.grpcMetadata.headersToSend, md), st.grpcMetadata.headersToSend)

		_ = st.closeSend(nil) // avoid goroutine leak
		test.Assert(t, writeCnt == 2, writeCnt)
	})
}

func Test_ttheaderStream_SetTrailer(t *testing.T) {
	t.Run("client", func(t *testing.T) {
		var st *ttheaderStream
		defer func() {
			panicInfo := recover()
			test.Assert(t, panicInfo != nil, panicInfo)
			_ = st.closeSend(nil) // avoid goroutine leak
		}()
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := metadata.MD{
			"key": []string{"v1", "v2"},
		}
		st.SetTrailer(md)
		test.Assert(t, false, "should panic")
	})

	t.Run("server:normal", func(t *testing.T) {
		var st *ttheaderStream
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := metadata.MD{
			"key": []string{"v1", "v2"},
		}
		st.SetTrailer(md)
		test.Assert(t, reflect.DeepEqual(st.grpcMetadata.trailerToSend, md), st.grpcMetadata.headersToSend)

		_ = st.closeSend(nil) // avoid goroutine leak
	})

	t.Run("server:empty-md", func(t *testing.T) {
		var st *ttheaderStream
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := metadata.MD{}
		st.SetTrailer(md)
		test.Assert(t, len(st.grpcMetadata.trailerToSend) == 0, st.grpcMetadata.trailerToSend)

		_ = st.closeSend(nil) // avoid goroutine leak
	})

	t.Run("server:closed", func(t *testing.T) {
		var st *ttheaderStream
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		_ = st.closeSend(nil) // avoid goroutine leak

		md := metadata.MD{"key": []string{"k1", "k2"}}
		st.SetTrailer(md)
		test.Assert(t, len(st.grpcMetadata.trailerToSend) == 0, st.grpcMetadata.trailerToSend)
	})
}

func Test_ttheaderStream_Header(t *testing.T) {
	t.Run("server:panic", func(t *testing.T) {
		var st *ttheaderStream
		defer func() {
			panicInfo := recover()
			test.Assert(t, panicInfo != nil, panicInfo)
			_ = st.closeSend(nil) // avoid goroutine leak
		}()
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		_, _ = st.Header()
		test.Assert(t, false, "should panic")
	})

	t.Run("client:received", func(t *testing.T) {
		var st *ttheaderStream
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		st.grpcMetadata.headersReceived = metadata.MD{
			"key": []string{"v1", "v2"},
		}

		st.recvState.setHeader()

		md, err := st.Header()
		test.Assert(t, err == nil, err)
		test.Assert(t, len(md["key"]) == 2, md)

		_ = st.closeSend(nil) // avoid goroutine leak
	})

	t.Run("client:not-received,read-header", func(t *testing.T) {
		var st *ttheaderStream
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				ri := mockRPCInfo()
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"header":["v1","v2"]}`
				buf, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md, err := st.Header()
		test.Assert(t, err == nil, err)
		test.Assert(t, len(md["header"]) == 2, md)

		_ = st.closeSend(nil) // avoid goroutine leak
	})

	t.Run("client:not-received,read-header-err", func(t *testing.T) {
		var st *ttheaderStream
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		readErr := errors.New("read header error")
		conn := &mockConn{
			read: func(b []byte) (int, error) {
				return 0, readErr
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		_, err := st.Header()
		test.Assert(t, errors.Is(err, readErr), err)

		_ = st.closeSend(nil) // avoid goroutine leak
	})
}

func Test_ttheaderStream_Trailer(t *testing.T) {
	t.Run("server:panic", func(t *testing.T) {
		var st *ttheaderStream
		defer func() {
			panicInfo := recover()
			test.Assert(t, panicInfo != nil, panicInfo)
			_ = st.closeSend(nil) // avoid goroutine leak
		}()
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		_ = st.Trailer()
		test.Assert(t, false, "should panic")
	})

	t.Run("client:trailer-received", func(t *testing.T) {
		var st *ttheaderStream
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		st.recvState.setTrailer()
		st.grpcMetadata.trailerReceived = metadata.MD{
			"trailer": []string{"v1", "v2"},
		}

		md := st.Trailer()
		test.Assert(t, len(md["trailer"]) == 2, md)

		_ = st.closeSend(nil) // avoid goroutine leak
	})

	t.Run("client:not-received,read-trailer", func(t *testing.T) {
		var st *ttheaderStream
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				ri := mockRPCInfo()
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				bufHeader, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}

				msg = remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
				msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"trailer":["v1","v2"]}`
				bufTrailer, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				return append(bufHeader, bufTrailer...)
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st = newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		test.Assert(t, rpcinfo.GetRPCInfo(st.Context()) != nil)

		md := st.Trailer()
		test.Assert(t, len(md["trailer"]) == 2, md)

		_ = st.closeSend(nil) // avoid goroutine leak
	})
}

func Test_ttheaderStream_RecvMsg(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		req := &ktest.Local{L: 1}
		remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
		conn := &mockConn{
			readBuf: func() []byte {
				cfg := rpcinfo.NewRPCConfig()
				cfg.(rpcinfo.MutableRPCConfig).SetPayloadCodec(serviceinfo.Thrift)
				ri := mockRPCInfo(cfg)
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				bufHeader, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}

				msg = remote.NewMessage(req, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeData
				bufData, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				return append(bufHeader, bufData...)
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		gotReq := &ktest.Local{}
		err := st.RecvMsg(gotReq)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(req, gotReq), req, gotReq)

		_ = st.closeSend(nil) // avoid goroutine leak
	})

	t.Run("closed", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		st.closeRecv(io.EOF)

		err := st.RecvMsg(nil)
		test.Assert(t, err == io.EOF, err)

		_ = st.closeSend(nil) // avoid goroutine leak
	})

	t.Run("io.EOF,BizStatusError", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
		conn := &mockConn{
			readBuf: func() []byte {
				ri := mockRPCInfo()
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				bufHeader, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}

				msg = remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
				bufTrailer, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				return append(bufHeader, bufTrailer...)
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		ri.Invocation().(rpcinfo.InvocationSetter).SetBizStatusErr(kerrors.NewBizStatusError(1, "biz err"))

		err := st.RecvMsg(nil)
		test.Assert(t, err == nil, err)

		_ = st.closeSend(nil) // avoid goroutine leak
	})

	t.Run("read-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
		conn := &mockConn{
			readBuf: []byte{},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		err := st.RecvMsg(nil)
		test.Assert(t, err != nil, err)

		_ = st.closeSend(nil) // avoid goroutine leak
	})
}

func Test_ttheaderStream_SendMsg(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		cfg := rpcinfo.NewRPCConfig()
		cfg.(rpcinfo.MutableRPCConfig).SetPayloadCodec(serviceinfo.Thrift)
		ri := mockRPCInfo(cfg)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
		var writeBuf []byte
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				writeBuf = append(writeBuf, b...)
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		req := &ktest.Local{L: 1}
		err := st.SendMsg(req)
		test.Assert(t, err == nil, err)

		_ = st.closeSend(nil) // avoid goroutine leak

		in := remote.NewReaderBuffer(writeBuf)
		msg, err := decodeMessageFromBuffer(context.Background(), in, nil, remote.Client)
		test.Assert(t, err == nil, err)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))

		data := &ktest.Local{}
		msg, err = decodeMessageFromBuffer(context.Background(), in, data, remote.Client)
		test.Assert(t, err == nil, err)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeData, codec.MessageFrameType(msg))
		test.Assert(t, reflect.DeepEqual(req, data), req, data)

		msg, err = decodeMessageFromBuffer(context.Background(), in, nil, remote.Client)
		test.Assert(t, err == nil, err)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, codec.MessageFrameType(msg))
	})

	t.Run("send-closed", func(t *testing.T) {
		cfg := rpcinfo.NewRPCConfig()
		cfg.(rpcinfo.MutableRPCConfig).SetPayloadCodec(serviceinfo.Thrift)
		ri := mockRPCInfo(cfg)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		err := st.closeSend(nil)
		test.Assert(t, err == nil, err)

		err = st.SendMsg(nil)
		test.Assert(t, errors.Is(err, ErrSendClosed), err)
	})

	t.Run("send-header-err", func(t *testing.T) {
		cfg := rpcinfo.NewRPCConfig()
		cfg.(rpcinfo.MutableRPCConfig).SetPayloadCodec(serviceinfo.Thrift)
		ri := mockRPCInfo(cfg)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				return -1, io.ErrClosedPipe
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		err := st.SendMsg(nil)
		test.Assert(t, err != nil, err)

		_ = st.closeSend(nil)
	})
}

func Test_ttheaderStream_Close(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		var writeBuf []byte
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				writeBuf = append(writeBuf, b...)
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		_ = st.Close() // avoid goroutine leak

		in := remote.NewReaderBuffer(writeBuf)
		msg, err := decodeMessageFromBuffer(context.Background(), in, nil, remote.Client)
		test.Assert(t, err == nil, err)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))

		msg, err = decodeMessageFromBuffer(context.Background(), in, nil, remote.Client)
		test.Assert(t, err == nil, err)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, codec.MessageFrameType(msg))
	})
}

func Test_ttheaderStream_waitSendLoopClose(t *testing.T) {
	ri := mockRPCInfo()
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	var writeBuf []byte
	conn := &mockConn{
		write: func(b []byte) (int, error) {
			writeBuf = append(writeBuf, b...)
			return len(b), nil
		},
	}
	streamCodec := ttheader.NewStreamCodec()
	ext := &mockExtension{}
	handler := getClientFrameMetaHandler(nil)

	st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
	defer st.Close()

	finished := make(chan struct{})
	go func() {
		st.waitSendLoopClose()
		finished <- struct{}{}
	}()

	close(st.sendLoopCloseSignal)
	select {
	case <-finished:
	case <-time.NewTimer(time.Millisecond * 100).C:
		t.Error("timeout")
	}
}

func Test_ttheaderStream_sendHeaderFrame(t *testing.T) {
	t.Run("already-sent", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Server)
				test.Assert(t, err == nil, err)
				// only TrailerFrame
				test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, codec.MessageFrameType(msg))
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		st.sendState.setHeader()

		err := st.sendHeaderFrame(nil)
		test.Assert(t, err == nil, err)

		_ = st.Close() // avoid goroutine leak
	})

	t.Run("client-header", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		writeCnt := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Server)
				test.Assert(t, err == nil, err)
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
					test.Assert(t, msg.TransInfo().TransStrInfo()["key"] == "value", msg.TransInfo().TransStrInfo())
				}
				writeCnt += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		msg.TransInfo().TransStrInfo()["key"] = "value"

		err := st.sendHeaderFrame(msg)
		test.Assert(t, err == nil, err)

		_ = st.Close() // avoid goroutine leak
	})

	t.Run("server-header", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		writeCnt := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Server)
				test.Assert(t, err == nil, err)
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
					test.Assert(t, msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] != "", msg.TransInfo().TransStrInfo())
				}
				writeCnt += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		ctx = metadata.AppendToOutgoingContext(ctx, "key", "value")

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		st.loadOutgoingMetadataForGRPC()

		err := st.sendHeaderFrame(nil)
		test.Assert(t, err == nil, err)

		_ = st.Close() // avoid goroutine leak
	})
}

func Test_ttheaderStream_sendTrailerFrame(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		writeCnt := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Server)
				test.Assert(t, err == nil, err)
				switch writeCnt {
				case 0:
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
				case 1:
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, codec.MessageFrameType(msg))
				default:
					test.Assert(t, false, "should not be here")
				}
				writeCnt += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		err := st.sendTrailerFrame(nil)
		test.Assert(t, err == nil, err)

		err = st.sendTrailerFrame(nil)
		test.Assert(t, err == nil, err)

		_ = st.Close() // avoid goroutine leak
	})

	t.Run("send-header-fail", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				return 0, errors.New("write error")
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		err := st.sendTrailerFrame(nil)
		test.Assert(t, err != nil, err)

		_ = st.Close() // avoid goroutine leak
	})
}

func Test_ttheaderStream_closeRecv(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				return 0, errors.New("write error")
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		st.closeRecv(nil)
		test.Assert(t, st.lastRecvError == io.EOF, st.lastRecvError)

		st.closeRecv(io.ErrClosedPipe) // close before, no use
		test.Assert(t, st.lastRecvError == io.EOF, st.lastRecvError)

		_ = st.Close() // avoid goroutine leak
	})

	t.Run("closed-with-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				return 0, errors.New("write error")
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

		st.closeRecv(io.ErrClosedPipe)

		test.Assert(t, st.lastRecvError == io.ErrClosedPipe, st.lastRecvError)

		_ = st.Close() // avoid goroutine leak
	})
}

func Test_ttheaderStream_newMessageToSend(t *testing.T) {
	ri := mockRPCInfo()
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	conn := &mockConn{
		write: func(b []byte) (int, error) {
			return 0, errors.New("write error")
		},
	}
	streamCodec := ttheader.NewStreamCodec()
	ext := &mockExtension{}
	handler := getClientFrameMetaHandler(nil)

	st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)

	data := ktest.Local{L: 1}
	msg := st.newMessageToSend(data, codec.FrameTypeData)

	test.Assert(t, msg.RPCRole() == remote.Client, msg.RPCRole())
	test.Assert(t, msg.MessageType() == remote.Stream, msg.MessageType())
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Protobuf, msg.ProtocolInfo().CodecType)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeader, msg.ProtocolInfo().TransProto)
	test.Assert(t, msg.Tags()[codec.HeaderFlagsKey] == codec.HeaderFlagsStreaming, msg.Tags())
	test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeData, codec.MessageFrameType(msg))

	_ = st.Close() // avoid goroutine leak
}

func Test_ttheaderStream_readMessageWatchingCtxDone(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				ri := mockRPCInfo()
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				bufHeader, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				return bufHeader
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg, err := st.readMessageWatchingCtxDone(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
	})

	t.Run("read-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		_, err := st.readMessageWatchingCtxDone(nil)
		test.Assert(t, err != nil, err)
	})

	t.Run("timeout", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			read: func(b []byte) (int, error) {
				time.Sleep(time.Millisecond * 20)
				return 0, io.EOF
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)
		ctx, cancel := context.WithTimeout(ctx, time.Millisecond*10)
		defer cancel()

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		_, err := st.readMessageWatchingCtxDone(nil)
		test.Assert(t, err != nil, err)
		test.Assert(t, errors.Is(st.lastRecvError, context.DeadlineExceeded), st.lastRecvError)
	})
}

func Test_ttheaderStream_readMessage(t *testing.T) {
	t.Run("recv-closed", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		_ = st.closeSend(nil)

		_, err := st.readMessage(nil)
		test.Assert(t, errors.Is(err, ErrSendClosed), err)
	})

	t.Run("read-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg, err := st.readMessage(nil)
		test.Assert(t, err != nil, err)
		test.Assert(t, msg == nil, msg)
	})

	t.Run("read-success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				bufHeader, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				return bufHeader
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg, err := st.readMessage(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
	})

	t.Run("read-panic", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			read: func(b []byte) (int, error) {
				panic("test panic")
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		_, err := st.readMessage(nil)
		test.Assert(t, err != nil, err)
		test.Assert(t, strings.Contains(err.Error(), "test panic"), err.Error())
	})
}

func Test_ttheaderStream_parseMetaFrame(t *testing.T) {
	t.Run("already-received", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			read: func(b []byte) (int, error) {
				panic("test panic")
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		st.recvState.setMeta()

		err := st.parseMetaFrame(nil)
		test.Assert(t, strings.Contains(err.Error(), "unexpected meta frame"), err.Error())
	})

	t.Run("server-got-meta-frame:should-panic", func(t *testing.T) {
		defer func() {
			panicInfo := recover()
			test.Assert(t, panicInfo != nil, panicInfo)
		}()
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			read: func(b []byte) (int, error) {
				panic("test panic")
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		_ = st.parseMetaFrame(msg)
		t.Error("should panic")
	})

	t.Run("normal", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			read: func(b []byte) (int, error) {
				panic("test panic")
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		called := false
		handler := &mockFrameMetaHandler{
			readMeta: func(ctx context.Context, msg remote.Message) error {
				called = true
				return nil
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		err := st.parseMetaFrame(msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, called)
	})
}

func Test_ttheaderStream_parseHeaderFrame(t *testing.T) {
	t.Run("client:success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		called := false
		handler := &mockFrameMetaHandler{
			clientReadHeader: func(ctx context.Context, msg remote.Message) error {
				called = true
				return nil
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"key":["v1","v2"]}`
		err := st.parseHeaderFrame(msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, called)
		test.Assert(t, len(st.grpcMetadata.headersReceived["key"]) == 2, st.grpcMetadata.headersReceived)
		test.Assert(t, st.recvState.hasHeader())
	})

	t.Run("client:read-header-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		readErr := errors.New("read header error")
		handler := &mockFrameMetaHandler{
			clientReadHeader: func(ctx context.Context, msg remote.Message) error {
				return readErr
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"key":["v1","v2"]}`
		err := st.parseHeaderFrame(msg)
		test.Assert(t, errors.Is(err, readErr), err)
		test.Assert(t, len(st.grpcMetadata.headersReceived) == 0, st.grpcMetadata.headersReceived)
		test.Assert(t, st.recvState.hasHeader())
	})

	t.Run("server", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		called := false
		handler := &mockFrameMetaHandler{
			readMeta: func(ctx context.Context, msg remote.Message) error {
				called = true
				return nil
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		err := st.parseHeaderFrame(msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, !called, called) // it's called in server_handler.go
		test.Assert(t, st.recvState.hasHeader())
	})
}

func Test_ttheaderStream_parseDataFrame(t *testing.T) {
	t.Run("no-header", func(t *testing.T) {
		st := &ttheaderStream{}
		err := st.parseDataFrame(nil)
		test.Assert(t, err != nil, err)
	})
	t.Run("has-header", func(t *testing.T) {
		st := &ttheaderStream{}
		st.recvState.setHeader()
		err := st.parseDataFrame(nil)
		test.Assert(t, err == nil, err)
	})
}

func Test_ttheaderStream_parseTrailerFrame(t *testing.T) {
	t.Run("client:no-header", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
		err := st.parseTrailerFrame(msg)
		test.Assert(t, err != nil, err)
		test.Assert(t, st.recvState.hasTrailer())
	})

	t.Run("client:ReadTrailer-TransErr", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		transErr := remote.NewTransErrorWithMsg(1, "mock err")
		handler := &mockFrameMetaHandler{
			readTrailer: func(ctx context.Context, msg remote.Message) error {
				return transErr
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.recvState.setHeader()

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
		err := st.parseTrailerFrame(msg)
		test.Assert(t, errors.Is(err, transErr), err)
		test.Assert(t, st.recvState.hasTrailer())
	})

	t.Run("client:ReadTrailer-other-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		readErr := errors.New("read error")
		handler := &mockFrameMetaHandler{
			readTrailer: func(ctx context.Context, msg remote.Message) error {
				return readErr
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.recvState.setHeader()

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
		err := st.parseTrailerFrame(msg)
		var transError *remote.TransError
		isTransErr := errors.As(err, &transError)
		test.Assert(t, !isTransErr && transError == nil, err)
		test.Assert(t, strings.Contains(err.Error(), readErr.Error()), err)
		test.Assert(t, st.recvState.hasTrailer())
	})

	t.Run("client:parse-grpc-trailer-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{
			readTrailer: func(ctx context.Context, msg remote.Message) error {
				return nil
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.recvState.setHeader()

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
		msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `invalid trailer`
		err := st.parseTrailerFrame(msg)
		test.Assert(t, err != nil, err)
		test.Assert(t, len(st.grpcMetadata.trailerReceived) == 0, st.grpcMetadata.trailerReceived)
		test.Assert(t, errors.Is(err, st.lastRecvError), st.lastRecvError)
	})

	t.Run("client:success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{
			readTrailer: func(ctx context.Context, msg remote.Message) error {
				return nil
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.recvState.setHeader()

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
		msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"key":["v1","v2"]}`
		err := st.parseTrailerFrame(msg)
		test.Assert(t, err == io.EOF, err)
		test.Assert(t, len(st.grpcMetadata.trailerReceived) == 1, st.grpcMetadata.trailerReceived)
		test.Assert(t, st.lastRecvError == io.EOF, st.lastRecvError)
	})

	t.Run("server:success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{
			readTrailer: func(ctx context.Context, msg remote.Message) error {
				return nil
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.recvState.setHeader()

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
		msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"key":["v1","v2"]}`
		err := st.parseTrailerFrame(msg)
		test.Assert(t, err == io.EOF, err)
		// server: not grpc-trailer from client
		test.Assert(t, len(st.grpcMetadata.trailerReceived) == 0, st.grpcMetadata.trailerReceived)
		test.Assert(t, st.lastRecvError == io.EOF, st.lastRecvError)
	})
}

func Test_ttheaderStream_readAndParseMessage(t *testing.T) {
	t.Run("read-msg-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		ft, err := st.readAndParseMessage(nil)
		test.Assert(t, err != nil, err)
		test.Assert(t, ft == codec.FrameTypeInvalid, ft)
	})

	t.Run("parse-meta-frame-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeMeta
				buf, _ := encodeMessage(ctx, msg)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		mockErr := errors.New("mock error")
		handler := &mockFrameMetaHandler{
			readMeta: func(ctx context.Context, msg remote.Message) error {
				return mockErr
			},
		}
		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		ft, err := st.readAndParseMessage(nil)
		test.Assert(t, errors.Is(err, mockErr), err)
		test.Assert(t, ft == codec.FrameTypeMeta, ft)
	})

	t.Run("parse-meta-frame-success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeMeta
				buf, _ := encodeMessage(ctx, msg)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{
			readMeta: func(ctx context.Context, msg remote.Message) error {
				return nil
			},
		}
		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		ft, err := st.readAndParseMessage(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, ft == codec.FrameTypeMeta, ft)
	})

	t.Run("parse-header-frame-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				buf, _ := encodeMessage(ctx, msg)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		mockErr := errors.New("mock error")
		handler := &mockFrameMetaHandler{
			clientReadHeader: func(ctx context.Context, msg remote.Message) error {
				return mockErr
			},
		}
		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		ft, err := st.readAndParseMessage(nil)
		test.Assert(t, errors.Is(err, mockErr), err)
		test.Assert(t, ft == codec.FrameTypeHeader, ft)
	})

	t.Run("parse-header-frame-success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				buf, _ := encodeMessage(ctx, msg)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{
			clientReadHeader: func(ctx context.Context, msg remote.Message) error {
				return nil
			},
		}
		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		ft, err := st.readAndParseMessage(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, ft == codec.FrameTypeHeader, ft)
	})

	t.Run("parse-data-frame-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
		conn := &mockConn{
			readBuf: func() []byte {
				data := &ktest.Local{L: 1}
				msg := remote.NewMessage(data, nil, ri, remote.Stream, remote.Server)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeData
				buf, err := encodeMessage(ctx, msg)
				test.Assert(t, err == nil, err)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{}
		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		ft, err := st.readAndParseMessage(nil)
		test.Assert(t, err != nil, err) // read data frame before header frame
		test.Assert(t, ft == codec.FrameTypeData, ft)
	})

	t.Run("parse-data-frame-success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
		conn := &mockConn{
			readBuf: func() []byte {
				data := &ktest.Local{L: 1}
				msg := remote.NewMessage(data, nil, ri, remote.Stream, remote.Server)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeData
				buf, err := encodeMessage(ctx, msg)
				test.Assert(t, err == nil, err)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{
			clientReadHeader: func(ctx context.Context, msg remote.Message) error {
				return nil
			},
		}
		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.recvState.setHeader()

		ft, err := st.readAndParseMessage(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, ft == codec.FrameTypeData, ft)
	})

	t.Run("parse-trailer-frame-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
				buf, err := encodeMessage(ctx, msg)
				test.Assert(t, err == nil, err)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		mockErr := errors.New("mock error")
		handler := &mockFrameMetaHandler{
			readTrailer: func(ctx context.Context, msg remote.Message) error {
				return mockErr
			},
		}
		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.recvState.setHeader()

		ft, err := st.readAndParseMessage(nil)
		test.Assert(t, errors.Is(err, mockErr), err)
		test.Assert(t, ft == codec.FrameTypeTrailer, ft)
	})

	t.Run("parse-trailer-frame-success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
				buf, err := encodeMessage(ctx, msg)
				test.Assert(t, err == nil, err)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{
			clientReadHeader: func(ctx context.Context, msg remote.Message) error {
				return nil
			},
		}
		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.recvState.setHeader()

		ft, err := st.readAndParseMessage(nil)
		test.Assert(t, err == io.EOF, err)
		test.Assert(t, ft == codec.FrameTypeTrailer, ft)
	})

	t.Run("unknown-frame-type", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = "XXX"
				buf, err := encodeMessage(ctx, msg)
				test.Assert(t, err == nil, err)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{
			clientReadHeader: func(ctx context.Context, msg remote.Message) error {
				return nil
			},
		}
		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.recvState.setHeader()

		_, err := st.readAndParseMessage(nil)
		test.Assert(t, errors.Is(err, ErrInvalidFrame), err)
	})
}

func Test_ttheaderStream_readUntilTargetFrame(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				ri := mockRPCInfo()
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				bufHeader, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				return bufHeader
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		err := st.readUntilTargetFrame(nil, codec.FrameTypeHeader)
		test.Assert(t, err == nil, err)
	})

	t.Run("read-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		err := st.readUntilTargetFrame(nil, codec.FrameTypeHeader)
		test.Assert(t, err != nil, err)
	})

	t.Run("timeout", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			read: func(b []byte) (int, error) {
				time.Sleep(time.Millisecond * 20)
				return 0, io.EOF
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)
		ctx, cancel := context.WithTimeout(ctx, time.Millisecond*10)
		defer cancel()

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		err := st.readUntilTargetFrame(nil, codec.FrameTypeHeader)
		test.Assert(t, err != nil, err)
		test.Assert(t, errors.Is(st.lastRecvError, context.DeadlineExceeded), st.lastRecvError)
	})
}

func Test_ttheaderStream_readUntilTargetFrameSynchronously(t *testing.T) {
	t.Run("read-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		err := st.readUntilTargetFrameSynchronously(nil, codec.FrameTypeHeader)
		test.Assert(t, err != nil, err)
	})

	t.Run("read-target-frame", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				ri := mockRPCInfo()
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				bufHeader, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				return bufHeader
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		err := st.readUntilTargetFrameSynchronously(nil, codec.FrameTypeHeader)
		test.Assert(t, err == nil, err)
	})

	t.Run("2nd-to-be-target-frame", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{
			readBuf: func() []byte {
				ri := mockRPCInfo()
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				bufHeader, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				msg = remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
				bufTrailer, err := encodeMessage(context.Background(), msg)
				if err != nil {
					t.Fatal(err)
				}
				return append(bufHeader, bufTrailer...)
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		err := st.readUntilTargetFrameSynchronously(nil, codec.FrameTypeTrailer)
		test.Assert(t, err == io.EOF, err)
	})
}

func Test_ttheaderStream_putMessageToSendQueue(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		writeCnt := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Client)
				test.Assert(t, err == nil, err)
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
				}
				writeCnt += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		err := st.putMessageToSendQueue(msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, writeCnt > 0)
	})

	t.Run("ctx-done", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithTimeout(ctx, time.Millisecond*10)
		defer cancel()
		writeCnt := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Client)
				test.Assert(t, err == nil, err)
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
					time.Sleep(time.Millisecond * 20)
				}
				writeCnt += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		err := st.putMessageToSendQueue(msg)
		test.Assert(t, errors.Is(err, context.DeadlineExceeded), err)
	})

	t.Run("sendLoopCloseSignal-closed", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		writeCnt := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Client)
				test.Assert(t, err == nil, err)
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
					time.Sleep(time.Millisecond * 50)
				}
				writeCnt += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		_ = st.Close() // finish sendLoop and close sendLoopCloseSignal

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		err := st.putMessageToSendQueue(msg)
		test.Assert(t, errors.Is(err, ErrSendClosed), err)
	})
}

func Test_sendRequest_waitSendFinishedSignal(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		st := &ttheaderStream{
			ctx:                 context.Background(),
			sendLoopCloseSignal: make(chan struct{}),
		}
		ch := make(chan error, 1)
		ch <- nil
		err := st.waitSendFinishedSignal(ch)
		test.Assert(t, err == nil, err)
	})
	t.Run("ctx-done", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		st := &ttheaderStream{
			ctx:                 ctx,
			sendLoopCloseSignal: make(chan struct{}),
		}
		cancel()
		ch := make(chan error, 1)
		err := st.waitSendFinishedSignal(ch)
		test.Assert(t, errors.Is(err, context.Canceled), err)
	})
	t.Run("sendLoopCloseSignal", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		st := &ttheaderStream{
			ctx:                 ctx,
			sendLoopCloseSignal: make(chan struct{}),
		}
		defer cancel()
		close(st.sendLoopCloseSignal)
		ch := make(chan error, 1)
		err := st.waitSendFinishedSignal(ch)
		test.Assert(t, errors.Is(err, ErrSendClosed), err)
	})
}

func Test_sendRequest_waitFinishSignal(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		sendReq := sendRequest{
			finishSignal: make(chan error, 1),
		}
		sendReq.finishSignal <- nil
		err := sendReq.waitFinishSignal(context.Background())
		test.Assert(t, err == nil, err)
	})
	t.Run("canceled", func(t *testing.T) {
		sendReq := sendRequest{
			finishSignal: make(chan error, 1),
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := sendReq.waitFinishSignal(ctx)
		test.Assert(t, errors.Is(err, context.Canceled), err)
	})
}

func Test_ttheaderStream_closeSend(t *testing.T) {
	t.Run("closed-before", func(t *testing.T) {
		st := &ttheaderStream{}
		st.sendState.setClosed()
		err := st.closeSend(errors.New("XXX"))
		test.Assert(t, err == nil, err)
	})

	t.Run("client", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		writeCnt := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Client)
				test.Assert(t, err == nil, err)
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
				} else if writeCnt == 1 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, codec.MessageFrameType(msg))
					test.Assert(t, msg.TransInfo().TransIntInfo()[transmeta.TransCode] != "", msg.TransInfo())
					test.Assert(t, msg.TransInfo().TransIntInfo()[transmeta.TransMessage] != "", msg.TransInfo())
				}
				writeCnt += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		err := st.closeSend(errors.New("XXX"))
		test.Assert(t, err == nil, err)
		test.Assert(t, writeCnt == 2)
		_, active := <-st.sendLoopCloseSignal
		test.Assert(t, !active, "sendLoopCloseSignal should be closed")
	})

	t.Run("server", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		writeCnt := 0
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(context.Background(), b, nil, remote.Client)
				test.Assert(t, err == nil, err)
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, codec.MessageFrameType(msg))
				} else if writeCnt == 1 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, codec.MessageFrameType(msg))
					test.Assert(t, msg.TransInfo().TransIntInfo()[transmeta.TransCode] != "", msg.TransInfo())
					test.Assert(t, msg.TransInfo().TransIntInfo()[transmeta.TransMessage] != "", msg.TransInfo())
				}
				writeCnt += 1
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		err := st.closeSend(errors.New("XXX"))
		test.Assert(t, err == nil, err)
		test.Assert(t, writeCnt == 2)
		_, active := <-st.sendLoopCloseSignal
		test.Assert(t, !active, "sendLoopCloseSignal should be closed")
		test.Assert(t, st.recvState.isClosed(), st.recvState)
		test.Assert(t, errors.Is(st.ctx.Err(), context.Canceled), st.ctx.Err())
	})
}

func Test_ttheaderStream_newTrailerFrame(t *testing.T) {
	t.Run("write-trailer-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		mockErr := errors.New("XXX")
		handler := &mockFrameMetaHandler{
			writeTrailer: func(ctx context.Context, message remote.Message) error {
				return mockErr
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := st.newTrailerFrame(nil)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, msg.TransInfo())
	})

	t.Run("normal", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak
		st.SetTrailer(metadata.MD{"key": []string{"v1", "v2"}})

		msg := st.newTrailerFrame(nil)
		test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, msg.TransInfo())
		test.Assert(t, len(msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData]) != 0, msg.TransInfo())
	})
}

func Test_ttheaderStream_sendLoop(t *testing.T) {
	t.Run("normal:send-header", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		sendReq := sendRequest{
			message:      msg,
			finishSignal: make(chan error, 1),
		}
		st.sendQueue <- sendReq
		err := sendReq.waitFinishSignal(st.Context())
		test.Assert(t, err == nil, err)
		test.Assert(t, !st.sendState.isClosed(), st.recvState)
	})

	t.Run("normal:send-trailer-with-extra-message", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		sendFinishSignal := make(chan struct{})
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				<-sendFinishSignal // make sure all sendRequests are put into the sendQueue
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := &mockFrameMetaHandler{}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		var requests []sendRequest
		for i := 0; i < 10; i++ {
			msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
			msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
			sendReq := sendRequest{
				message:      msg,
				finishSignal: make(chan error, 1),
			}
			requests = append(requests, sendReq)
			st.sendQueue <- sendReq
		}
		close(sendFinishSignal)
		for i := 0; i < 10; i++ {
			err := requests[i].waitFinishSignal(st.Context())
			if i == 0 {
				test.Assert(t, err == nil, err)
			} else {
				test.Assert(t, errors.Is(err, ErrSendClosed), err)
			}
		}
		test.Assert(t, st.sendState.isClosed(), st.recvState)
		_, active := <-st.sendLoopCloseSignal
		test.Assert(t, !active, "sendLoopCloseSignal should be closed")
	})

	t.Run("ctx-done", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(ctx, b, nil, remote.Client)
				test.Assert(t, err == nil, err)
				test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer, msg.TransInfo())
				test.Assert(t, msg.TransInfo().TransIntInfo()[transmeta.TransMessage] == context.Canceled.Error(),
					msg.TransInfo().TransIntInfo())
				return len(b), nil
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		cancel()
		st.waitSendLoopClose()
		test.Assert(t, st.sendState.isClosed(), st.recvState)
	})

	t.Run("write-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				return 0, io.ErrClosedPipe
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		sendReq := sendRequest{
			message:      msg,
			finishSignal: make(chan error, 1),
		}
		st.sendQueue <- sendReq
		err := sendReq.waitFinishSignal(st.Context())
		test.Assert(t, err != nil, err)
		test.Assert(t, st.sendState.isClosed(), st.recvState)
	})
}

func Test_ttheaderStream_notifyAllSendRequests(t *testing.T) {
	st := &ttheaderStream{
		sendQueue: make(chan sendRequest, 10),
	}
	n := 5
	wg := sync.WaitGroup{}
	wg.Add(n)
	for i := 0; i < n; i++ {
		req := sendRequest{finishSignal: make(chan error, 1)}
		st.sendQueue <- req
		go func() {
			defer wg.Done()
			_ = req.waitFinishSignal(context.Background())
		}()
	}
	st.notifyAllSendRequests()
	wg.Wait()
	// if everything works well, wg.Wait() will not block
}

func Test_ttheaderStream_writeMessage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		mockErr := errors.New("mock error")
		conn := &mockConn{
			write: func(b []byte) (int, error) {
				msg, err := decodeMessage(ctx, b, nil, remote.Client)
				test.Assert(t, err == nil, err)
				test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader, msg.TransInfo())
				return 0, mockErr
			},
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Server)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		err := st.writeMessage(msg)
		test.Assert(t, errors.Is(err, mockErr), err)
	})

	t.Run("encode-err", func(t *testing.T) {
		ri := mockRPCInfo()
		remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		data := &sendRequest{}
		msg := remote.NewMessage(data, nil, ri, remote.Stream, remote.Server)
		msg.SetProtocolInfo(remote.NewProtocolInfo(transport.TTHeader, serviceinfo.Thrift))
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeData
		err := st.writeMessage(msg)
		test.Assert(t, err != nil, err)
		test.Assert(t, strings.Contains(err.Error(), "thriftCodec"), err)
	})
}

func Test_ttheaderStream_clientReadMetaFrame(t *testing.T) {
	t.Run("recv-closed", func(t *testing.T) {
		st := &ttheaderStream{}
		st.recvState.setClosed()
		st.lastRecvError = io.EOF
		err := st.clientReadMetaFrame()
		test.Assert(t, err == io.EOF, err)
	})
	t.Run("success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeMeta
				buf, _ := encodeMessage(ctx, msg)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getClientFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Client, ext, handler)
		defer st.Close() // avoid goroutine leak

		err := st.clientReadMetaFrame()
		test.Assert(t, err == nil, err)
	})
}

func Test_ttheaderStream_serverReadHeaderMeta(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		opt := &remote.ServerOption{
			Option: remote.Option{
				StreamingMetaHandlers: []remote.StreamingMetaHandler{
					remote.NewCustomMetaHandler(remote.WithOnReadStream(
						func(ctx context.Context) (context.Context, error) {
							return context.WithValue(ctx, "key", "value"), nil
						},
					)).(remote.StreamingMetaHandler),
				},
			},
		}

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = "ToMethod"

		ctx, err := st.serverReadHeaderMeta(opt, msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, msg.RPCInfo().Invocation().MethodName() == "ToMethod")
		test.Assert(t, ctx.Value("key") == "value", ctx.Value("key"))
	})

	t.Run("frameHandler.ServerReadHeader:err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		opt := &remote.ServerOption{}

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader

		_, err := st.serverReadHeaderMeta(opt, msg)
		test.Assert(t, err != nil, err)
	})

	t.Run("OnReadStream-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		opt := &remote.ServerOption{
			Option: remote.Option{
				StreamingMetaHandlers: []remote.StreamingMetaHandler{
					remote.NewCustomMetaHandler(remote.WithOnReadStream(
						func(ctx context.Context) (context.Context, error) {
							return nil, errors.New("err")
						},
					)).(remote.StreamingMetaHandler),
				},
			},
		}

		msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
		msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = "ToMethod"

		_, err := st.serverReadHeaderMeta(opt, msg)
		test.Assert(t, err != nil, err)
	})
}

func Test_ttheaderStream_serverReadHeaderFrame(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = "ToMethod"
				buf, _ := encodeMessage(ctx, msg)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		opt := &remote.ServerOption{
			TracerCtl: &rpcinfo.TraceController{},
		}
		opt.TracerCtl.Append(&mockTracer{
			start: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "key", "value")
			},
		})

		ctx, err := st.serverReadHeaderFrame(opt)
		test.Assert(t, err == nil, err)
		test.Assert(t, ctx.Value("key") == "value", ctx.Value("key"))
		test.Assert(t, st.recvState.hasHeader(), st.recvState)
	})

	t.Run("read-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		opt := &remote.ServerOption{
			TracerCtl: &rpcinfo.TraceController{},
		}

		_, err := st.serverReadHeaderFrame(opt)
		test.Assert(t, err != nil, err)
	})

	t.Run("read-err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		opt := &remote.ServerOption{
			TracerCtl: &rpcinfo.TraceController{},
		}

		_, err := st.serverReadHeaderFrame(opt)
		test.Assert(t, err != nil, err)
	})

	t.Run("dirty-frame", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer
				buf, _ := encodeMessage(ctx, msg)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		handler := getServerFrameMetaHandler(nil)

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		opt := &remote.ServerOption{
			TracerCtl: &rpcinfo.TraceController{},
		}

		ctx, err := st.serverReadHeaderFrame(opt)
		test.Assert(t, errors.Is(err, ErrDirtyFrame), err)
	})

	t.Run("serverReadHeaderMeta:err", func(t *testing.T) {
		ri := mockRPCInfo()
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		conn := &mockConn{
			readBuf: func() []byte {
				msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = "ToMethod"
				buf, _ := encodeMessage(ctx, msg)
				return buf
			}(),
		}
		streamCodec := ttheader.NewStreamCodec()
		ext := &mockExtension{}
		mockErr := errors.New("mock error")
		handler := &mockFrameMetaHandler{
			serverReadHeader: func(ctx context.Context, msg remote.Message) (context.Context, error) {
				return nil, mockErr
			},
		}

		st := newTTHeaderStream(ctx, conn, ri, streamCodec, remote.Server, ext, handler)
		defer st.Close() // avoid goroutine leak

		opt := &remote.ServerOption{
			TracerCtl: &rpcinfo.TraceController{},
		}
		opt.TracerCtl.Append(&mockTracer{
			start: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, "key", "value")
			},
		})

		ctx, err := st.serverReadHeaderFrame(opt)
		test.Assert(t, errors.Is(err, mockErr), err)
	})
}
