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
	"github.com/cloudwego/kitex/pkg/transmeta"
)

var (
	_ streamx.ClientStream         = (*clientStream)(nil)
	_ streamx.ServerStream         = (*serverStream)(nil)
	_ streamx.ClientStreamMetadata = (*clientStream)(nil)
	_ streamx.ServerStreamMetadata = (*serverStream)(nil)
	_ StreamMeta                   = (*stream)(nil)
)

func newStream(ctx context.Context, trans *transport, mode streamx.StreamingMode, smeta streamFrame) (s *stream) {
	s = new(stream)
	s.streamFrame = smeta
	s.trans = trans
	s.mode = mode
	s.headerSig = make(chan struct{})
	s.trailerSig = make(chan struct{})
	s.StreamMeta = newStreamMeta()
	trans.storeStreamIO(ctx, s)
	return s
}

type streamFrame struct {
	sid     int32
	method  string
	header  streamx.Header // key:value, key is full name
	trailer streamx.Trailer
}

type stream struct {
	streamFrame
	trans      *transport
	mode       streamx.StreamingMode
	wheader    streamx.Header
	wtrailer   streamx.Trailer
	selfEOF    int32
	peerEOF    int32
	headerSig  chan struct{}
	trailerSig chan struct{}
	err        error

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

func (s *stream) close() {
	select {
	case <-s.headerSig:
	default:
		close(s.headerSig)
	}
	select {
	case <-s.trailerSig:
	default:
		close(s.trailerSig)
	}
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
	s.header = hd
	select {
	case <-s.headerSig:
		return errors.New("already set header")
	default:
		close(s.headerSig)
	}
	klog.Debugf("stream[%s] read header: %v", s.method, hd)
	return nil
}

// setHeader use the hd as the underlying header
func (s *stream) setHeader(hd streamx.Header) {
	s.wheader = hd
	return
}

// writeHeader copy kvs into s.wheader
func (s *stream) writeHeader(hd streamx.Header) {
	if s.wheader == nil {
		s.wheader = make(streamx.Header)
	}
	for k, v := range hd {
		s.wheader[k] = v
	}
}

func (s *stream) sendHeader() (err error) {
	wheader := s.wheader
	s.wheader = nil
	err = s.trans.streamSendHeader(s.sid, s.method, wheader)
	return err
}

// readTrailer by client: unblock recv function and return EOF if no unread frame
// readTrailer by server: unblock recv function and return EOF if no unread frame
func (s *stream) readTrailerFrame(fr *Frame) (err error) {
	if !atomic.CompareAndSwapInt32(&s.peerEOF, 0, 1) {
		return nil
	}

	if len(fr.payload) > 0 {
		_, _, ex := thrift.UnmarshalFastMsg(fr.payload, nil)
		s.err = ex.(*thrift.ApplicationException)
	} else {
		bizErr, err := transmeta.ParseBizStatusErr(fr.trailer)
		if err != nil {
			s.err = err
		} else if bizErr != nil {
			s.err = bizErr
		}
	}
	s.trailer = fr.trailer
	select {
	case <-s.trailerSig:
		return errors.New("already set trailer")
	default:
		close(s.trailerSig)
	}

	klog.Debugf("stream[%d] recv trailer: %v, err: %v", s.sid, s.trailer, s.err)
	return s.trans.streamCloseRecv(s)
}

func (s *stream) writeTrailer(tl streamx.Trailer) (err error) {
	if s.wtrailer == nil {
		s.wtrailer = make(streamx.Trailer)
	}
	for k, v := range tl {
		s.wtrailer[k] = v
	}
	return nil
}

func (s *stream) appendTrailer(kvs ...string) (err error) {
	if len(kvs)%2 != 0 {
		return fmt.Errorf("got the odd number of input kvs for Trailer: %d", len(kvs))
	}
	if s.wtrailer == nil {
		s.wtrailer = make(streamx.Trailer)
	}
	var key string
	for i, str := range kvs {
		if i%2 == 0 {
			key = str
			continue
		}
		s.wtrailer[key] = str
	}
	return nil
}

func (s *stream) sendTrailer(ctx context.Context, ex tException) (err error) {
	if !atomic.CompareAndSwapInt32(&s.selfEOF, 0, 1) {
		return nil
	}
	klog.Debugf("stream[%d] send trialer", s.sid)
	return s.trans.streamCloseSend(s.sid, s.method, s.wtrailer, ex)
}

func (s *stream) finished() bool {
	return atomic.LoadInt32(&s.peerEOF) == 1 &&
		atomic.LoadInt32(&s.selfEOF) == 1
}

func (s *stream) setRecvTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	s.recvTimeout = timeout
}

func (s *stream) SendMsg(ctx context.Context, res any) (err error) {
	err = s.trans.streamSend(ctx, s.sid, s.method, s.wheader, res)
	return err
}

func (s *stream) RecvMsg(ctx context.Context, req any) error {
	if s.recvTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.recvTimeout)
		defer cancel()
	}
	return s.trans.streamRecv(ctx, s.sid, req)
}

func (s *stream) cancel() {
	_ = s.trans.streamCancel(s)
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

// after close stream cannot be access again
func (s *clientStream) close() error {
	// client should CloseSend first and then close stream
	err := s.sendTrailer(context.Background(), nil)
	if err != nil {
		return err
	}
	err = s.trans.streamClose(s.sid)
	s.stream.close()
	return err
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
	err = s.trans.streamClose(s.sid)
	s.stream.close()
	return err
}
