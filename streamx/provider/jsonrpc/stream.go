package jsonrpc

import (
	"context"
	"github.com/cloudwego/kitex/streamx"
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
	id     int
	mode   streamx.StreamingMode
	method string
	eof    int32 // eof handshake, eof==2 means it's time to close the whole stream
	trans  *transport
}

func (s *stream) Mode() streamx.StreamingMode {
	return s.mode
}

func (s *stream) Method() string {
	return s.method
}

func (s *stream) close() (closed bool, err error) {
	eof := atomic.AddInt32(&s.eof, 1)
	closed = eof >= 2
	if eof <= 2 {
		err = s.trans.streamEnd(s)
		if err != nil {
			return
		}
	}
	if eof == 2 {
		err = s.trans.streamClose(s)
	}
	return
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
	_, err := s.close()
	return err
}

func newServerStream(s *stream) streamx.ServerStream {
	ss := &serverStream{stream: s}
	return ss
}

type serverStream struct {
	*stream
}
