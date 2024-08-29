package ttstream

import (
	"context"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/kitex/streamx"
	"log"
	"sync/atomic"
)

var (
	_ streamx.ClientStream = (*clientStream)(nil)
	_ streamx.ServerStream = (*serverStream)(nil)
)

// TTClientStream cannot send header directly, should send from ctx
type TTClientStream interface {
	streamx.ClientStream
	RecvHeader() (map[string]string, error)
	RecvTrailer() (map[string]string, error)
}

// TTServerStream cannot read header directly, should read from ctx
type TTServerStream interface {
	streamx.ClientStream
	SetHeader(hd map[string]string) error
	SendHeader(hd map[string]string) error
	SetTrailer(hd map[string]string) error
}

func SendHeader(ss streamx.ServerStream, hd map[string]string) error {
	return ss.(TTServerStream).SendHeader(hd)
}

func newStream(trans *transport, mode streamx.StreamingMode, smeta streamMeta) (s *stream) {
	s = new(stream)
	s.streamMeta = smeta
	s.trans = trans
	s.mode = mode
	trans.storeStreamIO(s)
	return s
}

type streamMeta struct {
	sid     int32
	service string
	method  string
	header  map[string]string // key:value, key is full name
	trailer map[string]string
}

type stream struct {
	streamMeta
	trans   *transport
	mode    streamx.StreamingMode
	eof     int32 // eof handshake, 1: wait for EOF ack 2: eof hand shacked
	selfEOF int32
	peerEOF int32
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
	payload, err := EncodePayload(ctx, res)
	if err != nil {
		return err
	}
	return s.trans.streamSend(s.streamMeta, payload)
}

func (s *stream) RecvMsg(ctx context.Context, req any) error {
	payload, err := s.trans.streamRecv(s.streamMeta)
	if err != nil {
		return err
	}
	log.Printf("stream[%s] recv payload %v", s.method, payload)
	err = DecodePayload(ctx, payload, req.(thrift.FastCodec))
	log.Printf("stream[%s] decode payload %v", s.method, req)
	return err
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
