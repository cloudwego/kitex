package jsonrpc

import (
	"context"
	"log"
	"sync/atomic"

	"github.com/cloudwego/kitex/streamx"
)

var (
	_ streamx.ClientStream                          = (*clientStream)(nil)
	_ streamx.ServerStream                          = (*serverStream)(nil)
	_ streamx.ClientStreamMetadata[Header, Trailer] = (*clientStream)(nil)
	_ streamx.ServerStreamMetadata[Header, Trailer] = (*serverStream)(nil)
)

func newStream(trans *transport, sid int, mode streamx.StreamingMode, service, method string) (s *stream) {
	s = new(stream)
	s.id = sid
	s.mode = mode
	s.service = service
	s.method = method
	s.trans = trans
	return s
}

type stream struct {
	id      int
	mode    streamx.StreamingMode
	service string
	method  string
	selfEOF int32
	peerEOF int32
	trans   *transport
}

func (s *stream) Header() (Header, error) {
	return make(Header), nil
}

func (s *stream) Trailer() (Trailer, error) {
	return make(Trailer), nil
}

func (s *stream) Mode() streamx.StreamingMode {
	return s.mode
}

func (s *stream) Service() string {
	return s.service
}

func (s *stream) Method() string {
	return s.method
}

func (s *stream) sendEOF() (err error) {
	if !atomic.CompareAndSwapInt32(&s.selfEOF, 0, 1) {
		return nil
	}
	log.Printf("stream[%s] send EOF", s.method)
	return s.trans.streamCloseSend(s)
}

func (s *stream) recvEOF() (err error) {
	if !atomic.CompareAndSwapInt32(&s.peerEOF, 0, 1) {
		return nil
	}
	log.Printf("stream[%s] recv EOF", s.method)
	return s.trans.streamCloseRecv(s)
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
	return s.sendEOF()
}

func newServerStream(s *stream) streamx.ServerStream {
	ss := &serverStream{stream: s}
	return ss
}

type serverStream struct {
	*stream
}

func (s *serverStream) SetHeader(hd Header) error {
	return nil
}

func (s *serverStream) SendHeader(hd Header) error {
	return nil
}

func (s *serverStream) SetTrailer(hd Trailer) error {
	return nil
}
