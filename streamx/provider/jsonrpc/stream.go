package jsonrpc

import (
	"context"
	"github.com/cloudwego/kitex/streamx"
	"log"
	"sync/atomic"
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
	id      int
	mode    streamx.StreamingMode
	method  string
	eof     int32 // eof handshake, 1: wait for EOF ack 2: eof hand shacked
	selfEOF int32
	peerEOF int32
	trans   *transport
}

func (s *stream) Mode() streamx.StreamingMode {
	return s.mode
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
