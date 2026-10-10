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

// Package clientstream provides the shared client-side streaming wrapper used by
// both the network client (client package) and the in-process LocalCaller
// (server package). It centralizes stream send/recv event tracing, DoFinish
// idempotency, BizStatusError surfacing and the client-streaming end-of-stream
// semantics so the two callers do not maintain divergent copies.
//
// It must not import the client/server/remotecli packages to avoid import
// cycles; all transport-specific behavior (connection release, recv timeout,
// trace finish ownership) is injected through Options.
package clientstream

import (
	"context"
	"io"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/util/gopool"

	internal_stream "github.com/cloudwego/kitex/internal/stream"
	"github.com/cloudwego/kitex/pkg/endpoint/cep"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

const recvTimeoutErrTpl = "stream Recv timeout, timeout config=%+v"

// DefaultSendEndpoint / DefaultRecvEndpoint directly delegate to the underlying
// ClientStream. They are the sentinel endpoints used to detect whether a custom
// send/recv middleware chain has been installed (see Options.Send/Recv).
var (
	DefaultSendEndpoint cep.StreamSendEndpoint = func(ctx context.Context, s streaming.ClientStream, m interface{}) error {
		return s.SendMsg(ctx, m)
	}
	DefaultRecvEndpoint cep.StreamRecvEndpoint = func(ctx context.Context, s streaming.ClientStream, m interface{}) error {
		return s.RecvMsg(ctx, m)
	}
)

// FinishHandler is invoked exactly once when a CommonStream finishes, after the
// err has been filtered by isRPCError.
type FinishHandler interface {
	OnFinish(err error, ri rpcinfo.RPCInfo)
}

// Options carries the optional, caller-specific behavior injected into a CommonStream.
// The network client fills all fields; the in-process LocalCaller leaves them at zero.
type Options struct {
	// Send / Recv are the (already-built) send/recv middleware chains. When nil,
	// the stream directly delegates to the underlying ClientStream.
	Send cep.StreamSendEndpoint
	Recv cep.StreamRecvEndpoint

	// RecvTmCfg / EnableRecvTimeout enable the recv timeout path, which is only
	// meaningful for gRPC streams whose transport implements CancelableClientStream.
	RecvTmCfg         streaming.TimeoutConfig
	EnableRecvTimeout bool

	// OnFinish is invoked exactly once when the stream finishes, after the err has
	// been filtered by isRPCError. The network client uses it to release the
	// connection and call TracerCtl.DoFinish. The LocalCaller leaves it nil because
	// its trace finish is owned by the server handler goroutine.
	OnFinish func(err error, ri rpcinfo.RPCInfo)

	// FinishHandler is the allocation-friendly equivalent of OnFinish. Prefer this
	// on hot paths so callers can pass an existing wrapper object instead of
	// allocating a closure.
	FinishHandler FinishHandler
}

// CommonStream is the shared core wrapping a transport ClientStream.
type CommonStream struct {
	streaming.ClientStream

	ctx           context.Context
	traceCtl      *rpcinfo.TraceController
	ri            rpcinfo.RPCInfo
	streamingMode serviceinfo.StreamingMode

	send cep.StreamSendEndpoint
	recv cep.StreamRecvEndpoint
	opts Options

	// sendFixCtx / recvFixCtx mark whether a custom (non-default) middleware chain
	// is installed. Only then do we defensively re-inject the stream's RPCInfo into
	// the incoming context, matching the historical network-client behavior.
	sendFixCtx bool
	recvFixCtx bool

	onFinish      func(err error, ri rpcinfo.RPCInfo)
	finishHandler FinishHandler
	finished      uint32

	// grpcStream is the gRPC-compatible view returned by GetGRPCStream. It is set
	// by callers that rely on CommonStream.GetGRPCStream directly, such as
	// LocalCaller with GRPCForwardStream. The network client keeps its
	// middleware-aware gRPC view on its outer stream wrapper.
	grpcStream streaming.Stream
}

var _ streaming.GRPCStreamGetter = (*CommonStream)(nil)

// SetGRPCStream sets the gRPC-compatible stream view returned by GetGRPCStream.
func (s *CommonStream) SetGRPCStream(gs streaming.Stream) { s.grpcStream = gs }

// GetGRPCStream returns the gRPC-compatible stream view, or nil if none.
func (s *CommonStream) GetGRPCStream() streaming.Stream {
	if s.grpcStream == nil {
		return nil
	}
	return s.grpcStream
}

// New creates a CommonStream. traceCtl must not be nil.
func New(ctx context.Context, cs streaming.ClientStream, traceCtl *rpcinfo.TraceController,
	ri rpcinfo.RPCInfo, mode serviceinfo.StreamingMode, opts Options,
) *CommonStream {
	s := new(CommonStream)
	Init(s, ctx, cs, traceCtl, ri, mode, opts)
	return s
}

// Init initializes s as a CommonStream. It exists so performance-sensitive
// wrappers (notably the network client stream) can embed CommonStream by value
// and avoid one extra allocation per stream creation.
func Init(s *CommonStream, ctx context.Context, cs streaming.ClientStream, traceCtl *rpcinfo.TraceController,
	ri rpcinfo.RPCInfo, mode serviceinfo.StreamingMode, opts Options,
) {
	send, recv := opts.Send, opts.Recv
	if send == nil {
		send = DefaultSendEndpoint
	}
	if recv == nil {
		recv = DefaultRecvEndpoint
	}
	*s = CommonStream{
		ClientStream:  cs,
		ctx:           ctx,
		traceCtl:      traceCtl,
		ri:            ri,
		streamingMode: mode,
		send:          send,
		recv:          recv,
		opts:          opts,
		onFinish:      opts.OnFinish,
		finishHandler: opts.FinishHandler,
		sendFixCtx:    opts.Send != nil && !opts.Send.EqualsTo(DefaultSendEndpoint),
		recvFixCtx:    opts.Recv != nil && !opts.Recv.EqualsTo(DefaultRecvEndpoint),
	}
}

// Ctx returns the stream context captured at creation time.
func (s *CommonStream) Ctx() context.Context { return s.ctx }

// RI returns the stream RPCInfo.
func (s *CommonStream) RI() rpcinfo.RPCInfo { return s.ri }

// StreamingMode returns the streaming mode.
func (s *CommonStream) StreamingMode() serviceinfo.StreamingMode { return s.streamingMode }

// Header returns the header data sent by the server if any.
// If an error is returned, DoFinish is called to record the end of stream.
func (s *CommonStream) Header() (hd streaming.Header, err error) {
	if hd, err = s.ClientStream.Header(); err != nil {
		s.DoFinish(err)
	}

	return
}

// SendMsg sends a message to the server.
// If an error is returned, DoFinish is called to record the end of stream.
func (s *CommonStream) SendMsg(ctx context.Context, m interface{}) (err error) {
	if s.sendFixCtx {
		// Custom send middleware present: some middleware relies on rpcinfo from
		// ctx. Guard against callers passing a ctx lacking (or with a different)
		// rpcinfo by re-injecting the stream's own rpcinfo.
		if ri := rpcinfo.GetRPCInfo(ctx); ri != s.ri {
			ctx = rpcinfo.NewCtxWithRPCInfo(ctx, s.ri)
		}
	}
	err = s.send(ctx, s.ClientStream, m)
	s.HandleSendEvent(err)
	if err != nil {
		s.DoFinish(err)
	}
	return
}

// RecvMsg receives a message from the server.
// If an error is returned, DoFinish is called to record the end of stream.
func (s *CommonStream) RecvMsg(ctx context.Context, m interface{}) (err error) {
	if s.recvFixCtx {
		if ri := rpcinfo.GetRPCInfo(ctx); ri != s.ri {
			ctx = rpcinfo.NewCtxWithRPCInfo(ctx, s.ri)
		}
	}
	err = s.recvWithTimeout(ctx, m)
	return s.HandleRecvResult(err)
}

// HandleRecvResult applies the common client-stream recv post-processing to an
// already executed receive operation. It is used both by the streamx RecvMsg
// path and by gRPC-compatible adapters whose recv middleware has a different
// function signature from streamx.
func (s *CommonStream) HandleRecvResult(err error) error {
	if err == nil {
		// If the transport stores BizStatusErr in RPCInfo during a successful
		// recv, surface it to the caller. Transports that return the biz error
		// directly will skip this branch because err is already non-nil.
		err = s.ri.Invocation().BizStatusErr()
	}
	s.HandleRecvEvent(err)
	if err != nil || s.streamingMode == serviceinfo.StreamingClient {
		s.DoFinish(err)
	}
	return err
}

// HandleSendResult applies the common client-stream send post-processing to an
// already executed send operation. It is used by gRPC-compatible adapters whose
// send middleware has a different function signature from streamx.
func (s *CommonStream) HandleSendResult(err error) error {
	s.HandleSendEvent(err)
	if err != nil {
		s.DoFinish(err)
	}
	return err
}

// HandleSendEvent reports a stream send event to the tracer.
func (s *CommonStream) HandleSendEvent(err error) {
	s.traceCtl.HandleStreamSendEvent(s.ctx, s.ri, rpcinfo.StreamSendEvent{Err: err})
}

// HandleRecvEvent reports a stream recv event to the tracer.
func (s *CommonStream) HandleRecvEvent(err error) {
	s.traceCtl.HandleStreamRecvEvent(s.ctx, s.ri, rpcinfo.StreamRecvEvent{Err: err})
}

func (s *CommonStream) recvWithTimeout(ctx context.Context, m interface{}) error {
	if !s.opts.EnableRecvTimeout || s.opts.RecvTmCfg.Timeout <= 0 {
		return s.recv(ctx, s.ClientStream, m)
	}
	return callWithTimeout(s.opts.RecvTmCfg,
		func() error { return s.recv(ctx, s.ClientStream, m) },
		s.Cancel,
	)
}

// RecvTimeoutEnabled reports whether RecvWithTimeout would run call through the
// timeout path.
func (s *CommonStream) RecvTimeoutEnabled() bool {
	return s.opts.EnableRecvTimeout && s.opts.RecvTmCfg.Timeout > 0
}

// RecvWithTimeout executes call with the stream recv timeout policy, if enabled.
// It is used by gRPC-compatible adapters whose recv middleware has a different
// function signature from streamx.
func (s *CommonStream) RecvWithTimeout(call func() error) error {
	if !s.opts.EnableRecvTimeout || s.opts.RecvTmCfg.Timeout <= 0 {
		return call()
	}
	return callWithTimeout(s.opts.RecvTmCfg, call, s.Cancel)
}

// Cancel terminates the local stream lifecycle and cancels the remote peer when
// the underlying transport supports it (currently only gRPC).
func (s *CommonStream) Cancel(err error) {
	if c, ok := s.ClientStream.(internal_stream.CancelableClientStream); ok {
		c.CancelWithErr(err)
	}
}

// DoFinish records the end of stream. It is idempotent and filters non-RPC errors
// before invoking OnFinish. Trace finish and connection release, if any, are the
// responsibility of the injected OnFinish hook.
func (s *CommonStream) DoFinish(err error) {
	if atomic.SwapUint32(&s.finished, 1) == 1 {
		return
	}
	if !isRPCError(err) {
		err = nil
	}
	if s.finishHandler != nil {
		s.finishHandler.OnFinish(err, s.ri)
		return
	}
	if s.onFinish != nil {
		s.onFinish(err, s.ri)
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

// IsRPCError reports whether err should be treated as an RPC-level stream
// failure for finish reporting. BizStatusError is intentionally excluded because
// tracers can read it from rpcinfo.Invocation().BizStatusErr().
func IsRPCError(err error) bool { return isRPCError(err) }

func callWithTimeout(tmCfg streaming.TimeoutConfig, call func() error, cancel func(error)) error {
	timer := time.NewTimer(tmCfg.Timeout)
	defer timer.Stop()
	finishChan := make(chan error, 1)
	gopool.Go(func() {
		var callErr error
		defer func() {
			if r := recover(); r != nil {
				callErr = status.Errorf(codes.Internal, "stream Recv panic, panic=%v, stack=%s", r, debug.Stack())
				cancel(callErr)
			}
			finishChan <- callErr
		}()
		callErr = call()
	})
	select {
	case <-timer.C:
		err := status.Errorf(codes.RecvDeadlineExceeded, recvTimeoutErrTpl, tmCfg)
		if !tmCfg.DisableCancelRemote {
			// finish the stream lifecycle so that the goroutine could exit
			cancel(err)
		}
		return err
	case callErr := <-finishChan:
		return callErr
	}
}
