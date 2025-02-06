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
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpctimeout"
)

// workerPool is used to reduce the timeout goroutine overhead.
// if timeout middleware is not enabled, it will not cause any extra overhead
var workerPool = newTimeoutPool(128, time.Minute)

func makeTimeoutErr(ctx context.Context, start time.Time, timeout time.Duration) error {
	ri := rpcinfo.GetRPCInfo(ctx)
	to := ri.To()

	errMsg := fmt.Sprintf("timeout=%v, to=%s, method=%s, location=%s",
		timeout, to.ServiceName(), to.Method(), "kitex.rpcTimeoutMW")
	target := to.Address()
	if target != nil {
		errMsg = fmt.Sprintf("%s, remote=%s", errMsg, target.String())
	}

	needFineGrainedErrCode := rpctimeout.LoadGlobalNeedFineGrainedErrCode()
	// cancel error
	if ctx.Err() == context.Canceled {
		if needFineGrainedErrCode {
			return kerrors.ErrCanceledByBusiness.WithCause(errors.New(errMsg))
		} else {
			return kerrors.ErrRPCTimeout.WithCause(fmt.Errorf("%s: %w by business", errMsg, ctx.Err()))
		}
	}
	if ddl, ok := ctx.Deadline(); !ok {
		errMsg = fmt.Sprintf("%s, %s", errMsg, "unknown error: context deadline not set?")
	} else {
		// Go's timer implementation is not so accurate,
		// so if we need to check ctx deadline earlier than our timeout, we should consider the accuracy
		isBizTimeout := isBusinessTimeout(start, timeout, ddl, rpctimeout.LoadBusinessTimeoutThreshold())
		if isBizTimeout {
			// if timeout set in context is shorter than RPCTimeout in rpcinfo, it will trigger earlier.
			errMsg = fmt.Sprintf("%s, timeout by business, actual=%s", errMsg, ddl.Sub(start))
		} else if roundTimeout := timeout - time.Millisecond; roundTimeout >= 0 && ddl.Before(start.Add(roundTimeout)) {
			errMsg = fmt.Sprintf("%s, context deadline earlier than timeout, actual=%v", errMsg, ddl.Sub(start))
		}

		if needFineGrainedErrCode && isBizTimeout {
			return kerrors.ErrTimeoutByBusiness.WithCause(errors.New(errMsg))
		}
	}
	return kerrors.ErrRPCTimeout.WithCause(errors.New(errMsg))
}

func isBusinessTimeout(start time.Time, kitexTimeout time.Duration, actualDDL time.Time, threshold time.Duration) bool {
	if kitexTimeout <= 0 {
		return true
	}
	kitexDDL := start.Add(kitexTimeout)
	return actualDDL.Add(threshold).Before(kitexDDL)
}

func rpcTimeoutMW(mwCtx context.Context) endpoint.UnaryMiddleware {
	var moreTimeout time.Duration
	if v, ok := mwCtx.Value(rpctimeout.TimeoutAdjustKey).(*time.Duration); ok && v != nil {
		moreTimeout = *v
	}

	return func(next endpoint.UnaryEndpoint) endpoint.UnaryEndpoint {
		backgroundEP := func(ctx context.Context, request, response interface{}) error {
			err := next(ctx, request, response)
			if err != nil && ctx.Err() != nil &&
				!kerrors.IsTimeoutError(err) && !errors.Is(err, kerrors.ErrRPCFinish) {
				ri := rpcinfo.GetRPCInfo(ctx)
				// error occurs after the wait goroutine returns(RPCTimeout happens),
				// we should log this error for troubleshooting, or it will be discarded.
				// but ErrRPCTimeout and ErrRPCFinish can be ignored:
				//    ErrRPCTimeout: it is same with outer timeout, here only care about non-timeout err.
				//    ErrRPCFinish: it happens in retry scene, previous call returns first.
				var errMsg string
				if ri.To().Address() != nil {
					errMsg = fmt.Sprintf("KITEX: to_service=%s method=%s addr=%s error=%s",
						ri.To().ServiceName(), ri.To().Method(), ri.To().Address(), err.Error())
				} else {
					errMsg = fmt.Sprintf("KITEX: to_service=%s method=%s error=%s",
						ri.To().ServiceName(), ri.To().Method(), err.Error())
				}
				klog.CtxErrorf(ctx, "%s", errMsg)
			}
			return err
		}
		return func(ctx context.Context, request, response interface{}) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			tm := ri.Config().RPCTimeout()
			if tm > 0 {
				tm += moreTimeout
			}
			// fast path if no timeout
			if ctx.Done() == nil && tm <= 0 {
				return next(ctx, request, response)
			}
			start := time.Now()
			ctx, err := workerPool.RunTask(ctx, tm, request, response, backgroundEP)
			if err == nil {
				return nil
			}
			if err == ctx.Err() {
				// err is from ctx.Err()
				// either parent is cancelled by user, or timeout
				return makeTimeoutErr(ctx, start, tm)
			}
			return err
		}
	}
}
