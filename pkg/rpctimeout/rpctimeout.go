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

// Package rpctimeout implements logic for timeout controlling.
package rpctimeout

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func recoverFunc(ctx context.Context, logger klog.FormatLogger, ri rpcinfo.RPCInfo, done chan error) {
	if err := recover(); err != nil {
		e := fmt.Errorf("KITEX: panic, remote[to_psm=%s|method=%s], err=%v\n%s",
			ri.To().ServiceName(), ri.To().Method(), err, debug.Stack())
		if l, ok := logger.(klog.CtxLogger); ok {
			l.CtxErrorf(ctx, "%s", e.Error())
		} else {
			logger.Errorf("%s", e.Error())
		}
		rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats())
		rpcStats.SetPanicked(err)
		done <- e
	}
	close(done)
}

// MiddlewareBuilder builds timeout middleware.
func MiddlewareBuilder(moreTimeout time.Duration) endpoint.MiddlewareBuilder {
	return func(mwCtx context.Context) endpoint.Middleware {
		logger := mwCtx.Value(endpoint.CtxLoggerKey).(klog.FormatLogger)
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, request, response interface{}) error {
				var err error
				ri := rpcinfo.GetRPCInfo(ctx)
				tm := ri.Config().RPCTimeout() + moreTimeout

				start := time.Now()
				ctx, cancel := context.WithTimeout(ctx, tm)
				defer cancel()

				done := make(chan error, 1)
				gofunc.GoFunc(ctx, func() {
					defer recoverFunc(ctx, logger, ri, done)
					err = next(ctx, request, response)
					if err != nil && ctx.Err() != nil && !errors.Is(err, kerrors.ErrRPCFinish) {
						// error occurs after the wait goroutine returns,
						// we should log this error or it will be discarded.
						// Specially, ErrRPCFinish can be ignored, it happens in retry scene, previous call returns first.
						var errMsg string
						if ri.To().Address() != nil {
							errMsg = fmt.Sprintf("KITEX: remote[to_psm=%s|method=%s|addr=%s]，err=%s",
								ri.To().ServiceName(), ri.To().Method(), ri.To().Address(), err.Error())
						} else {
							errMsg = fmt.Sprintf("KITEX: remote[to_psm=%s|method=%s], err=%s",
								ri.To().ServiceName(), ri.To().Method(), err.Error())
						}
						if l, ok := logger.(klog.CtxLogger); ok {
							l.CtxErrorf(ctx, "%s", errMsg)
						} else {
							logger.Errorf("%s", errMsg)
						}
					}
				})

				select {
				case panicErr := <-done:
					if panicErr != nil {
						return panicErr
					}
					return err
				case <-ctx.Done():
					return makeTimeoutErr(ctx, start)
				}
			}
		}
	}
}

func makeTimeoutErr(ctx context.Context, start time.Time) error {
	ri := rpcinfo.GetRPCInfo(ctx)
	to := ri.To()
	timeout := ri.Config().RPCTimeout()

	errMsg := fmt.Sprintf("timeout=%v, to=%s, method=%s", timeout, to.ServiceName(), to.Method())
	target := to.Address()
	if target != nil {
		errMsg = fmt.Sprintf("%s, remote=%s", errMsg, target.String())
	}

	// cancel error
	if ctx.Err() == context.Canceled {
		return kerrors.ErrRPCTimeout.WithCause(fmt.Errorf("%s: %w", errMsg, ctx.Err()))
	}

	if ddl, ok := ctx.Deadline(); !ok {
		errMsg = fmt.Sprintf("%s, %s", errMsg, "unknown error: context deadline not set?")
	} else {
		if ddl.Before(start.Add(timeout)) {
			errMsg = fmt.Sprintf("%s, context deadline earlier than timeout, actual=%v", errMsg, ddl.Sub(start))
		}
	}
	return kerrors.ErrRPCTimeout.WithCause(errors.New(errMsg))
}
