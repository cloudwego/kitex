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

package nphttp2

import (
	"context"
	"net"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type serverStream struct {
	sx *serverStreamX
}

type serverStreamX struct {
	ctx     context.Context
	svcInfo *serviceinfo.ServiceInfo
	conn    *serverConn
	pipe    *remote.ServerStreamPipeline
	t       *svrTransHandler
	rawConn net.Conn
	// for grpc compatibility
	grpcStream *serverStream
}

func (s *serverStreamX) GetGRPCStream() streaming.Stream {
	return s.grpcStream
}

func (s *serverStreamX) SetPipeline(pipe *remote.ServerStreamPipeline) {
	s.pipe = pipe
}

// newServerStream ...
func newServerStream(ctx context.Context, svcInfo *serviceinfo.ServiceInfo, conn *serverConn, t *svrTransHandler, rawConn net.Conn) *serverStreamX {
	sx := &serverStreamX{
		ctx:     ctx,
		svcInfo: svcInfo,
		conn:    conn,
		t:       t,
		rawConn: rawConn,
	}
	sx.grpcStream = &serverStream{
		sx: sx,
	}
	return sx
}

func (s *serverStream) Context() context.Context {
	return s.sx.ctx
}

func (s *serverStream) Trailer() metadata.MD {
	panic("this method should only be used in client side!")
}

func (s *serverStream) Header() (metadata.MD, error) {
	panic("this method should only be used in client side!")
}

// SendHeader is used for server side serverStream
func (s *serverStream) SendHeader(md metadata.MD) error {
	return s.sx.conn.s.SendHeader(md)
}

// SetHeader is used for server side serverStream
func (s *serverStream) SetHeader(md metadata.MD) error {
	return s.sx.conn.s.SetHeader(md)
}

// SetTrailer is used for server side serverStream
func (s *serverStream) SetTrailer(md metadata.MD) {
	s.sx.conn.s.SetTrailer(md)
}

func (s *serverStream) RecvMsg(m interface{}) error {
	return s.sx.RecvMsg(s.sx.ctx, m)
}

func (s *serverStream) SendMsg(m interface{}) error {
	return s.sx.SendMsg(s.sx.ctx, m)
}

func (s *serverStream) Close() error {
	return s.sx.conn.Close()
}

func (s *serverStreamX) Write(ctx context.Context, msg remote.Message) (nctx context.Context, err error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if bizStatusErr := ri.Invocation().BizStatusErr(); bizStatusErr != nil {
		// BizError: do not send the message
		var st *status.Status
		if sterr, ok := bizStatusErr.(status.Iface); ok {
			st = sterr.GRPCStatus()
		} else {
			st = status.New(codes.Internal, bizStatusErr.BizMessage())
		}
		grpcStream := s.conn.s
		grpcStream.SetBizStatusErr(bizStatusErr)
		s.conn.tr.WriteStatus(grpcStream, st)
		return ctx, nil
	}
	buf := newBuffer(s.conn)
	defer buf.Release(err)

	if err = s.t.codec.Encode(ctx, msg, buf); err != nil {
		return ctx, err
	}
	return ctx, buf.Flush()
}

func (s *serverStreamX) Read(ctx context.Context, msg remote.Message) (nctx context.Context, err error) {
	buf := newBuffer(s.conn)
	defer buf.Release(err)

	err = s.t.codec.Decode(ctx, msg, buf)
	return ctx, err
}

func (s *serverStreamX) SetHeader(hd streaming.Header) error {
	return s.conn.s.SendHeader(streamingHeaderToHTTP2MD(hd))
}

func (s *serverStreamX) SendHeader(hd streaming.Header) error {
	return s.conn.s.SendHeader(streamingHeaderToHTTP2MD(hd))
}

func (s *serverStreamX) SetTrailer(tl streaming.Trailer) error {
	return s.conn.s.SetTrailer(streamingTrailerToHTTP2MD(tl))
}

func (s *serverStreamX) RecvMsg(ctx context.Context, m interface{}) error {
	ri := rpcinfo.GetRPCInfo(ctx)

	msg := remote.NewMessage(m, s.svcInfo, ri, remote.Stream, remote.Server)
	defer msg.Recycle()

	_, err := s.pipe.Read(ctx, msg)
	return err
}

func (s *serverStreamX) SendMsg(ctx context.Context, m interface{}) error {
	ri := rpcinfo.GetRPCInfo(ctx)

	msg := remote.NewMessage(m, s.svcInfo, ri, remote.Stream, remote.Server)
	defer msg.Recycle()

	_, err := s.pipe.Write(ctx, msg)
	return err
}

type clientStream struct {
	sx *clientStreamX
}

type clientStreamX struct {
	ctx     context.Context
	svcInfo *serviceinfo.ServiceInfo
	conn    *clientConn
	pipe    *remote.ClientStreamPipeline
	t       *cliTransHandler
	// for grpc compatibility
	grpcStream *clientStream
}

func (s *clientStreamX) GetGRPCStream() streaming.Stream {
	return s.grpcStream
}

// newClientStream ...
func newClientStream(ctx context.Context, svcInfo *serviceinfo.ServiceInfo, conn *clientConn, t *cliTransHandler) *clientStreamX {
	sx := &clientStreamX{
		ctx:     ctx,
		svcInfo: svcInfo,
		conn:    conn,
		t:       t,
	}
	sx.grpcStream = &clientStream{
		sx: sx,
	}
	return sx
}

func (s *clientStream) Context() context.Context {
	return s.sx.ctx
}

// Trailer is used for client side serverStream
func (s *clientStream) Trailer() metadata.MD {
	return s.sx.conn.s.Trailer()
}

// Header is used for client side serverStream
func (s *clientStream) Header() (metadata.MD, error) {
	return s.sx.conn.s.Header()
}

func (s *clientStream) Close() error {
	return s.sx.conn.Close()
}

func (s *clientStream) SendHeader(md metadata.MD) error {
	panic("this method should only be used in server side!")
}

func (s *clientStream) SetHeader(md metadata.MD) error {
	panic("this method should only be used in server side!")
}

func (s *clientStream) SetTrailer(md metadata.MD) {
	panic("this method should only be used in server side!")
}

func (s *clientStream) RecvMsg(m interface{}) error {
	return s.sx.RecvMsg(s.sx.ctx, m)
}

func (s *clientStream) SendMsg(m interface{}) error {
	return s.sx.SendMsg(s.sx.ctx, m)
}

func (s *clientStreamX) SetPipeline(pipe *remote.ClientStreamPipeline) {
	s.pipe = pipe
}

func (s *clientStreamX) Header() (streaming.Header, error) {
	hd, err := s.conn.s.Header()
	if err != nil {
		return nil, err
	}
	return http2MDToStreamingHeader(hd), nil
}

func (s *clientStreamX) Trailer() (streaming.Trailer, error) {
	tl := s.conn.s.Trailer()
	return http2MDToStreamingTrailer(tl), nil
}

func (s *clientStreamX) RecvMsg(ctx context.Context, m interface{}) error {
	ri := rpcinfo.GetRPCInfo(ctx)

	msg := remote.NewMessage(m, s.svcInfo, ri, remote.Stream, remote.Client)
	defer msg.Recycle()

	_, err := s.pipe.Read(ctx, msg)
	return err
}

func (s *clientStreamX) SendMsg(ctx context.Context, m interface{}) error {
	ri := rpcinfo.GetRPCInfo(ctx)

	msg := remote.NewMessage(m, s.svcInfo, ri, remote.Stream, remote.Client)
	defer msg.Recycle()

	_, err := s.pipe.Write(ctx, msg)
	return err
}

func (s *clientStreamX) CloseSend() error {
	return s.conn.Close()
}

func (s *clientStreamX) Write(ctx context.Context, msg remote.Message) (nctx context.Context, err error) {
	buf := newBuffer(s.conn)
	defer buf.Release(err)
	if err = s.t.codec.Encode(ctx, msg, buf); err != nil {
		return ctx, err
	}
	return ctx, buf.Flush()
}

func (s *clientStreamX) Read(ctx context.Context, msg remote.Message) (nctx context.Context, err error) {
	buf := newBuffer(s.conn)
	defer buf.Release(err)

	// set recv grpc compressor at client to decode the pack from server
	ri := msg.RPCInfo()
	remote.SetRecvCompressor(ri, s.conn.GetRecvCompress())
	err = s.t.codec.Decode(ctx, msg, buf)
	if bizStatusErr, isBizErr := kerrors.FromBizStatusError(err); isBizErr {
		if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
			setter.SetBizStatusErr(bizStatusErr)
			return ctx, nil
		}
	}
	ctx = receiveHeaderAndTrailer(ctx, s.conn)
	return ctx, err
}

func streamingHeaderToHTTP2MD(header streaming.Header) metadata.MD {
	md := metadata.MD{}
	for k, v := range header {
		md.Append(k, v)
	}
	return md
}

func streamingTrailerToHTTP2MD(trailer streaming.Trailer) metadata.MD {
	md := metadata.MD{}
	for k, v := range trailer {
		md.Append(k, v)
	}
	return md
}

func http2MDToStreamingHeader(md metadata.MD) streaming.Header {
	header := streaming.Header{}
	for k, v := range md {
		if len(v) > 1 {
			panic("cannot convert http2 metadata to streaming header, because the value of key " + k + " is more than one")
		}
		header[k] = v[0]
	}
	return header
}

func http2MDToStreamingTrailer(md metadata.MD) streaming.Trailer {
	trailer := streaming.Trailer{}
	for k, v := range md {
		if len(v) > 1 {
			panic("cannot convert http2 metadata to streaming trailer, because the value of key " + k + " is more than one")
		}
		trailer[k] = v[0]
	}
	return trailer
}
