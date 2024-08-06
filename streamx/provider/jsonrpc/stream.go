package jsonrpc

import (
	"context"
	"github.com/cloudwego/kitex/streamx"
)

var (
	_ streamx.ClientStream = (*clientStream)(nil)
	_ streamx.ServerStream = (*serverStream)(nil)
)

func newStream(trans *transport, sid int, mode streamx.StreamingMode, method string) (s *stream) {
	s = new(stream)
	s.id = sid
	s.mode = mode
	s.method = method
	s.trans = trans
	return s
}

type stream struct {
	id     int
	mode   streamx.StreamingMode
	method string
	trans  *transport
}

func (s *stream) Mode() streamx.StreamingMode {
	return s.mode
}

func (s *stream) Method() string {
	return s.method
}

func (s *stream) close() error {
	return s.trans.streamEnd(s)
}

func (s *stream) SendMsg(ctx context.Context, res any) error {
	payload, err := EncodePayload(res)
	if err != nil {
		return err
	}
	return s.trans.streamSend(s, payload)
}

func (s *stream) RecvMsg(ctx context.Context, req any) error {
	payload, err := s.trans.streamRecv(s)
	if err != nil {
		return err
	}
	return DecodePayload(payload, req)
}

func newClientStream(s *stream) *clientStream {
	cs := &clientStream{stream: s}
	return cs
}

type clientStream struct {
	*stream
}

func (s *clientStream) CloseSend(ctx context.Context) error {
	return s.close()
}

func newServerStream(s *stream) streamx.ServerStream {
	ss := &serverStream{stream: s}
	return ss
}

type serverStream struct {
	*stream
}
