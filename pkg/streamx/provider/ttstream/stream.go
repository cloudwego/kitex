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
	"sync/atomic"

	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/streamx"
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

	StreamMeta
	metaHandler MetaFrameHandler
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

func (s *stream) writeHeader(hd streamx.Header) (err error) {
	if s.wheader == nil {
		s.wheader = make(streamx.Header)
	}
	for k, v := range hd {
		s.wheader[k] = v
	}
	return nil
}

func (s *stream) sendHeader() (err error) {
	wheader := s.wheader
	s.wheader = nil
	err = s.trans.streamSendHeader(s.sid, s.method, wheader)
	return err
}

// readTrailer by client: unblock recv function and return EOF if no unread frame
// readTrailer by server: unblock recv function and return EOF if no unread frame
func (s *stream) readTrailer(tl streamx.Trailer) (err error) {
	if !atomic.CompareAndSwapInt32(&s.peerEOF, 0, 1) {
		return nil
	}

	s.trailer = tl
	select {
	case <-s.trailerSig:
		return errors.New("already set trailer")
	default:
		close(s.trailerSig)
	}

	klog.Debugf("stream[%d] recv trailer: %v", s.sid, tl)
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

func (s *stream) sendTrailer() (err error) {
	if !atomic.CompareAndSwapInt32(&s.selfEOF, 0, 1) {
		return nil
	}
	klog.Debugf("stream[%d] send trialer", s.sid)
	return s.trans.streamSendTrailer(s.sid, s.method, s.wtrailer)
}

func (s *stream) SendMsg(ctx context.Context, res any) (err error) {
	err = s.trans.streamSend(s.sid, s.method, s.wheader, res)
	return err
}

func (s *stream) RecvMsg(ctx context.Context, req any) error {
	return s.trans.streamRecv(s.sid, req)
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

func (s *clientStream) CloseSend(ctx context.Context) error {
	return s.sendTrailer()
}

func (s *clientStream) close() error {
	return s.trans.streamClose(s.stream)
}

func newServerStream(s *stream) streamx.ServerStream {
	ss := &serverStream{stream: s}
	return ss
}

type serverStream struct {
	*stream
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
