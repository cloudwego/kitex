/*
 * Copyright 2026 CloudWeGo Authors
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

// Package localstream provides in-process stream pairs used by LocalCaller.
package localstream

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

const defaultBufferSize = 16

type message struct {
	v any
}

type pipe struct {
	ch chan message

	mu     sync.Mutex
	closed bool
	err    error
}

func newPipe(size int) *pipe {
	if size <= 0 {
		size = defaultBufferSize
	}
	return &pipe{ch: make(chan message, size)}
}

func (p *pipe) write(ctx context.Context, v any) error {
	p.mu.Lock()
	closed := p.closed
	err := p.err
	p.mu.Unlock()
	if closed {
		if err != nil {
			return err
		}
		return io.ErrClosedPipe
	}
	select {
	case p.ch <- message{v: v}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *pipe) read(ctx context.Context) (any, error) {
	select {
	case msg, ok := <-p.ch:
		if ok {
			return msg.v, nil
		}
		p.mu.Lock()
		err := p.err
		p.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *pipe) close(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	p.err = err
	close(p.ch)
}

// NewPair creates a connected in-process client/server stream pair.
func NewPair(ctx context.Context, ri rpcinfo.RPCInfo) (*Endpoint, *Endpoint) {
	c2s := newPipe(defaultBufferSize)
	s2c := newPipe(defaultBufferSize)
	shared := &sharedState{ri: ri}
	client := &Endpoint{ctx: ctx, send: c2s, recv: s2c, shared: shared, clientSide: true}
	server := &Endpoint{ctx: ctx, send: s2c, recv: c2s, shared: shared}
	return client, server
}

type sharedState struct {
	ri rpcinfo.RPCInfo

	mu         sync.Mutex
	header     streaming.Header
	headerSent bool
	headerDone chan struct{}
	trailer    streaming.Trailer
}

func (s *sharedState) ensureHeaderDone() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.headerDone == nil {
		s.headerDone = make(chan struct{})
	}
	return s.headerDone
}

func (s *sharedState) sendHeader(h streaming.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.headerDone == nil {
		s.headerDone = make(chan struct{})
	}
	if s.headerSent {
		return
	}
	s.header = mergeHeader(s.header, h)
	s.headerSent = true
	close(s.headerDone)
}

func (s *sharedState) setHeader(h streaming.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.header = mergeHeader(s.header, h)
}

func (s *sharedState) setTrailer(t streaming.Trailer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trailer == nil {
		s.trailer = streaming.Trailer{}
	}
	for k, v := range t {
		s.trailer[k] = v
	}
}

func (s *sharedState) getTrailer() streaming.Trailer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := streaming.Trailer{}
	for k, v := range s.trailer {
		out[k] = v
	}
	return out
}

func (s *sharedState) getHeader() streaming.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := streaming.Header{}
	for k, v := range s.header {
		out[k] = v
	}
	return out
}

func mergeHeader(dst, src streaming.Header) streaming.Header {
	if dst == nil {
		dst = streaming.Header{}
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Endpoint implements streaming.ClientStream, streaming.ServerStream and the
// deprecated gRPC-compatible streaming.Stream interface.
type Endpoint struct {
	ctx        context.Context
	send       *pipe
	recv       *pipe
	shared     *sharedState
	clientSide bool

	finishOnce sync.Once
	callbacks  []func(error)
	cbMu       sync.Mutex
	grpc       *grpcAdapter
}

var (
	_ streaming.ClientStream          = (*Endpoint)(nil)
	_ streaming.ServerStream          = (*Endpoint)(nil)
	_ streaming.GRPCStreamGetter      = (*Endpoint)(nil)
	_ streaming.WithDoFinish          = (*Endpoint)(nil)
	_ streaming.CloseCallbackRegister = (*Endpoint)(nil)
)

func (s *Endpoint) SendMsg(ctx context.Context, m any) error {
	if !s.clientSide {
		s.shared.sendHeader(nil)
	}
	return s.send.write(ctx, m)
}

func (s *Endpoint) RecvMsg(ctx context.Context, m any) error {
	v, err := s.recv.read(ctx)
	if err != nil {
		// Mirror the nphttp2 client transport behavior: when the client observes
		// end of stream and the server reported a BizStatusError, surface the biz
		// error instead of io.EOF. This replacement is limited to the client side:
		// the server side must keep observing raw io.EOF.
		if err == io.EOF && s.clientSide && s.shared != nil && s.shared.ri != nil {
			if bizErr := s.shared.ri.Invocation().BizStatusErr(); bizErr != nil {
				return bizErr
			}
		}
		return err
	}
	return assignMessage(m, v)
}

func (s *Endpoint) Header() (streaming.Header, error) {
	done := s.shared.ensureHeaderDone()
	select {
	case <-done:
		return s.shared.getHeader(), nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *Endpoint) Trailer() (streaming.Trailer, error) {
	return s.shared.getTrailer(), nil
}

func (s *Endpoint) CloseSend(ctx context.Context) error {
	s.send.close(nil)
	return nil
}

func (s *Endpoint) Context() context.Context {
	return s.ctx
}

func (s *Endpoint) SetHeader(h streaming.Header) error {
	s.shared.setHeader(h)
	return nil
}

func (s *Endpoint) SendHeader(h streaming.Header) error {
	s.shared.sendHeader(h)
	return nil
}

func (s *Endpoint) SetTrailer(t streaming.Trailer) error {
	s.shared.setTrailer(t)
	return nil
}

func (s *Endpoint) GetGRPCStream() streaming.Stream {
	if s.grpc == nil {
		s.grpc = &grpcAdapter{st: s}
	}
	return s.grpc
}

func (s *Endpoint) RegisterCloseCallback(cb func(error)) {
	s.cbMu.Lock()
	defer s.cbMu.Unlock()
	s.callbacks = append(s.callbacks, cb)
}

func (s *Endpoint) DoFinish(err error) {
	s.finishOnce.Do(func() {
		s.cbMu.Lock()
		callbacks := append([]func(error){}, s.callbacks...)
		s.cbMu.Unlock()
		for _, cb := range callbacks {
			cb(err)
		}
	})
}

// FinishServer closes the server-to-client direction after the server handler returns.
func (s *Endpoint) FinishServer(err error) {
	s.shared.sendHeader(nil)
	s.send.close(err)
}

type grpcAdapter struct {
	st *Endpoint
}

var _ streaming.Stream = (*grpcAdapter)(nil)
var _ streaming.WithDoFinish = (*grpcAdapter)(nil)

func (g *grpcAdapter) SetHeader(md metadata.MD) error {
	return g.st.SetHeader(mdToHeader(md))
}

func (g *grpcAdapter) SendHeader(md metadata.MD) error {
	return g.st.SendHeader(mdToHeader(md))
}

func (g *grpcAdapter) SetTrailer(md metadata.MD) {
	_ = g.st.SetTrailer(mdToTrailer(md))
}

func (g *grpcAdapter) Header() (metadata.MD, error) {
	h, err := g.st.Header()
	if err != nil {
		return nil, err
	}
	return headerToMD(h), nil
}

func (g *grpcAdapter) Trailer() metadata.MD {
	t, _ := g.st.Trailer()
	return trailerToMD(t)
}

func (g *grpcAdapter) Context() context.Context {
	return g.st.Context()
}

func (g *grpcAdapter) RecvMsg(m interface{}) error {
	return g.st.RecvMsg(g.st.ctx, m)
}

func (g *grpcAdapter) SendMsg(m interface{}) error {
	return g.st.SendMsg(g.st.ctx, m)
}

func (g *grpcAdapter) Close() error {
	return g.st.CloseSend(g.st.ctx)
}

func (g *grpcAdapter) DoFinish(err error) {
	g.st.DoFinish(err)
}

func assignMessage(dst, src any) error {
	if dst == nil {
		return fmt.Errorf("localstream: nil receive target")
	}
	dv := reflect.ValueOf(dst)
	if dv.Kind() != reflect.Pointer || dv.IsNil() {
		return fmt.Errorf("localstream: receive target must be non-nil pointer, got %T", dst)
	}
	de := dv.Elem()
	sv := reflect.ValueOf(src)
	if !sv.IsValid() {
		de.Set(reflect.Zero(de.Type()))
		return nil
	}
	if sv.Type().AssignableTo(de.Type()) {
		de.Set(sv)
		return nil
	}
	if sv.Kind() == reflect.Pointer && !sv.IsNil() && sv.Elem().Type().AssignableTo(de.Type()) {
		de.Set(sv.Elem())
		return nil
	}
	if de.Kind() == reflect.Pointer && sv.Type().AssignableTo(de.Type()) {
		de.Set(sv)
		return nil
	}
	return fmt.Errorf("localstream: cannot assign sent message %T to receive target %T", src, dst)
}

func mdToHeader(md metadata.MD) streaming.Header {
	out := streaming.Header{}
	for k, vals := range md {
		if len(vals) > 0 {
			out[k] = vals[len(vals)-1]
		}
	}
	return out
}

func mdToTrailer(md metadata.MD) streaming.Trailer {
	out := streaming.Trailer{}
	for k, vals := range md {
		if len(vals) > 0 {
			out[k] = vals[len(vals)-1]
		}
	}
	return out
}

func headerToMD(h streaming.Header) metadata.MD {
	out := metadata.MD{}
	for k, v := range h {
		out[k] = []string{v}
	}
	return out
}

func trailerToMD(t streaming.Trailer) metadata.MD {
	out := metadata.MD{}
	for k, v := range t {
		out[k] = []string{v}
	}
	return out
}
