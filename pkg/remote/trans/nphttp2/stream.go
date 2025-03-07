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
	"errors"
	"net"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type grpcServerStream struct {
	sx *serverStream
}

type serverStream struct {
	ctx     context.Context
	rpcInfo rpcinfo.RPCInfo
	svcInfo *serviceinfo.ServiceInfo
	conn    *serverConn
	handler remote.TransReadWriter
	// for grpc compatibility
	grpcStream *grpcServerStream
}

var _ streaming.GRPCStreamGetter = (*serverStream)(nil)

func (s *serverStream) GetGRPCStream() streaming.Stream {
	return s.grpcStream
}

// newServerStream ...
func newServerStream(ctx context.Context, svcInfo *serviceinfo.ServiceInfo, conn *serverConn, handler remote.TransReadWriter) *serverStream {
	sx := &serverStream{
		ctx:     ctx,
		rpcInfo: rpcinfo.GetRPCInfo(ctx),
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
	return s.sx.conn.s.SendHeader(md)
}

// SetHeader is used for server side grpcServerStream
func (s *grpcServerStream) SetHeader(md metadata.MD) error {
	return s.sx.conn.s.SetHeader(md)
}

// SetTrailer is used for server side grpcServerStream
func (s *grpcServerStream) SetTrailer(md metadata.MD) {
	s.sx.conn.s.SetTrailer(md)
}

func (s *grpcServerStream) RecvMsg(m interface{}) error {
	return s.sx.RecvMsg(s.sx.ctx, m)
}

func (s *grpcServerStream) SendMsg(m interface{}) error {
	return s.sx.SendMsg(s.sx.ctx, m)
}

func (s *grpcServerStream) Close() error {
	return s.sx.conn.Close()
}

func (s *serverStream) SetHeader(hd streaming.Header) error {
	return s.conn.s.SetHeader(streamingHeaderToHTTP2MD(hd))
}

func (s *serverStream) SendHeader(hd streaming.Header) error {
	return s.conn.s.SendHeader(streamingHeaderToHTTP2MD(hd))
}

func (s *serverStream) SetTrailer(tl streaming.Trailer) error {
	return s.conn.s.SetTrailer(streamingTrailerToHTTP2MD(tl))
}

// RecvMsg receives a message from the client.
// In order to avoid underlying execution errors when the context passed in by the user does not
// contain information related to this RPC, the context specified when creating the stream is used
// here, and the context passed in by the user is ignored.
func (s *serverStream) RecvMsg(ctx context.Context, m interface{}) error {
	ri := s.rpcInfo

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

// SendMsg sends a message to the client.
// context handling logic is the same as RecvMsg.
func (s *serverStream) SendMsg(ctx context.Context, m interface{}) error {
	ri := s.rpcInfo

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
	subType := s.conn.s.ContentSubtype()
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
	rpcInfo rpcinfo.RPCInfo
	svcInfo *serviceinfo.ServiceInfo
	conn    *clientConn
	handler remote.TransReadWriter
	// for grpc compatibility
	grpcStream *grpcClientStream
}

var _ streaming.GRPCStreamGetter = (*clientStream)(nil)

func (s *clientStream) GetGRPCStream() streaming.Stream {
	return s.grpcStream
}

// NewClientStream ...
func NewClientStream(ctx context.Context, svcInfo *serviceinfo.ServiceInfo, conn net.Conn, handler remote.TransReadWriter) streaming.ClientStream {
	sx := &clientStream{
		ctx:     ctx,
		rpcInfo: rpcinfo.GetRPCInfo(ctx),
		svcInfo: svcInfo,
		handler: handler,
	}
	sx.conn, _ = conn.(*clientConn)
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
	return s.sx.conn.s.Trailer()
}

// Header is used for client side grpcServerStream
func (s *grpcClientStream) Header() (metadata.MD, error) {
	return s.sx.conn.s.Header()
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
	return s.sx.RecvMsg(s.sx.ctx, m)
}

func (s *grpcClientStream) SendMsg(m interface{}) error {
	return s.sx.SendMsg(s.sx.ctx, m)
}

func (s *clientStream) Header() (streaming.Header, error) {
	hd, err := s.conn.Header()
	if err != nil {
		return nil, err
	}
	return http2MDToStreamingHeader(hd)
}

func (s *clientStream) Trailer() (streaming.Trailer, error) {
	tl := s.conn.Trailer()
	return http2MDToStreamingTrailer(tl)
}

// RecvMsg receives a message from the server.
// context handling logic is the same as serverStream.RecvMsg.
func (s *clientStream) RecvMsg(ctx context.Context, m interface{}) error {
	ri := s.rpcInfo

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

// SendMsg sends a message to the server.
// context handling logic is the same as serverStream.RecvMsg.
func (s *clientStream) SendMsg(ctx context.Context, m interface{}) error {
	ri := s.rpcInfo

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

func (s *clientStream) CloseSend(ctx context.Context) error {
	return s.conn.Close()
}

func (s *clientStream) Context() context.Context {
	return s.ctx
}

func (s *clientStream) getPayloadCodecFromContentType() (serviceinfo.PayloadCodec, error) {
	var subType string
	if s.conn != nil {
		// actually it must be non nil, but for unit testing compatibility, we still need to check it
		subType = s.conn.s.ContentSubtype()
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

var handleStreamingMetadataMultipleValues func(k string, v []string) string

// HandleStreamingMetadataMultipleValues is used to register a handler to handle the scene
// when the value of the key is zero or more than one.
// e.g.
//
//	nphttp2.HandleStreamingMetadataMultipleValues(func(k string, v []string) string {
//		return v[0]
//	})
func HandleStreamingMetadataMultipleValues(hd func(k string, v []string) string) {
	handleStreamingMetadataMultipleValues = hd
}

func http2MDToStreamingHeader(md metadata.MD) (streaming.Header, error) {
	header := streaming.Header{}
	for k, v := range md {
		if len(v) > 1 || len(v) == 0 {
			if handleStreamingMetadataMultipleValues != nil {
				header[k] = handleStreamingMetadataMultipleValues(k, v)
				continue
			}
			return nil, errors.New("cannot convert http2 metadata to streaming header, because the value of key " + k + " is zero or more than one")
		}
		header[k] = v[0]
	}
	return header, nil
}

func http2MDToStreamingTrailer(md metadata.MD) (streaming.Trailer, error) {
	trailer := streaming.Trailer{}
	for k, v := range md {
		if len(v) > 1 || len(v) == 0 {
			if handleStreamingMetadataMultipleValues != nil {
				trailer[k] = handleStreamingMetadataMultipleValues(k, v)
				continue
			}
			return nil, errors.New("cannot convert http2 metadata to streaming trailer, because the value of key " + k + " is zero or more than one")
		}
		trailer[k] = v[0]
	}
	return trailer, nil
}
