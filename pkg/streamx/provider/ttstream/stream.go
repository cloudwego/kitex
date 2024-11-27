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

package ttstream

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/streamx"
	terrors "github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/errors"
	"github.com/cloudwego/kitex/pkg/transmeta"
)

var (
	_ streamx.ClientStream         = (*clientStream)(nil)
	_ streamx.ServerStream         = (*serverStream)(nil)
	_ streamx.ClientStreamMetadata = (*clientStream)(nil)
	_ streamx.ServerStreamMetadata = (*serverStream)(nil)
	_ StreamMeta                   = (*stream)(nil)
)

func newStream(trans *transport, mode streamx.StreamingMode, smeta streamFrame) *stream {
	s := new(stream)
	s.streamFrame = smeta
	s.trans = trans
	s.mode = mode
	s.wheader = make(streamx.Header)
	s.wtrailer = make(streamx.Trailer)
	s.headerSig = make(chan int32, 1)
	s.trailerSig = make(chan int32, 1)
	s.StreamMeta = newStreamMeta()
	return s
}

type streamFrame struct {
	sid     int32
	method  string
	header  streamx.Header // key:value, key is full name
	trailer streamx.Trailer
}

const (
	streamSigNone     int32 = 0
	streamSigActive   int32 = 1
	streamSigInactive int32 = -1
)

type stream struct {
	streamFrame
	trans      *transport
	mode       streamx.StreamingMode
	wheader    streamx.Header  // wheader == nil means it already be sent
	wtrailer   streamx.Trailer // wtrailer == nil means it already be sent
	selfEOF    int32
	peerEOF    int32
	headerSig  chan int32
	trailerSig chan int32
	sio        *streamIO
	closedFlag int32 // 1 means stream is closed in exception scenario

	StreamMeta
	metaHandler MetaFrameHandler
	recvTimeout time.Duration
}

func (s *stream) Mode() streamx.StreamingMode {
	return s.mode
}

func (s *stream) Service() string {
	if len(s.header) == 0 {
		return ""
	}
	return s.header[ttheader.HeaderIDLServiceName]
}

func (s *stream) Method() string {
	return s.method
}

// close stream in exception scenario
func (s *stream) close(exception error) {
	if !atomic.CompareAndSwapInt32(&s.closedFlag, 0, 1) {
		return
	}
	select {
	case s.headerSig <- streamSigInactive:
	default:
	}
	select {
	case s.trailerSig <- streamSigInactive:
	default:
	}
	s.sio.close(exception)
	s.trans.streamDelete(s.sid)
}

func (s *stream) isClosed() bool {
	return atomic.LoadInt32(&s.closedFlag) == 1
}

func (s *stream) isSendFinished() bool {
	return atomic.LoadInt32(&s.selfEOF) == 1
}

func (s *stream) cancel() {
	s.sio.cancel()
}

func (s *stream) setMetaFrameHandler(h MetaFrameHandler) {
	s.metaHandler = h
}

func (s *stream) readMetaFrame(intHeader IntHeader, header streamx.Header, payload []byte) (err error) {
	if s.metaHandler == nil {
		return nil
	}
	return s.metaHandler.OnMetaFrame(s.StreamMeta, intHeader, header, payload)
}

func (s *stream) readHeader(hd streamx.Header) (err error) {
	if s.header != nil {
		return terrors.ErrUnexpectedHeader.WithCause(fmt.Errorf("stream[%d] already set header", s.sid))
	}
	s.header = hd
	select {
	case s.headerSig <- streamSigActive:
	default:
		return terrors.ErrUnexpectedHeader.WithCause(fmt.Errorf("stream[%d] already set header", s.sid))
	}
	klog.Debugf("stream[%s] read header: %v", s.method, hd)
	return nil
}

// writeHeader copy kvs into s.wheader
func (s *stream) writeHeader(hd streamx.Header) error {
	if s.wheader == nil {
		return fmt.Errorf("stream header already sent")
	}
	for k, v := range hd {
		s.wheader[k] = v
	}
	return nil
}

func (s *stream) sendHeader() (err error) {
	wheader := s.wheader
	s.wheader = nil
	if wheader == nil {
		return fmt.Errorf("stream header already sent")
	}
	err = s.trans.streamSendHeader(s, wheader)
	return err
}

// readTrailer by client: unblock recv function and return EOF if no unread frame
// readTrailer by server: unblock recv function and return EOF if no unread frame
func (s *stream) readTrailerFrame(fr *Frame) (err error) {
	if !atomic.CompareAndSwapInt32(&s.peerEOF, 0, 1) {
		return terrors.ErrUnexpectedTrailer.WithCause(fmt.Errorf("content: %v", fr))
	}

	var exception error
	// when server-side returns non-biz error, it will be wrapped as ApplicationException stored in trailer frame payload
	if len(fr.payload) > 0 {
		// exception is type of (*thrift.ApplicationException)
		_, _, err = thrift.UnmarshalFastMsg(fr.payload, nil)
		exception = terrors.ErrApplicationException.WithCause(err)
	} else {
		// when server-side returns biz error, payload is empty and biz error information is stored in trailer frame header
		bizErr, err := transmeta.ParseBizStatusErr(fr.trailer)
		if err != nil {
			exception = terrors.ErrIllegalBizErr.WithCause(err)
		} else if bizErr != nil {
			// bizErr is independent of rpc exception handling
			exception = bizErr
		}
	}
	s.trailer = fr.trailer
	select {
	case s.trailerSig <- streamSigActive:
	default:
		return terrors.ErrUnexpectedTrailer.WithCause(errors.New("already set trailer"))
	}
	select {
	case s.headerSig <- streamSigNone:
		// if trailer arrived, we should return unblock stream.Header()
	default:
	}

	klog.Debugf("stream[%d] recv trailer: %v, exception: %v", s.sid, s.trailer, exception)
	return s.trans.streamCloseRecv(s, exception)
}

func (s *stream) writeTrailer(tl streamx.Trailer) (err error) {
	if s.wtrailer == nil {
		return fmt.Errorf("stream trailer already sent")
	}
	for k, v := range tl {
		s.wtrailer[k] = v
	}
	return nil
}

func (s *stream) sendTrailer(ctx context.Context, ex tException) (err error) {
	if !atomic.CompareAndSwapInt32(&s.selfEOF, 0, 1) {
		return nil
	}
	wtrailer := s.wtrailer
	s.wtrailer = nil
	if wtrailer == nil {
		return fmt.Errorf("stream trailer already sent")
	}
	klog.Debugf("transport[%d]-stream[%d] send trailer", s.trans.kind, s.sid)
	return s.trans.streamCloseSend(s, wtrailer, ex)
}

func (s *stream) setRecvTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	s.recvTimeout = timeout
}

func (s *stream) SendMsg(ctx context.Context, res any) (err error) {
	err = s.trans.streamSend(ctx, s, res)
	return err
}

func (s *stream) RecvMsg(ctx context.Context, req any) error {
	if s.recvTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.recvTimeout)
		defer cancel()
	}
	return s.trans.streamRecv(ctx, s, req)
}

func newClientStream(s *stream) *clientStream {
	cs := &clientStream{stream: s}
	return cs
}

type clientStream struct {
	*stream
}

func (s *clientStream) RecvMsg(ctx context.Context, req any) error {
	return s.stream.RecvMsg(ctx, req)
}

func (s *clientStream) CloseSend(ctx context.Context) error {
	return s.sendTrailer(ctx, nil)
}

func newServerStream(s *stream) streamx.ServerStream {
	ss := &serverStream{stream: s}
	return ss
}

type serverStream struct {
	*stream
}

func (s *serverStream) RecvMsg(ctx context.Context, req any) error {
	return s.stream.RecvMsg(ctx, req)
}

// SendMsg should send left header first
func (s *serverStream) SendMsg(ctx context.Context, res any) error {
	if len(s.wheader) > 0 {
		if err := s.sendHeader(); err != nil {
			return err
		}
	}
	return s.stream.SendMsg(ctx, res)
}

// close will be called after server handler returned
// after close stream cannot be access again
func (s *serverStream) close(ex tException) error {
	// write loop should help to delete stream
	err := s.sendTrailer(context.Background(), ex)
	if err != nil {
		return err
	}
	s.stream.close(ex)
	return nil
}
