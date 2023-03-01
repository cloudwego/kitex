/*
 * Copyright 2023 CloudWeGo Authors
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

package fallback

import (
	"context"
	"errors"
	"testing"

	mockthrift "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

func TestNewFallbackPolicy(t *testing.T) {
	// case0: policy is nil
	var fbP *Policy
	result := mockthrift.NewMockTestResult()
	fbResp, fbErr, reportAsFallback := fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), result, mockErr)
	test.Assert(t, fbResp == result)
	test.Assert(t, fbErr == mockErr)
	test.Assert(t, !reportAsFallback)

	// case1: original error is non-nil and set result in fallback
	fbP = NewFallbackPolicy(func(ctx context.Context, req, resp interface{}, err error) (fbResp interface{}, fbErr error) {
		if setResp, ok := resp.(utils.KitexResult); ok {
			setResp.SetSuccess(&retStr)
		}
		return resp, nil
	})
	test.Assert(t, !fbP.ReportAsFallback)
	test.Assert(t, fbP.FallbackFunc != nil)
	fbResp, fbErr, reportAsFallback = fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), mockthrift.NewMockTestResult(), mockErr)
	test.Assert(t, fbResp != nil)
	test.Assert(t, *fbResp.(utils.KitexResult).GetResult().(*string) == retStr)
	test.Assert(t, fbErr == nil)
	test.Assert(t, !reportAsFallback)

	// case2: enable reportAsFallback
	fbP = NewFallbackPolicy(func(ctx context.Context, req, resp interface{}, err error) (fbResp interface{}, fbErr error) {
		if setResp, ok := resp.(utils.KitexResult); ok {
			setResp.SetSuccess(&retStr)
		}
		return resp, nil
	}).EnableReportAsFallback()
	test.Assert(t, fbP.ReportAsFallback)
	fbResp, fbErr, reportAsFallback = fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), mockthrift.NewMockTestResult(), mockErr)
	test.Assert(t, reportAsFallback)

	// case3: original error is nil, still can update result
	fbP = NewFallbackPolicy(func(ctx context.Context, req, resp interface{}, err error) (fbResp interface{}, fbErr error) {
		if setResp, ok := resp.(utils.KitexResult); ok {
			setResp.SetSuccess(&retStr)
		}
		return resp, nil
	}).EnableReportAsFallback()
	test.Assert(t, fbP.ReportAsFallback)
	fbResp, fbErr, reportAsFallback = fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), mockthrift.NewMockTestResult(), nil)
	test.Assert(t, fbResp != nil)
	test.Assert(t, *fbResp.(utils.KitexResult).GetResult().(*string) == retStr)
	test.Assert(t, fbErr == nil)
	test.Assert(t, !reportAsFallback)

	// case4: fallback return nil-resp and nil-err, framework will return the original resp and err
	fbP = NewFallbackPolicy(func(ctx context.Context, req, resp interface{}, err error) (fbResp interface{}, fbErr error) {
		return
	}).EnableReportAsFallback()
	test.Assert(t, fbP.ReportAsFallback)
	fbResp, fbErr, reportAsFallback = fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), mockthrift.NewMockTestResult(), mockErr)
	test.Assert(t, fbErr == mockErr)
	test.Assert(t, !reportAsFallback)
}

func TestErrorFallback(t *testing.T) {
	// case1: err is non-nil
	fbP := ErrorFallback(func(ctx context.Context, req, resp interface{}, err error) (fbResp interface{}, fbErr error) {
		fbResp = mockthrift.NewMockTestResult()
		if setResp, ok := fbResp.(utils.KitexResult); ok {
			setResp.SetSuccess(&retStr)
		}
		return fbResp, nil
	})
	fbResp, fbErr, reportAsFallback := fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), mockthrift.NewMockTestResult(), mockErr)
	test.Assert(t, fbResp != nil)
	test.Assert(t, *fbResp.(utils.KitexResult).GetResult().(*string) == retStr)
	test.Assert(t, fbErr == nil)
	test.Assert(t, !reportAsFallback)

	// case 2: err is nil, then the fallback func won't be executed
	fbP = ErrorFallback(func(ctx context.Context, req, resp interface{}, err error) (fbResp interface{}, fbErr error) {
		fbResp = mockthrift.NewMockTestResult()
		if setResp, ok := fbResp.(utils.KitexResult); ok {
			setResp.SetSuccess(&retStr)
		}
		return fbResp, nil
	})
	result := mockthrift.NewMockTestResult()
	fbResp, fbErr, reportAsFallback = fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), result, nil)
	test.Assert(t, fbResp == result)
	test.Assert(t, fbErr == nil)
}

func TestTimeoutAndCBFallback(t *testing.T) {
	// case1: rpc timeout will do fallback
	fbP := TimeoutAndCBFallback(func(ctx context.Context, req, resp interface{}, err error) (fbResp interface{}, fbErr error) {
		fbResp = mockthrift.NewMockTestResult()
		if setResp, ok := fbResp.(utils.KitexResult); ok {
			setResp.SetSuccess(&retStr)
		}
		return fbResp, nil
	})
	result := mockthrift.NewMockTestResult()
	fbResp, fbErr, reportAsFallback := fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), result, kerrors.ErrRPCTimeout.WithCause(mockErr))
	test.Assert(t, fbResp != nil)
	test.Assert(t, *fbResp.(utils.KitexResult).GetResult().(*string) == retStr)
	test.Assert(t, fbErr == nil)
	test.Assert(t, !reportAsFallback)

	// case2: circuit breaker error will do fallback
	fbP = TimeoutAndCBFallback(func(ctx context.Context, req, resp interface{}, err error) (fbResp interface{}, fbErr error) {
		fbResp = mockthrift.NewMockTestResult()
		if setResp, ok := fbResp.(utils.KitexResult); ok {
			setResp.SetSuccess(&retStr)
		}
		return fbResp, nil
	}).EnableReportAsFallback()
	result = mockthrift.NewMockTestResult()
	fbResp, fbErr, reportAsFallback = fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), result, kerrors.ErrCircuitBreak.WithCause(mockErr))
	test.Assert(t, fbResp != nil)
	test.Assert(t, *fbResp.(utils.KitexResult).GetResult().(*string) == retStr)
	test.Assert(t, fbErr == nil)
	test.Assert(t, reportAsFallback)

	// case3: err is non-nil, but not rpc timeout or circuit breaker
	fbP = TimeoutAndCBFallback(func(ctx context.Context, req, resp interface{}, err error) (fbResp interface{}, fbErr error) {
		fbResp = mockthrift.NewMockTestResult()
		if setResp, ok := fbResp.(utils.KitexResult); ok {
			setResp.SetSuccess(&retStr)
		}
		return fbResp, nil
	})
	result = mockthrift.NewMockTestResult()
	fbResp, fbErr, reportAsFallback = fbP.DoIfNeeded(context.Background(), genRPCInfo(), mockthrift.NewMockTestArgs(), result, mockErr)
	test.Assert(t, fbResp == result)
	test.Assert(t, fbErr == mockErr)
	test.Assert(t, !reportAsFallback)
}

var (
	method  = "test"
	mockErr = errors.New("mock")
	retStr  = "success"
)

func genRPCInfo() rpcinfo.RPCInfo {
	to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{Method: method}, method).ImmutableView()
	ri := rpcinfo.NewRPCInfo(to, to, rpcinfo.NewInvocation("", method), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	return ri
}
