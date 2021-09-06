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

package rpctimeout

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackedelic/kitex/internal/pkg/endpoint"
	"github.com/jackedelic/kitex/internal/pkg/kerrors"
	"github.com/jackedelic/kitex/internal/pkg/klog"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
	"github.com/jackedelic/kitex/internal/test"
)

var panicMsg = "hello world"

func block(ctx context.Context, request, response interface{}) (err error) {
	time.Sleep(1 * time.Second)
	return nil
}

func pass(ctx context.Context, request, response interface{}) (err error) {
	time.Sleep(200 * time.Millisecond)
	return nil
}

func ache(ctx context.Context, request, response interface{}) (err error) {
	panic(panicMsg)
}

func TestNewRPCTimeoutMW(t *testing.T) {
	t.Parallel()

	s := rpcinfo.NewEndpointInfo("mockService", "mockMethod", nil, nil)
	c := rpcinfo.NewRPCConfig()
	r := rpcinfo.NewRPCInfo(nil, s, nil, c, rpcinfo.NewRPCStats())
	m := rpcinfo.AsMutableRPCConfig(c)
	m.SetRPCTimeout(time.Millisecond * 500)

	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), r)
	mwCtx := context.Background()
	mwCtx = context.WithValue(mwCtx, endpoint.CtxLoggerKey, klog.DefaultLogger())

	var err error
	// 1. normal
	err = MiddlewareBuilder(0)(mwCtx)(pass)(ctx, nil, nil)
	test.Assert(t, err == nil)

	// 2. block to mock timeout
	err = MiddlewareBuilder(0)(mwCtx)(block)(ctx, nil, nil)
	test.Assert(t, err != nil, err)
	test.Assert(t, err.(*kerrors.DetailedError).ErrorType() == kerrors.ErrRPCTimeout)

	// 3. block, pass more timeout, timeout won't happen
	err = MiddlewareBuilder(510*time.Millisecond)(mwCtx)(block)(ctx, nil, nil)
	test.Assert(t, err == nil)

	// 4. mock panic happen
	// < v1.1.* panic happen, >=v1.1* wrap panic to error
	err = MiddlewareBuilder(0)(mwCtx)(ache)(ctx, nil, nil)
	test.Assert(t, strings.Contains(err.Error(), panicMsg))

	// 5. cancel
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	time.AfterFunc(100*time.Millisecond, func() {
		cancelFunc()
	})
	err = MiddlewareBuilder(0)(mwCtx)(block)(cancelCtx, nil, nil)
	test.Assert(t, errors.Is(err, context.Canceled), err)
}
