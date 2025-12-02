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
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/util/gopool"

	internal_stream "github.com/cloudwego/kitex/internal/stream"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/endpoint/cep"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/remotecli"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
	streaming_types "github.com/cloudwego/kitex/pkg/streaming/types"
	"github.com/cloudwego/kitex/transport"
)

// Streaming client streaming interface for code generate
type Streaming interface {
	// Deprecated, keep this method for compatibility with gen code with version < v0.13.0,
	// regenerate code with kitex command with version >= v0.13.0 to use StreamX instead.
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
	ctx, ri, _ = kc.initRPCInfo(ctx, method, 0, nil, true)

	ctx = kc.opt.TracerCtl.DoStart(ctx, ri)
	var reportErr error
	var err error
	var cs streaming.ClientStream
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = rpcinfo.ClientPanicToErr(ctx, panicInfo, ri, false)
			reportErr = err
		}
		if reportErr != nil {
			kc.opt.TracerCtl.DoFinish(ctx, ri, reportErr)
		}
	}()

	mi := ri.Invocation().MethodInfo()
	if mi == nil {
		err = kerrors.ErrNonExistentMethod(kc.svcInfo.ServiceName, method)
		reportErr = err
		return err
	}
	sm := mi.StreamingMode()
	if sm == serviceinfo.StreamingNone || sm == serviceinfo.StreamingUnary {
		err = kerrors.ErrNotStreamingMethod(kc.svcInfo.ServiceName, method)
		reportErr = err
		return err
	}

	cs, err = kc.sEps(ctx)
	if err != nil {
		reportErr = err
		return err
	}
	result := response.(*streaming.Result)
	result.ClientStream = cs
	if getter, ok := cs.(streaming.GRPCStreamGetter); ok {
		grpcStream := getter.GetGRPCStream()
		if grpcStream == nil {
			err = fmt.Errorf("ClientStream.GetGRPCStream() returns nil: %T", cs)
			reportErr = err
			return err
		}
		result.Stream = grpcStream
		return nil
	} else {
		err = fmt.Errorf("ClientStream does not implement streaming.GRPCStreamGetter interface: %T", cs)
		reportErr = err
		return err
	}
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
	ctx, ri, _ = kc.initRPCInfo(ctx, method, 0, nil, true)

	ctx = kc.opt.TracerCtl.DoStart(ctx, ri)
	var reportErr error
	var err error
	var cs streaming.ClientStream
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = rpcinfo.ClientPanicToErr(ctx, panicInfo, ri, false)
			reportErr = err
		}
		if reportErr != nil {
			kc.opt.TracerCtl.DoFinish(ctx, ri, reportErr)
		}
	}()

	mi := ri.Invocation().MethodInfo()
	if mi == nil {
		err = kerrors.ErrNonExistentMethod(kc.svcInfo.ServiceName, method)
		reportErr = err
		return nil, err
	}
	sm := mi.StreamingMode()
	if sm == serviceinfo.StreamingNone || sm == serviceinfo.StreamingUnary {
		err = kerrors.ErrNotStreamingMethod(kc.svcInfo.ServiceName, method)
		reportErr = err
		return nil, err
	}

	cs, err = kc.sEps(ctx)
	if err != nil {
		reportErr = err
	}
	return cs, err
}

func (kc *kClient) invokeStreamingEndpoint() (endpoint.Endpoint, error) {
	var handler remote.ClientTransHandler
	var err error
	if kc.opt.RemoteOpt.GRPCStreamingCliHandlerFactory != nil {
		handler, err = kc.opt.RemoteOpt.GRPCStreamingCliHandlerFactory.NewTransHandler(kc.opt.RemoteOpt)
	} else {
		handler, err = kc.opt.RemoteOpt.CliHandlerFactory.NewTransHandler(kc.opt.RemoteOpt)
	}
	if err != nil {
		return nil, err
	}

	// recvEP and sendEP are the endpoints for the new stream interface,
	// and grpcRecvEP and grpcSendEP are the endpoints for the old grpc stream interface,
	// the latter two are going to be removed in the future.
	recvEP := kc.opt.StreamOptions.BuildRecvChain(recvEndpoint)
	sendEP := kc.opt.StreamOptions.BuildSendChain(sendEndpoint)
	grpcRecvEP := kc.opt.Streaming.BuildRecvInvokeChain()
	grpcSendEP := kc.opt.Streaming.BuildSendInvokeChain()

	return func(ctx context.Context, req, resp interface{}) (err error) {
		// req and resp as &streaming.Stream
		ri := rpcinfo.GetRPCInfo(ctx)
		var cancel context.CancelFunc
		// apply stream timeout
		if tm := ri.Config().StreamTimeout(); tm > 0 {
			ctx, cancel = context.WithTimeout(ctx, tm)
		}
		st, scm, err := remotecli.NewStream(ctx, ri, handler, kc.opt.RemoteOpt)
		if err != nil {
			if cancel != nil {
				cancel()
			}
			return
		}

		clientStream := newStream(ctx, cancel,
			st, scm, kc, ri, ri.Invocation().MethodInfo().StreamingMode(),
			sendEP, recvEP, kc.opt.StreamOptions.EventHandler, grpcSendEP, grpcRecvEP)
		rresp := resp.(*streaming.Result)
		rresp.ClientStream = clientStream
		rresp.Stream = clientStream.GetGRPCStream()
		return
	}, nil
}

type stream struct {
	streaming.ClientStream
	grpcStream   *grpcStream
	ctx          context.Context
	scm          *remotecli.StreamConnManager
	kc           *kClient
	ri           rpcinfo.RPCInfo
	eventHandler internal_stream.StreamEventHandler

	recv   cep.StreamRecvEndpoint
	recvTm time.Duration
	send   cep.StreamSendEndpoint
	sendTm time.Duration

	streamingMode serviceinfo.StreamingMode
	finished      uint32
	isGRPC        bool
	cancelFunc    context.CancelFunc
}

var (
	_ streaming.GRPCStreamGetter = (*stream)(nil)
	_ streaming.WithDoFinish     = (*stream)(nil)
	_ streaming.WithDoFinish     = (*grpcStream)(nil)
)

func newStream(ctx context.Context, cancel context.CancelFunc, s streaming.ClientStream, scm *remotecli.StreamConnManager, kc *kClient, ri rpcinfo.RPCInfo, mode serviceinfo.StreamingMode,
	sendEP cep.StreamSendEndpoint, recvEP cep.StreamRecvEndpoint, eventHandler internal_stream.StreamEventHandler, grpcSendEP endpoint.SendEndpoint, grpcRecvEP endpoint.RecvEndpoint,
) *stream {
	recvTm := ri.Config().StreamRecvTimeout()
	sendTm := ri.Config().StreamSendTimeout()
	isGRPC := ri.Config().TransportProtocol()&transport.GRPC != 0
	st := &stream{
		ClientStream:  s,
		ctx:           ctx,
		scm:           scm,
		kc:            kc,
		ri:            ri,
		streamingMode: mode,
		recv:          recvEP,
		recvTm:        recvTm,
		send:          sendEP,
		sendTm:        sendTm,
		eventHandler:  eventHandler,
		isGRPC:        isGRPC,
		cancelFunc:    cancel,
	}
	if grpcStreamGetter, ok := s.(streaming.GRPCStreamGetter); ok {
		if grpcStream := grpcStreamGetter.GetGRPCStream(); grpcStream != nil {
			st.grpcStream = newGRPCStream(grpcStream, grpcSendEP, sendTm, grpcRecvEP, recvTm)
			st.grpcStream.st = st
		}
	}
	if register, ok := s.(streaming.CloseCallbackRegister); ok {
		register.RegisterCloseCallback(st.DoFinish)
	}
	return st
}

// Header returns the header data sent by the server if any.
func (s *stream) Header() (hd streaming.Header, err error) {
	if hd, err = s.ClientStream.Header(); err != nil {
		s.DoFinish(err)
	}
	return
}

// RecvMsg receives a message from the server.
// If an error is returned, stream.DoFinish() will be called to record the end of stream
func (s *stream) RecvMsg(ctx context.Context, m interface{}) (err error) {
	if !s.recv.EqualsTo(recvEndpoint) {
		// If the values are not equal, it indicates the presence of custom middleware.
		// To prevent errors caused by middleware code that relies on rpcinfo when users
		// incorrectly pass a context lacking rpcinfo, this safeguard logic is added to
		// propagate the rpcinfo from the stream to the incoming context.
		ri := rpcinfo.GetRPCInfo(ctx)
		if ri != s.ri {
			ctx = rpcinfo.NewCtxWithRPCInfo(ctx, s.ri)
		}
	}
	err = s.recvWithTimeout(ctx, m)
	if err == nil {
		// BizStatusErr is returned by the server handle, meaning the stream is ended;
		// And it should be returned to the calling business code for error handling
		err = s.ri.Invocation().BizStatusErr()
	}
	if s.eventHandler != nil {
		s.eventHandler(s.ctx, stats.StreamRecv, err)
	}
	if err != nil || s.streamingMode == serviceinfo.StreamingClient {
		s.DoFinish(err)
	}
	return
}

func (s *stream) recvWithTimeout(ctx context.Context, m interface{}) error {
	return callWithTimeout(s.recvTm,
		func() error {
			return s.recv(ctx, s.ClientStream, m)
		},
		func(tm time.Duration) error {
			if s.isGRPC {
				return status.NewTimeoutStatus(codes.DeadlineExceeded, fmt.Sprintf(recvTimeoutErrTpl, tm), streaming_types.StreamRecvTimeout).Err()
			}
			return ttstream.NewTimeoutException(streaming_types.StreamRecvTimeout, remote.Client, tm)
		},
		func(err error) {
			s.Cancel(err)
		})
}

// SendMsg sends a message to the server.
// If an error is returned, stream.DoFinish() will be called to record the end of stream
func (s *stream) SendMsg(ctx context.Context, m interface{}) (err error) {
	if !s.send.EqualsTo(sendEndpoint) {
		// same with RecvMsg
		ri := rpcinfo.GetRPCInfo(ctx)
		if ri != s.ri {
			ctx = rpcinfo.NewCtxWithRPCInfo(ctx, s.ri)
		}
	}
	err = s.sendWithTimeout(ctx, m)
	if s.eventHandler != nil {
		s.eventHandler(s.ctx, stats.StreamSend, err)
	}
	if err != nil {
		s.DoFinish(err)
	}
	return
}

func (s *stream) sendWithTimeout(ctx context.Context, m interface{}) error {
	return callWithTimeout(s.sendTm,
		func() error {
			return s.send(ctx, s.ClientStream, m)
		},
		func(tm time.Duration) error {
			if s.isGRPC {
				return status.NewTimeoutStatus(codes.DeadlineExceeded, fmt.Sprintf(sendTimeoutErrTpl, tm), streaming_types.StreamSendTimeout).Err()
			}
			return ttstream.NewTimeoutException(streaming_types.StreamSendTimeout, remote.Client, tm)
		},
		func(err error) {
			s.Cancel(err)
		})
}

// DoFinish implements the streaming.WithDoFinish interface, and it records the end of stream
// It will release the connection.
func (s *stream) DoFinish(err error) {
	if atomic.SwapUint32(&s.finished, 1) == 1 {
		// already called
		return
	}
	// release stream timeout cancel
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	if !isRPCError(err) {
		// only rpc errors are reported
		err = nil
	}
	if s.scm != nil {
		s.scm.ReleaseConn(err, s.ri)
	}
	s.kc.opt.TracerCtl.DoFinish(s.ctx, s.ri, err)
}

func (s *stream) GetGRPCStream() streaming.Stream {
	if s.grpcStream == nil {
		return nil
	}
	return s.grpcStream
}

const (
	recvTimeoutErrTpl = "stream Recv timeout, timeout config=%v"
	sendTimeoutErrTpl = "stream Send timeout, timeout config=%v"
)

func newGRPCStream(st streaming.Stream, sendEP endpoint.SendEndpoint, sendTm time.Duration, recvEP endpoint.RecvEndpoint, recvTm time.Duration) *grpcStream {
	return &grpcStream{
		Stream:       st,
		sendEndpoint: sendEP,
		sendTm:       sendTm,
		recvEndpoint: recvEP,
		recvTm:       recvTm,
	}
}

type grpcStream struct {
	streaming.Stream

	st *stream

	sendEndpoint endpoint.SendEndpoint
	sendTm       time.Duration
	recvEndpoint endpoint.RecvEndpoint
	recvTm       time.Duration
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
	err = s.recvWithTimeout(m)
	if err == nil {
		// BizStatusErr is returned by the server handle, meaning the stream is ended;
		// And it should be returned to the calling business code for error handling
		err = s.st.ri.Invocation().BizStatusErr()
	}
	if s.st.eventHandler != nil {
		s.st.eventHandler(s.st.ctx, stats.StreamRecv, err)
	}
	if err != nil || s.st.streamingMode == serviceinfo.StreamingClient {
		s.st.DoFinish(err)
	}
	return
}

func (s *grpcStream) recvWithTimeout(m interface{}) error {
	return callWithTimeout(s.recvTm,
		func() error {
			return s.recvEndpoint(s.Stream, m)
		},
		func(tm time.Duration) error {
			return status.NewTimeoutStatus(codes.DeadlineExceeded, fmt.Sprintf(recvTimeoutErrTpl, tm), streaming_types.StreamRecvTimeout).Err()
		},
		func(err error) {
			s.st.Cancel(err)
		},
	)
}

func (s *grpcStream) SendMsg(m interface{}) (err error) {
	err = s.sendWithTimeout(m)
	if s.st.eventHandler != nil {
		s.st.eventHandler(s.st.ctx, stats.StreamSend, err)
	}
	if err != nil {
		s.st.DoFinish(err)
	}
	return
}

func (s *grpcStream) sendWithTimeout(m interface{}) error {
	return callWithTimeout(s.sendTm,
		func() error {
			return s.sendEndpoint(s.Stream, m)
		},
		func(tm time.Duration) error {
			return status.NewTimeoutStatus(codes.DeadlineExceeded, fmt.Sprintf(sendTimeoutErrTpl, tm), streaming_types.StreamSendTimeout).Err()
		},
		func(err error) {
			s.st.Cancel(err)
		},
	)
}

func (s *grpcStream) DoFinish(err error) {
	s.st.DoFinish(err)
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

func callWithTimeout(tm time.Duration, call func() error, buildTmErr func(time.Duration) error, cancel func(error)) error {
	if tm <= 0 {
		return call()
	}

	timer := time.NewTimer(tm)
	defer timer.Stop()
	finishChan := make(chan error, 1)
	gopool.Go(func() {
		callErr := call()
		finishChan <- callErr
	})
	select {
	case <-timer.C:
		err := buildTmErr(tm)
		cancel(err)
		return err
	case callErr := <-finishChan:
		return callErr
	}
}

var (
	recvEndpoint cep.StreamRecvEndpoint = func(ctx context.Context, stream streaming.ClientStream, m interface{}) error {
		return stream.RecvMsg(ctx, m)
	}
	sendEndpoint cep.StreamSendEndpoint = func(ctx context.Context, stream streaming.ClientStream, m interface{}) error {
		return stream.SendMsg(ctx, m)
	}
)
