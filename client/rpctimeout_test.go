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
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpctimeout"
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

	var moreTimeout time.Duration
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), r)
	mwCtx := context.Background()
	mwCtx = context.WithValue(mwCtx, rpctimeout.TimeoutAdjustKey, &moreTimeout)

	var err error
	var mw1, mw2 endpoint.Middleware

	// 1. normal
	mw1 = rpctimeout.MiddlewareBuilder(0)(mwCtx)
	mw2 = rpcTimeoutMW(mwCtx)
	err = mw1(mw2(pass))(ctx, nil, nil)
	test.Assert(t, err == nil)

	// 2. block to mock timeout
	mw1 = rpctimeout.MiddlewareBuilder(0)(mwCtx)
	mw2 = rpcTimeoutMW(mwCtx)
	err = mw1(mw2(block))(ctx, nil, nil)
	test.Assert(t, err != nil, err)
	test.Assert(t, err.(*kerrors.DetailedError).ErrorType() == kerrors.ErrRPCTimeout)

	// 3. block, pass more timeout, timeout won't happen
	mw1 = rpctimeout.MiddlewareBuilder(510 * time.Millisecond)(mwCtx)
	mw2 = rpcTimeoutMW(mwCtx)
	err = mw1(mw2(block))(ctx, nil, nil)
	test.Assert(t, err == nil)

	// 4. mock panic happen
	// < v1.1.* panic happen, >=v1.1* wrap panic to error
	mw1 = rpctimeout.MiddlewareBuilder(0)(mwCtx)
	mw2 = rpcTimeoutMW(mwCtx)
	err = mw1(mw2(ache))(ctx, nil, nil)
	test.Assert(t, strings.Contains(err.Error(), panicMsg))
}

func TestHandleCtxDone(t *testing.T) {
	to := rpcinfo.NewEndpointInfo("service", "method", nil, nil)
	ri := rpcinfo.NewRPCInfo(nil, to, nil, nil, nil)
	canceledCtx, cancel := context.WithCancel(rpcinfo.NewCtxWithRPCInfo(context.Background(), ri))
	cancel()

	type args struct {
		ctx     context.Context
		start   time.Time
		timeout time.Duration
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "nil rpcinfo",
			args: args{
				ctx:     context.Background(),
				start:   time.Time{},
				timeout: time.Duration(0),
			},
			wantErr: false,
		},
		{
			name: "normal",
			args: args{
				ctx:     rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
				start:   time.Time{},
				timeout: time.Duration(0),
			},
			wantErr: true,
		},
		{
			name: "context canceled",
			args: args{
				ctx:     canceledCtx,
				start:   time.Time{},
				timeout: time.Duration(0),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleCtxDone(tt.args.ctx, tt.args.start, tt.args.timeout); (err != nil) != tt.wantErr {
				t.Errorf("handleCtxDone() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
