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

	ep "github.com/cloudwego/kitex/pkg/endpoint"
	sep "github.com/cloudwego/kitex/pkg/endpoint/server"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type grpcServerStream struct {
	grpcRecv ep.RecvEndpoint
	grpcSend ep.SendEndpoint
	sx       *serverStream
}

type serverStream struct {
	ctx     context.Context
	svcInfo *serviceinfo.ServiceInfo
	conn    net.Conn // clientConn or serverConn
	handler remote.TransReadWriter
	recv    sep.StreamRecvEndpoint
	send    sep.StreamSendEndpoint
	// for grpc compatibility
	grpcStream *grpcServerStream
}

func (s *serverStream) GetGRPCStream() streaming.Stream {
	return s.grpcStream
}

// newServerStream ...
func newServerStream(ctx context.Context, svcInfo *serviceinfo.ServiceInfo, conn net.Conn, handler remote.TransReadWriter) *serverStream {
	sx := &serverStream{
		ctx:     ctx,
		svcInfo: svcInfo,
		conn:    conn,
		handler: handler,
	}
	sx.grpcStream = &grpcServerStream{
		sx: sx,
	}
	return sx
}

func (s *grpcServerStream) Context() context.Context {
	return s.sx.ctx
}

func (s *grpcServerStream) Trailer() metadata.MD {
	panic("this method should only be used in client side!")
}

func (s *grpcServerStream) Header() (metadata.MD, error) {
	panic("this method should only be used in client side!")
}

// SendHeader is used for server side grpcServerStream
func (s *grpcServerStream) SendHeader(md metadata.MD) error {
	sc := s.sx.conn.(*serverConn)
	return sc.s.SendHeader(md)
}

// SetHeader is used for server side grpcServerStream
func (s *grpcServerStream) SetHeader(md metadata.MD) error {
	sc := s.sx.conn.(*serverConn)
	return sc.s.SetHeader(md)
}

// SetTrailer is used for server side grpcServerStream
func (s *grpcServerStream) SetTrailer(md metadata.MD) {
	sc := s.sx.conn.(*serverConn)
	sc.s.SetTrailer(md)
}

func (s *grpcServerStream) RecvMsg(m interface{}) error {
	return s.sx.RecvMsg(context.Background(), m)
}

func (s *grpcServerStream) SendMsg(m interface{}) error {
	return s.sx.SendMsg(context.Background(), m)
}

func (s *grpcServerStream) Close() error {
	return s.sx.conn.Close()
}

func (s *serverStream) SetHeader(hd streaming.Header) error {
	sc := s.conn.(*serverConn)
	return sc.s.SetHeader(streamingHeaderToHTTP2MD(hd))
}

func (s *serverStream) SendHeader(hd streaming.Header) error {
	sc := s.conn.(*serverConn)
	return sc.s.SendHeader(streamingHeaderToHTTP2MD(hd))
}

func (s *serverStream) SetTrailer(tl streaming.Trailer) error {
	sc := s.conn.(*serverConn)
	return sc.s.SetTrailer(streamingTrailerToHTTP2MD(tl))
}

func (s *serverStream) RecvMsg(ctx context.Context, m interface{}) error {
	ri := rpcinfo.GetRPCInfo(s.ctx)

	msg := remote.NewMessage(m, s.svcInfo, ri, remote.Stream, remote.Client)
	payloadCodec, err := s.getPayloadCodecFromContentType()
	if err != nil {
		return err
	}
	msg.SetProtocolInfo(remote.NewProtocolInfo(ri.Config().TransportProtocol(), payloadCodec))
	defer msg.Recycle()

	_, err = s.handler.Read(s.ctx, s.conn, msg)
	return err
}

func (s *serverStream) SendMsg(ctx context.Context, m interface{}) error {
	ri := rpcinfo.GetRPCInfo(s.ctx)

	msg := remote.NewMessage(m, s.svcInfo, ri, remote.Stream, remote.Client)
	payloadCodec, err := s.getPayloadCodecFromContentType()
	if err != nil {
		return err
	}
	msg.SetProtocolInfo(remote.NewProtocolInfo(ri.Config().TransportProtocol(), payloadCodec))
	defer msg.Recycle()

	_, err = s.handler.Write(s.ctx, s.conn, msg)
	return err
}

func (s *serverStream) getPayloadCodecFromContentType() (serviceinfo.PayloadCodec, error) {
	// TODO: handle other protocols in the future. currently only supports grpc
	var subType string
	switch sc := s.conn.(type) {
	case *clientConn:
		subType = sc.s.ContentSubtype()
	case *serverConn:
		subType = sc.s.ContentSubtype()
	}
	switch subType {
	case contentSubTypeThrift:
		return serviceinfo.Thrift, nil
	default:
		return serviceinfo.Protobuf, nil
	}
}

type grpcClientStream struct {
	sx *clientStream
}

type clientStream struct {
	ctx     context.Context
	svcInfo *serviceinfo.ServiceInfo
	conn    net.Conn // clientConn or serverConn
	handler remote.TransReadWriter
	// for grpc compatibility
	grpcStream *grpcClientStream
}

func (s *clientStream) GetGRPCStream() streaming.Stream {
	return s.grpcStream
}

// NewClientStream ...
func NewClientStream(ctx context.Context, svcInfo *serviceinfo.ServiceInfo, conn net.Conn, handler remote.TransReadWriter) streaming.ClientStream {
	sx := &clientStream{
		ctx:     ctx,
		svcInfo: svcInfo,
		conn:    conn,
		handler: handler,
	}
	sx.grpcStream = &grpcClientStream{
		sx: sx,
	}
	return sx
}

func (s *grpcClientStream) Context() context.Context {
	return s.sx.ctx
}

// Trailer is used for client side grpcServerStream
func (s *grpcClientStream) Trailer() metadata.MD {
	sc := s.sx.conn.(*clientConn)
	return sc.s.Trailer()
}

// Header is used for client side grpcServerStream
func (s *grpcClientStream) Header() (metadata.MD, error) {
	sc := s.sx.conn.(*clientConn)
	return sc.s.Header()
}

func (s *grpcClientStream) Close() error {
	return s.sx.conn.Close()
}

func (s *grpcClientStream) SendHeader(md metadata.MD) error {
	panic("this method should only be used in server side!")
}

func (s *grpcClientStream) SetHeader(md metadata.MD) error {
	panic("this method should only be used in server side!")
}

func (s *grpcClientStream) SetTrailer(md metadata.MD) {
	panic("this method should only be used in server side!")
}

func (s *grpcClientStream) RecvMsg(m interface{}) error {
	return s.sx.RecvMsg(context.Background(), m)
}

func (s *grpcClientStream) SendMsg(m interface{}) error {
	return s.sx.SendMsg(context.Background(), m)
}

func (s *clientStream) Header() (streaming.Header, error) {
	sc := s.conn.(*clientConn)
	hd, err := sc.Header()
	if err != nil {
		return nil, err
	}
	return http2MDToStreamingHeader(hd), nil
}

func (s *clientStream) Trailer() (streaming.Trailer, error) {
	sc := s.conn.(*clientConn)
	tl := sc.Trailer()
	return http2MDToStreamingTrailer(tl), nil
}

func (s *clientStream) RecvMsg(ctx context.Context, m interface{}) error {
	ri := rpcinfo.GetRPCInfo(s.ctx)

	msg := remote.NewMessage(m, s.svcInfo, ri, remote.Stream, remote.Client)
	payloadCodec, err := s.getPayloadCodecFromContentType()
	if err != nil {
		return err
	}
	msg.SetProtocolInfo(remote.NewProtocolInfo(ri.Config().TransportProtocol(), payloadCodec))
	defer msg.Recycle()

	_, err = s.handler.Read(s.ctx, s.conn, msg)
	return err
}

func (s *clientStream) SendMsg(ctx context.Context, m interface{}) error {
	ri := rpcinfo.GetRPCInfo(s.ctx)

	msg := remote.NewMessage(m, s.svcInfo, ri, remote.Stream, remote.Client)
	payloadCodec, err := s.getPayloadCodecFromContentType()
	if err != nil {
		return err
	}
	msg.SetProtocolInfo(remote.NewProtocolInfo(ri.Config().TransportProtocol(), payloadCodec))
	defer msg.Recycle()

	_, err = s.handler.Write(s.ctx, s.conn, msg)
	return err
}

func (s *clientStream) CloseSend() error {
	return s.conn.Close()
}

func (s *clientStream) getPayloadCodecFromContentType() (serviceinfo.PayloadCodec, error) {
	// TODO: handle other protocols in the future. currently only supports grpc
	var subType string
	switch sc := s.conn.(type) {
	case *clientConn:
		subType = sc.s.ContentSubtype()
	case *serverConn:
		subType = sc.s.ContentSubtype()
	}
	switch subType {
	case contentSubTypeThrift:
		return serviceinfo.Thrift, nil
	default:
		return serviceinfo.Protobuf, nil
	}
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
