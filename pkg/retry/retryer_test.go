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

package retry

import (
	"context"
	"strconv"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func TestNewRetryContainer(t *testing.T) {
	rc := NewRetryContainerWithCB(nil, nil)
	rc.NotifyPolicyChange(method, Policy{
		Enable:        true,
		BackupPolicy:  NewBackupPolicy(10),
		FailurePolicy: NewFailurePolicy(),
	})
	r := rc.getRetryer(genRPCInfo())
	_, ok := r.(*failureRetryer)
	test.Assert(t, ok)

	rc.NotifyPolicyChange(method, Policy{
		Enable:        false,
		BackupPolicy:  NewBackupPolicy(10),
		FailurePolicy: NewFailurePolicy(),
	})
	_, ok = r.(*failureRetryer)
	test.Assert(t, ok)
	_, allow := r.AllowRetry(context.Background())
	test.Assert(t, !allow)

	rc.NotifyPolicyChange(method, Policy{
		Enable:        true,
		BackupPolicy:  NewBackupPolicy(10),
		FailurePolicy: NewFailurePolicy(),
	})
	r = rc.getRetryer(genRPCInfo())
	_, ok = r.(*failureRetryer)
	test.Assert(t, ok)
	_, allow = r.AllowRetry(context.Background())
	test.Assert(t, allow)

	rc.NotifyPolicyChange(method, Policy{
		Enable:       true,
		Type:         1,
		BackupPolicy: NewBackupPolicy(20),
	})
	r = rc.getRetryer(genRPCInfo())
	_, ok = r.(*backupRetryer)
	test.Assert(t, ok)
	_, allow = r.AllowRetry(context.Background())
	test.Assert(t, allow)

	rc.NotifyPolicyChange(method, Policy{
		Enable:       false,
		Type:         1,
		BackupPolicy: NewBackupPolicy(20),
	})
	r = rc.getRetryer(genRPCInfo())
	_, ok = r.(*backupRetryer)
	test.Assert(t, ok)
	_, allow = r.AllowRetry(context.Background())
	test.Assert(t, !allow)

	err := rc.Init(&Policy{
		Enable:       true,
		Type:         1,
		BackupPolicy: NewBackupPolicy(20),
	})
	test.Assert(t, err == nil, err)
	rcDump := rc.Dump()
	_, ok = rcDump.(*Container)
	test.Assert(t, !ok)
}

func TestFailurePolicyCall(t *testing.T) {
	rc := NewRetryContainer()
	err := rc.Init(&Policy{
		Enable:        true,
		Type:          0,
		FailurePolicy: NewFailurePolicy(),
	})
	test.Assert(t, err == nil, err)

	callTimes := 0
	var prevRI rpcinfo.RPCInfo
	ri := genRPCInfo()
	_, err = rc.WithRetryIfNeeded(context.Background(), func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			//retryCtx, cRI = kc.initRPCInfo(ctx, method)
			retryCtx = metainfo.WithPersistentValue(retryCtx, TransitKey, strconv.Itoa(callTimes-1))
			if prevRI == nil {
				prevRI = ri
			}
			r.Prepare(retryCtx, prevRI, cRI)
			prevRI = cRI
		}
		return cRI, nil
	}, ri, nil)
	test.Assert(t, err == nil, err)
}

func TestMakeFailurePolicy(t *testing.T) {
	rc := NewRetryContainer()
	failurePolicy := &FailurePolicy{
		StopPolicy: StopPolicy{
			MaxRetryTimes:    2,
			DisableChainStop: false,
			CBPolicy: CBPolicy{
				ErrorRate: defaultCBErrRate,
			},
		},
		BackOffPolicy: &BackOffPolicy{
			BackOffType: RandomBackOffType,
			CfgItems: map[BackOffCfgKey]float64{
				MinMSBackOffCfgKey: 100.0,
				MaxMSBackOffCfgKey: 200.0,
			},
		},
	}
	err := rc.Init(&Policy{
		Enable:        true,
		Type:          0,
		FailurePolicy: failurePolicy,
	})
	test.Assert(t, err == nil, err)

	rc1 := NewRetryContainer()
	failurePolicy1 := &FailurePolicy{
		StopPolicy: StopPolicy{
			MaxRetryTimes:    2,
			DisableChainStop: false,
			CBPolicy: CBPolicy{
				ErrorRate: defaultCBErrRate,
			},
		},
		BackOffPolicy: &BackOffPolicy{
			BackOffType: FixedBackOffType,
			CfgItems: map[BackOffCfgKey]float64{
				FixMSBackOffCfgKey: 100.0,
			},
		},
	}
	err1 := rc1.Init(&Policy{
		Enable:        true,
		Type:          0,
		FailurePolicy: failurePolicy1,
	})
	test.Assert(t, err1 == nil, err)
}

func TestFailurePolicyCall2(t *testing.T) {
	rc := NewRetryContainer()
	failurePolicy := NewFailurePolicy()
	failurePolicy.BackOffPolicy.BackOffType = FixedBackOffType
	failurePolicy.BackOffPolicy.CfgItems = map[BackOffCfgKey]float64{
		FixMSBackOffCfgKey: 100.0,
	}
	err := rc.Init(&Policy{
		Enable:        true,
		Type:          0,
		FailurePolicy: failurePolicy,
	})
	test.Assert(t, err == nil, err)

	callTimes := 0
	var prevRI rpcinfo.RPCInfo
	ri := genRPCInfo()
	_, err = rc.WithRetryIfNeeded(context.Background(), func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			//retryCtx, cRI = kc.initRPCInfo(ctx, method)
			retryCtx = metainfo.WithPersistentValue(retryCtx, TransitKey, strconv.Itoa(callTimes-1))
			if prevRI == nil {
				prevRI = ri
			}
			r.Prepare(retryCtx, prevRI, cRI)
			prevRI = cRI
		}
		return cRI, nil
	}, ri, nil)
	test.Assert(t, err == nil, err)
}

func TestBackupPolicyCall(t *testing.T) {
	rc := NewRetryContainer()
	err := rc.Init(&Policy{
		Enable:       true,
		Type:         1,
		BackupPolicy: NewBackupPolicy(20),
	})
	test.Assert(t, err == nil, err)

	callTimes := 0
	var prevRI rpcinfo.RPCInfo
	var ri rpcinfo.RPCInfo = nil
	fromPoint := rpcinfo.NewEndpointInfo("inServerName", "inMethod", nil, nil)
	endPoint := rpcinfo.NewEndpointInfo("outServerName", "outMethod", nil, nil)
	cfg := rpcinfo.AsMutableRPCConfig(rpcinfo.NewRPCConfig())
	rpcStats := rpcinfo.AsMutableRPCStats(rpcinfo.NewRPCStats())
	ri = rpcinfo.NewRPCInfo(
		fromPoint,
		endPoint,
		rpcinfo.NewInvocation("invocationService", "invocationMethod", "invocationPackageName"),
		cfg.ImmutableView(),
		rpcStats.ImmutableView(),
	)
	_, err = rc.WithRetryIfNeeded(context.Background(), func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			//retryCtx, cRI = kc.initRPCInfo(ctx, method)
			retryCtx = metainfo.WithPersistentValue(retryCtx, TransitKey, strconv.Itoa(callTimes-1))
			if prevRI == nil {
				prevRI = ri
			}
			r.Prepare(retryCtx, prevRI, cRI)
			prevRI = cRI
		}
		return cRI, nil
	}, ri, nil)
	test.Assert(t, err == nil, err)
}
