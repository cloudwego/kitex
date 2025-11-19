/*
 * Copyright 2025 CloudWeGo Authors
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
	"errors"
	"io"
	"time"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	streaming_types "github.com/cloudwego/kitex/pkg/streaming/types"
)

func handleGRPC(ctx context.Context, ri rpcinfo.RPCInfo, err error, callback *streaming.FinishCallback) {
	if err == nil || err == io.EOF {
		return
	}

	// business err processing
	// todo: optimize this function
	if gRPCBizErr, ok := err.(*kerrors.GRPCBizStatusError); ok && callback.OnBusinessError != nil {
		callback.OnBusinessError(ctx, gRPCBizErr)
		return
	}
	st, ok := status.FromError(err)
	if !ok {
		handleNonStatusError(ctx, err, callback)
		return
	}

	// framework/proxy err processing
	handleGRPCStatus(ctx, ri, err, st, callback)
}

// handleNonStatusError handles errors that are not gRPC status errors
func handleNonStatusError(ctx context.Context, err error, callback *streaming.FinishCallback) {
	if callback.OnBusinessError == nil {
		return
	}

	// client-side streaming callback or remote handler return BizStatusErr
	bizErr, bizOK := kerrors.FromBizStatusError(err)
	if !bizOK {
		// just unify err handling, do not modify BizStatusErr in rpcinfo
		bizErr = kerrors.NewGRPCBizStatusError(int32(codes.Internal), err.Error())
	}
	callback.OnBusinessError(ctx, bizErr)
}

// handleFrameworkError handles framework and proxy errors based on gRPC status code
func handleGRPCStatus(ctx context.Context, ri rpcinfo.RPCInfo, err error, st *status.Status, callback *streaming.FinishCallback) {
	switch {
	case st.IsFromBusiness(): // remote handler/mw return non-BizStatusErr err
		if callback.OnBusinessError != nil {
			// do not set BizStatusError in rpcinfo.Invocation
			// because we do not hope modifying related tracer/metrics behaviour
			bizErr := kerrors.NewGRPCBizStatusError(int32(st.Code()), st.Message())
			callback.OnBusinessError(ctx, bizErr)
		}
	case st.CancelType() != 0:
		handleGRPCCancelStatus(ctx, ri, err, st, callback)
	case st.TimeoutType() != 0:
		handleGRPCTimeoutStatus(ctx, ri, err, st, callback)
	default:
		if callback.OnWireError != nil {
			callback.OnWireError(ctx, wireError{
				err:  err,
				code: int32(st.Code()),
			})
		}
	}
}

func handleGRPCCancelStatus(ctx context.Context, ri rpcinfo.RPCInfo, err error, st *status.Status, callback *streaming.FinishCallback) {
	if callback.OnCancelError == nil {
		return
	}

	var finalErr streaming_types.CancelError
	cErr := cancelError{
		err:  err,
		code: int32(codes.Canceled),
	}
	cancelType := st.CancelType()
	switch cancelType {
	case streaming_types.LocalCascadedCancel:
		cErr.typ = cancelType
		finalErr = localCascadedCancelError{
			cancelError: cErr,
		}
	case streaming_types.RemoteCascadedCancel:
		cErr.typ = cancelType
		// gRPC only supports tracing the previous hop service
		triggerBy := ri.From().ServiceName()
		if triggerBy == "" {
			triggerBy = "remote service"
		}
		finalErr = remoteCascadedCancelError{
			cancelError: cErr,
			cancelPath:  []string{triggerBy},
		}
	case streaming_types.ActiveCancel:
		cErr.typ = cancelType
		finalErr = activeCancelError{
			cancelError: cErr,
		}
	}
	callback.OnCancelError(ctx, finalErr)
}

func handleGRPCTimeoutStatus(ctx context.Context, ri rpcinfo.RPCInfo, err error, st *status.Status, callback *streaming.FinishCallback) {
	if callback.OnTimeoutError == nil {
		return
	}

	tErr := timeoutError{
		err:  err,
		code: int32(codes.DeadlineExceeded),
	}
	switch st.TimeoutType() {
	case streaming_types.StreamTimeout:
		tErr.timeoutType = streaming_types.StreamTimeout
		tErr.timeout = ri.Config().StreamTimeout()
	case streaming_types.StreamRecvTimeout:
		tErr.timeoutType = streaming_types.StreamRecvTimeout
		tErr.timeout = ri.Config().StreamRecvTimeout()
	case streaming_types.StreamSendTimeout:
		tErr.timeoutType = streaming_types.StreamSendTimeout
		tErr.timeout = ri.Config().StreamSendTimeout()
	}

	callback.OnTimeoutError(ctx, tErr)
}

func handleTTStream(ctx context.Context, ri rpcinfo.RPCInfo, err error, callback *streaming.FinishCallback) {
	if err == nil || err == io.EOF {
		return
	}

	ex, ok := err.(*ttstream.Exception)
	if !ok {
		if callback.OnBusinessError == nil {
			return
		}
		// remote side return BizStatusError
		bizErr, bizOK := kerrors.FromBizStatusError(err)
		if !bizOK {
			// just unify err handling, do not modify BizStatusErr in rpcinfo
			bizErr = kerrors.NewBizStatusError(ttstream.ErrApplicationException.TypeId(), err.Error())
		}
		callback.OnBusinessError(ctx, bizErr)
		return
	}
	if errors.Is(ex, ttstream.ErrApplicationException) {
		if callback.OnBusinessError == nil {
			return
		}
		bizErr := kerrors.NewBizStatusError(ex.TypeId(), ex.Error())
		callback.OnBusinessError(ctx, bizErr)
		return
	}

	switch {
	case errors.Is(ex, kerrors.ErrStreamingProtocol):
		if callback.OnWireError != nil {
			wErr := wireError{
				err:  err,
				code: ex.TypeId(),
			}
			callback.OnWireError(ctx, wErr)
		}
	case errors.Is(ex, kerrors.ErrStreamingTimeout):
		handleTTStreamTimeoutException(ctx, ri, err, ex, callback)
	case errors.Is(ex, kerrors.ErrStreamingCanceled):
		handleTTStreamCancelException(ctx, err, ex, callback)
	default:
		if callback.OnWireError != nil {
			wErr := wireError{
				err:  err,
				code: ex.TypeId(),
			}
			callback.OnWireError(ctx, wErr)
		}
	}
}

func handleTTStreamTimeoutException(ctx context.Context, ri rpcinfo.RPCInfo, err error, ex *ttstream.Exception, callback *streaming.FinishCallback) {
	if callback.OnTimeoutError == nil {
		return
	}

	tErr := timeoutError{
		err:  err,
		code: ex.TypeId(),
	}
	switch ex.TimeoutType() {
	case streaming_types.StreamTimeout:
		tErr.timeoutType = streaming_types.StreamTimeout
		tErr.timeout = ri.Config().StreamTimeout()
	case streaming_types.StreamRecvTimeout:
		tErr.timeoutType = streaming_types.StreamRecvTimeout
		tErr.timeout = ri.Config().StreamRecvTimeout()
	case streaming_types.StreamSendTimeout:
		tErr.timeoutType = streaming_types.StreamSendTimeout
		tErr.timeout = ri.Config().StreamSendTimeout()
	}

	callback.OnTimeoutError(ctx, tErr)
}

func handleTTStreamCancelException(ctx context.Context, err error, ex *ttstream.Exception, callback *streaming.FinishCallback) {
	if callback.OnCancelError == nil {
		return
	}

	var finalErr streaming_types.CancelError
	cErr := cancelError{
		err:  err,
		code: ex.TypeId(),
	}
	cancelType := ex.CancelType()
	cErr.typ = cancelType
	switch cancelType {
	case streaming_types.LocalCascadedCancel:
		finalErr = localCascadedCancelError{
			cancelError: cErr,
		}
	case streaming_types.RemoteCascadedCancel:
		finalErr = remoteCascadedCancelError{
			cancelError: cErr,
			cancelPath:  ex.CancelPaths(),
		}
	case streaming_types.ActiveCancel:
		finalErr = activeCancelError{
			cancelError: cErr,
		}
	}
	callback.OnCancelError(ctx, finalErr)
}

func handleBizStatusError(ctx context.Context, bizErr kerrors.BizStatusErrorIface, callback *streaming.FinishCallback) {
	if callback.OnBusinessError != nil {
		callback.OnBusinessError(ctx, bizErr)
	}
}

var (
	_ streaming_types.WireError    = wireError{}
	_ streaming_types.CancelError  = cancelError{}
	_ streaming_types.TimeoutError = timeoutError{}
)

type wireError struct {
	err  error
	code int32
}

func (we wireError) Error() string {
	return we.err.Error()
}

func (we wireError) StatusCode() int32 {
	return we.code
}

type remoteCascadedCancelError struct {
	cancelError
	cancelPath []string
}

func (c remoteCascadedCancelError) CancelPath() []streaming_types.CancelPoint {
	if c.cancelPath == nil {
		return nil
	}
	// todo: 内存分配优化
	res := make([]streaming_types.CancelPoint, len(c.cancelPath))
	for i, p := range c.cancelPath {
		res[i] = streaming_types.CancelPoint{Name: p}
	}
	return res
}

type localCascadedCancelError struct {
	cancelError
}

type activeCancelError struct {
	cancelError
}

type cancelError struct {
	err  error
	code int32
	typ  streaming_types.CancelType
}

func (ce cancelError) Error() string {
	return ce.err.Error()
}

func (ce cancelError) StatusCode() int32 {
	return ce.code
}

func (ce cancelError) Type() streaming_types.CancelType {
	return ce.typ
}

type timeoutError struct {
	err         error
	code        int32
	timeoutType streaming_types.TimeoutType
	timeout     time.Duration
}

func (te timeoutError) Error() string {
	return te.err.Error()
}

func (te timeoutError) StatusCode() int32 {
	return te.code
}

func (te timeoutError) TimeoutType() streaming_types.TimeoutType {
	return te.timeoutType
}

func (te timeoutError) Timeout() time.Duration {
	return te.timeout
}
