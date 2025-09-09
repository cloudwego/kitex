//go:build !windows

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

package ttstream

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func newTestClientStream(ctx context.Context) *clientStream {
	return newClientStream(ctx, mockStreamWriter{}, streamFrame{})
}

func Test_clientStreamStateChange(t *testing.T) {
	t.Run("serverStream close, then RecvMsg/SendMsg returning exception", func(t *testing.T) {
		cliSt := newTestClientStream(context.Background())
		testException := errors.New("test")
		cliSt.close(testException, false, "")
		rErr := cliSt.RecvMsg(cliSt.ctx, nil)
		test.Assert(t, rErr == testException, rErr)
		sErr := cliSt.SendMsg(cliSt.ctx, nil)
		test.Assert(t, sErr == testException, sErr)
	})
	t.Run("serverStream close twice with different exception, RecvMsg/SendMsg returning the first time exception", func(t *testing.T) {
		cliSt := newTestClientStream(context.Background())
		testException1 := errors.New("test1")
		cliSt.close(testException1, false, "")
		testException2 := errors.New("test2")
		cliSt.close(testException2, false, "")

		rErr := cliSt.RecvMsg(cliSt.ctx, nil)
		test.Assert(t, rErr == testException1, rErr)
		sErr := cliSt.SendMsg(cliSt.ctx, nil)
		test.Assert(t, sErr == testException1, sErr)
	})
	t.Run("serverStream close concurrently with RecvMsg/SendMsg", func(t *testing.T) {
		cliSt := newTestClientStream(context.Background())
		var wg sync.WaitGroup
		wg.Add(2)
		testException := errors.New("test")
		go func() {
			time.Sleep(100 * time.Millisecond)
			cliSt.close(testException, false, "")
		}()
		go func() {
			defer wg.Done()
			rErr := cliSt.RecvMsg(cliSt.ctx, nil)
			test.Assert(t, rErr == testException, rErr)
		}()
		go func() {
			defer wg.Done()
			for {
				sErr := cliSt.SendMsg(cliSt.ctx, &testRequest{})
				if sErr == nil {
					continue
				}
				test.Assert(t, sErr == testException, sErr)
				break
			}
		}()
		wg.Wait()
	})
}

func Test_clientStream_parseCtxErr(t *testing.T) {
	testcases := []struct {
		desc             string
		ctxFunc          func() (context.Context, context.CancelFunc)
		expectEx         *Exception
		expectCancelPath string
	}{
		{
			desc: "invoke cancel() actively",
			ctxFunc: func() (context.Context, context.CancelFunc) {
				ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), rpcinfo.NewRPCInfo(
					rpcinfo.NewEndpointInfo("serviceA", "testMethod", nil, nil), nil, nil, nil, nil))
				return context.WithCancel(ctx)
			},
			expectEx:         errBizCancel.newBuilder().withSide(clientSide),
			expectCancelPath: "serviceA",
		},
		{
			desc: "cascading cancel",
			ctxFunc: func() (context.Context, context.CancelFunc) {
				ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), rpcinfo.NewRPCInfo(
					rpcinfo.NewEndpointInfo("serviceB", "testMethod", nil, nil), nil, nil, nil, nil))
				ctx, cancel := context.WithCancel(ctx)
				ctx, cancelFunc := newContextWithCancelReason(ctx, cancel)
				cause := defaultRstException
				return ctx, func() {
					cancelFunc(errUpstreamCancel.newBuilder().withSide(serverSide).setOrAppendCancelPath("serviceA").withCauseAndTypeId(cause, cause.TypeId()))
				}
			},
			expectEx:         errUpstreamCancel.newBuilder().withSide(clientSide).setOrAppendCancelPath("serviceA").withCauseAndTypeId(defaultRstException, defaultRstException.TypeId()),
			expectCancelPath: "serviceA,serviceB",
		},
		{
			desc: "other customized ctx Err",
			ctxFunc: func() (context.Context, context.CancelFunc) {
				ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), rpcinfo.NewRPCInfo(
					rpcinfo.NewEndpointInfo("serviceA", "testMethod", nil, nil), nil, nil, nil, nil))
				ctx, cancel := context.WithCancel(ctx)
				ctx, cancelFunc := newContextWithCancelReason(ctx, cancel)
				return ctx, func() {
					cancelFunc(errors.New("test"))
				}
			},
			expectEx:         errInternalCancel.newBuilder().withSide(clientSide).withCause(errors.New("test")),
			expectCancelPath: "non-ttstream path,serviceA",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx, cancel := tc.ctxFunc()
			cs := newClientStream(ctx, nil, streamFrame{})
			cancel()
			finalEx, cancelPath := cs.parseCtxErr(ctx)
			test.DeepEqual(t, finalEx, tc.expectEx)
			test.Assert(t, tc.expectCancelPath == cancelPath, cancelPath)
		})
	}
}

func Test_clientStream_SendMsg(t *testing.T) {
	ctx := context.Background()
	cs := newClientStream(ctx, &mockStreamWriter{}, streamFrame{})
	req := &testRequest{B: "SendMsgTest"}

	// Send successfully
	err := cs.SendMsg(ctx, req)
	test.Assert(t, err == nil, err)
	// CloseSend
	err = cs.CloseSend(ctx)
	test.Assert(t, err == nil, err)
	err = cs.SendMsg(ctx, req)
	test.Assert(t, err != nil)
	test.Assert(t, errors.Is(err, errIllegalOperation), err)
	test.Assert(t, strings.Contains(err.Error(), "stream is closed send"))

	cs = newClientStream(ctx, &mockStreamWriter{}, streamFrame{})
	// Send retrieves the close stream exception
	ex := errDownstreamCancel.newBuilder().withSide(clientSide)
	cs.close(ex, false, "")
	err = cs.SendMsg(ctx, req)
	test.Assert(t, errors.Is(err, errDownstreamCancel), err)
	// subsequent close will not affect the error returned by Send
	ex1 := errApplicationException.newBuilder().withSide(clientSide)
	cs.close(ex1, false, "")
	err = cs.SendMsg(ctx, req)
	test.Assert(t, errors.Is(err, errDownstreamCancel), err)
}
