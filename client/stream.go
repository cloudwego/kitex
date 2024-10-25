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

package client

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/serviceinfo"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/remote/remotecli"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// Streaming client streaming interface for code generate
type Streaming interface {
	Stream(ctx context.Context, method string, request, response interface{}) error
	StreamX(ctx context.Context, method string) (streaming.ClientStream, error)
}

// Stream implements the Streaming interface
func (kc *kClient) Stream(ctx context.Context, method string, request, response interface{}) error {
	if !kc.inited {
		panic("client not initialized")
	}
	if kc.closed {
		panic("client is already closed")
	}
	if ctx == nil {
		panic("ctx is nil")
	}
	var ri rpcinfo.RPCInfo
	ctx, ri, _ = kc.initRPCInfo(ctx, method, 0, nil)

	rpcinfo.AsMutableRPCConfig(ri.Config()).SetInteractionMode(rpcinfo.Streaming)
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)

	ctx = kc.opt.TracerCtl.DoStart(ctx, ri)
	cs, err := kc.sEps(ctx)
	if err != nil {
		return err
	}
	result := response.(*streaming.Result)
	result.ClientStream = cs
	if getter, ok := cs.(streaming.GRPCStreamGetter); ok {
		result.Stream = getter.GetGRPCStream()
	}
	return nil
}

// StreamX implements the Streaming interface
func (kc *kClient) StreamX(ctx context.Context, method string) (streaming.ClientStream, error) {
	if !kc.inited {
		panic("client not initialized")
	}
	if kc.closed {
		panic("client is already closed")
	}
	if ctx == nil {
		panic("ctx is nil")
	}
	var ri rpcinfo.RPCInfo
	ctx, ri, _ = kc.initRPCInfo(ctx, method, 0, nil)

	rpcinfo.AsMutableRPCConfig(ri.Config()).SetInteractionMode(rpcinfo.Streaming)
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)

	ctx = kc.opt.TracerCtl.DoStart(ctx, ri)
	return kc.sEps(ctx)
}

func (kc *kClient) connectStreamEndpoint() (InnerEndpoint, error) {
	transPipl, err := newCliTransHandler(kc.opt.RemoteOpt)
	if err != nil {
		return nil, err
	}

	recvEndpoint := kc.opt.Streaming.BuildRecvInvokeChain(invokeRecvEndpoint())
	sendEndpoint := kc.opt.Streaming.BuildSendInvokeChain(invokeSendEndpoint())

	return func(ctx context.Context) (st streaming.ClientStream, err error) {
		ri := rpcinfo.GetRPCInfo(ctx)
		var cr remotecli.ConnReleaser
		st, cr, err = remotecli.NewStream(ctx, ri, transPipl, kc.opt.RemoteOpt)
		if err != nil {
			return
		}
		stMode := ri.Invocation().MethodInfo().StreamingMode()
		if stMode == serviceinfo.StreamingNone || stMode == serviceinfo.StreamingUnary {
			st = newUnary(st, cr, ri)
		} else {
			st = newStream(ctx, st, cr, kc, ri, kc.getStreamingMode(ri), sendEndpoint, recvEndpoint)
		}
		return
	}, nil
}

func (kc *kClient) getStreamingMode(ri rpcinfo.RPCInfo) serviceinfo.StreamingMode {
	methodInfo := kc.svcInfo.MethodInfo(ri.Invocation().MethodName())
	if methodInfo == nil {
		return serviceinfo.StreamingNone
	}
	return methodInfo.StreamingMode()
}

type unary struct {
	streaming.ClientStream

	ri rpcinfo.RPCInfo
	cr remotecli.ConnReleaser
}

func newUnary(st streaming.ClientStream, cr remotecli.ConnReleaser, ri rpcinfo.RPCInfo) streaming.ClientStream {
	return &unary{
		ClientStream: st,
		cr:           cr,
		ri:           ri,
	}
}

func (u *unary) SendMsg(m interface{}) (err error) {
	err = u.ClientStream.SendMsg(m)
	if err != nil {
		u.cr.ReleaseConn(err, u.ri)
	}
	return err
}

func (u *unary) RecvMsg(m interface{}) (err error) {
	if u.ri.Invocation().MethodInfo().OneWay() {
		// Wait for the request to be flushed out before closing the connection.
		// 500us is an acceptable duration to keep a minimal loss rate at best effort.
		time.Sleep(time.Millisecond / 2)
	} else {
		err = u.ClientStream.RecvMsg(m)
	}
	u.cr.ReleaseConn(err, u.ri)
	return err
}

type stream struct {
	streaming.ClientStream
	grpcStream grpcStream
	ctx        context.Context
	sr         remotecli.ConnReleaser
	kc         *kClient
	ri         rpcinfo.RPCInfo

	streamingMode serviceinfo.StreamingMode
	finished      uint32
}

var _ streaming.WithDoFinish = (*stream)(nil)

func newStream(ctx context.Context, s streaming.ClientStream, sr remotecli.ConnReleaser, kc *kClient, ri rpcinfo.RPCInfo,
	mode serviceinfo.StreamingMode, sendEP endpoint.SendEndpoint, recvEP endpoint.RecvEndpoint,
) *stream {
	st := &stream{
		ClientStream:  s,
		ctx:           ctx,
		sr:            sr,
		kc:            kc,
		ri:            ri,
		streamingMode: mode,
	}
	if grpcStreamGetter, ok := s.(streaming.GRPCStreamGetter); ok {
		grpcStream := grpcStreamGetter.GetGRPCStream()
		st.grpcStream = newGRPCStream(grpcStream, sendEP, recvEP)
		st.grpcStream.st = st
	}
	return st
}

// RecvMsg receives a message from the server.
// If an error is returned, stream.DoFinish() will be called to record the end of stream
func (s *stream) RecvMsg(m interface{}) (err error) {
	err = s.ClientStream.RecvMsg(m)
	if err == nil {
		// BizStatusErr is returned by the server handle, meaning the stream is ended;
		// And it should be returned to the calling business code for error handling
		err = s.ri.Invocation().BizStatusErr()
	}
	if err != nil || s.streamingMode == serviceinfo.StreamingClient {
		s.DoFinish(err)
	}
	return
}

// SendMsg sends a message to the server.
// If an error is returned, stream.DoFinish() will be called to record the end of stream
func (s *stream) SendMsg(m interface{}) (err error) {
	if err = s.ClientStream.SendMsg(m); err != nil {
		s.DoFinish(err)
	}
	return
}

// DoFinish implements the streaming.WithDoFinish interface, and it records the end of stream
// It will release the connection.
func (s *stream) DoFinish(err error) {
	if atomic.SwapUint32(&s.finished, 1) == 1 {
		// already called
		return
	}
	if !isRPCError(err) {
		// only rpc errors are reported
		err = nil
	}
	if s.sr != nil {
		s.sr.ReleaseConn(err, s.ri)
	}
	s.kc.opt.TracerCtl.DoFinish(s.ctx, s.ri, err)
}

func (s *stream) GetGRPCStream() streaming.Stream {
	return &s.grpcStream
}

func newGRPCStream(st streaming.Stream, sendEP endpoint.SendEndpoint, recvEP endpoint.RecvEndpoint) grpcStream {
	return grpcStream{
		Stream:       st,
		sendEndpoint: sendEP,
		recvEndpoint: recvEP,
	}
}

type grpcStream struct {
	streaming.Stream

	st *stream

	sendEndpoint endpoint.SendEndpoint
	recvEndpoint endpoint.RecvEndpoint
}

// Header returns the header metadata sent by the server if any.
// If a non-nil error is returned, stream.DoFinish() will be called to record the EndOfStream
func (s *grpcStream) Header() (md metadata.MD, err error) {
	if md, err = s.Stream.Header(); err != nil {
		s.st.DoFinish(err)
	}
	return
}

func (s *grpcStream) RecvMsg(m interface{}) (err error) {
	err = s.recvEndpoint(s.Stream, m)
	if err == nil {
		// BizStatusErr is returned by the server handle, meaning the stream is ended;
		// And it should be returned to the calling business code for error handling
		err = s.st.ri.Invocation().BizStatusErr()
	}
	if err != nil || s.st.streamingMode == serviceinfo.StreamingClient {
		s.st.DoFinish(err)
	}
	return
}

func (s *grpcStream) SendMsg(m interface{}) (err error) {
	if err = s.sendEndpoint(s.Stream, m); err != nil {
		s.st.DoFinish(err)
	}
	return
}

func (s *grpcStream) DoFinish(err error) {
	s.st.DoFinish(err)
}

func invokeRecvEndpoint() endpoint.RecvEndpoint {
	return func(stream streaming.Stream, resp interface{}) (err error) {
		return stream.RecvMsg(resp)
	}
}

func invokeSendEndpoint() endpoint.SendEndpoint {
	return func(stream streaming.Stream, req interface{}) (err error) {
		return stream.SendMsg(req)
	}
}

func isRPCError(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return false
	}
	_, isBizStatusError := err.(kerrors.BizStatusErrorIface)
	// if a tracer needs to get the BizStatusError, it should read from rpcinfo.invocation.bizStatusErr
	return !isBizStatusError
}
