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
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
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

	// backPolicy is nil
	rc = NewRetryContainer()
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   1,
	})
	msg := "new retryer[test-Backup] failed, err=newBackupRetryer failed, err=BackupPolicy is nil or retry type not match, cannot do update in backupRetryer, at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// backPolicy config invalid
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   1,
		BackupPolicy: &BackupPolicy{
			RetryDelayMS: 0,
		},
	})
	msg = "new retryer[test-Backup] failed, err=newBackupRetryer failed, err=invalid backup request delay duration or retryTimes, at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// backPolicy cBPolicy config invalid
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   1,
		BackupPolicy: &BackupPolicy{
			RetryDelayMS: 100,
			StopPolicy: StopPolicy{
				CBPolicy: CBPolicy{
					ErrorRate: 0.4,
				},
			},
		},
	})
	msg = "new retryer[test-Backup] at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// failurePolicy config invalid
	rc = NewRetryContainer()
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   0,
		FailurePolicy: &FailurePolicy{
			StopPolicy: StopPolicy{
				MaxRetryTimes: 6,
			},
		},
	})
	msg = "new retryer[test-Failure] failed, err=newfailureRetryer failed, err=invalid failure MaxRetryTimes[6], at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// failurePolicy cBPolicy config invalid
	rc = NewRetryContainer()
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   0,
		FailurePolicy: &FailurePolicy{
			StopPolicy: StopPolicy{
				MaxRetryTimes: 5,
				CBPolicy: CBPolicy{
					ErrorRate: 0.4,
				},
			},
		},
	})
	msg = "new retryer[test-Failure] at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// failurePolicy backOffPolicy fixedBackOffType cfg is nil
	rc = NewRetryContainerWithCB(nil, nil)
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   0,
		FailurePolicy: &FailurePolicy{
			StopPolicy: StopPolicy{
				MaxRetryTimes: 5,
			},
			BackOffPolicy: &BackOffPolicy{
				BackOffType: FixedBackOffType,
			},
		},
	})
	msg = "new retryer[test-Failure] at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// failurePolicy backOffPolicy fixedBackOffType cfg invalid
	rc = NewRetryContainerWithCB(nil, nil)
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   0,
		FailurePolicy: &FailurePolicy{
			StopPolicy: StopPolicy{
				MaxRetryTimes: 5,
			},
			BackOffPolicy: &BackOffPolicy{
				BackOffType: FixedBackOffType,
				CfgItems: map[BackOffCfgKey]float64{
					FixMSBackOffCfgKey: 0,
				},
			},
		},
	})
	msg = "new retryer[test-Failure] at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// failurePolicy backOffPolicy randomBackOffType cfg is nil
	rc = NewRetryContainerWithCB(nil, nil)
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   0,
		FailurePolicy: &FailurePolicy{
			StopPolicy: StopPolicy{
				MaxRetryTimes: 5,
			},
			BackOffPolicy: &BackOffPolicy{
				BackOffType: RandomBackOffType,
			},
		},
	})
	msg = "new retryer[test-Failure] at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// failurePolicy backOffPolicy randomBackOffType cfg invalid
	rc = NewRetryContainerWithCB(nil, nil)
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   0,
		FailurePolicy: &FailurePolicy{
			StopPolicy: StopPolicy{
				MaxRetryTimes: 5,
			},
			BackOffPolicy: &BackOffPolicy{
				BackOffType: RandomBackOffType,
				CfgItems: map[BackOffCfgKey]float64{
					MinMSBackOffCfgKey: 20,
					MaxMSBackOffCfgKey: 10,
				},
			},
		},
	})
	msg = "new retryer[test-Failure] at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// failurePolicy backOffPolicy randomBackOffType normal
	rc = NewRetryContainerWithCB(nil, nil)
	rc.NotifyPolicyChange(method, Policy{
		Enable: true,
		Type:   0,
		FailurePolicy: &FailurePolicy{
			StopPolicy: StopPolicy{
				MaxRetryTimes: 5,
			},
			BackOffPolicy: &BackOffPolicy{
				BackOffType: RandomBackOffType,
				CfgItems: map[BackOffCfgKey]float64{
					MinMSBackOffCfgKey: 10,
					MaxMSBackOffCfgKey: 20,
				},
			},
		},
	})
	msg = "new retryer[test-Failure] at "
	test.Assert(t, rc.msg[:len(msg)] == msg)

	// test init invalid case
	err := rc.Init(nil)
	test.Assert(t, err == nil, err)
	err = rc.Init(&Policy{
		Enable: true,
		Type:   1,
		BackupPolicy: &BackupPolicy{
			RetryDelayMS: 0,
		},
	})
	test.Assert(t, err != nil, err)
}

func TestContainer_Dump(t *testing.T) {
	// test backupPolicy dump
	rc := NewRetryContainerWithCB(nil, nil)
	methodPolicies := map[string]Policy{
		method: {
			Enable:       true,
			Type:         1,
			BackupPolicy: NewBackupPolicy(20),
		},
	}
	rc.InitWithPolicies(methodPolicies)
	err := rc.Init(&Policy{
		Enable:       true,
		Type:         1,
		BackupPolicy: NewBackupPolicy(20),
	})
	test.Assert(t, err == nil, err)
	rcDump, ok := rc.Dump().(map[string]interface{})
	test.Assert(t, ok)
	cliRetryerStr, err := jsoni.MarshalToString(rcDump["clientRetryer"])
	msg := "{\"backupRequest\":{\"retry_delay_ms\":20,\"stop_policy\":{\"max_retry_times\":1,\"max_duration_ms\":0,\"disable_chain_stop\":false,\"ddl_stop\":false,\"cb_policy\":{\"error_rate\":0.1}},\"retry_same_node\":false},\"enable\":true}"
	test.Assert(t, err == nil, err)
	test.Assert(t, cliRetryerStr == msg)
	testStr, err := jsoni.MarshalToString(rcDump["test"])
	test.Assert(t, err == nil, err)
	test.Assert(t, testStr == msg)

	// test failurePolicy dump
	rc = NewRetryContainerWithCB(nil, nil)
	methodPolicies = map[string]Policy{
		method: {
			Enable:        true,
			Type:          FailureType,
			FailurePolicy: NewFailurePolicy(),
		},
	}
	rc.InitWithPolicies(methodPolicies)
	err = rc.Init(&Policy{
		Enable:        true,
		Type:          FailureType,
		FailurePolicy: NewFailurePolicy(),
	})
	test.Assert(t, err == nil, err)
	rcDump, ok = rc.Dump().(map[string]interface{})
	test.Assert(t, ok)
	cliRetryerStr, err = jsoni.MarshalToString(rcDump["clientRetryer"])
	msg = "{\"enable\":true,\"failureRetry\":{\"stop_policy\":{\"max_retry_times\":2,\"max_duration_ms\":0,\"disable_chain_stop\":false,\"ddl_stop\":false,\"cb_policy\":{\"error_rate\":0.1}},\"backoff_policy\":{\"backoff_type\":\"none\"},\"retry_same_node\":false}}"
	test.Assert(t, err == nil, err)
	test.Assert(t, cliRetryerStr == msg)
	testStr, err = jsoni.MarshalToString(rcDump["test"])
	test.Assert(t, err == nil, err)
	test.Assert(t, testStr == msg)
}

func TestFailurePolicyCall(t *testing.T) {
	ctx := context.Background()
	rc := NewRetryContainer()
	failurePolicy := NewFailurePolicy()
	failurePolicy.BackOffPolicy.BackOffType = FixedBackOffType
	failurePolicy.BackOffPolicy.CfgItems = map[BackOffCfgKey]float64{
		FixMSBackOffCfgKey: 100.0,
	}
	failurePolicy.StopPolicy.MaxDurationMS = 100
	err := rc.Init(&Policy{
		Enable:        true,
		Type:          0,
		FailurePolicy: failurePolicy,
	})
	test.Assert(t, err == nil, err)
	callTimes := 0
	var prevRI rpcinfo.RPCInfo
	ri := genRPCInfo()
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ok, err := rc.WithRetryIfNeeded(ctx, func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			retryCtx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
			retryCtx = metainfo.WithPersistentValue(retryCtx, TransitKey, strconv.Itoa(callTimes-1))
			if prevRI == nil {
				prevRI = ri
			}
			r.Prepare(retryCtx, prevRI, cRI)
			prevRI = cRI
		}
		return cRI, kerrors.ErrRPCTimeout
	}, ri, nil)
	test.Assert(t, err != nil, err)
	test.Assert(t, !ok)

	failurePolicy.StopPolicy.MaxDurationMS = 0
	err = rc.Init(&Policy{
		Enable:        true,
		Type:          0,
		FailurePolicy: failurePolicy,
	})
	test.Assert(t, err == nil, err)
	callTimes = 0
	prevRI = nil
	ri = genRPCInfo()
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ok, err = rc.WithRetryIfNeeded(ctx, func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			retryCtx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
			retryCtx = metainfo.WithPersistentValue(retryCtx, TransitKey, strconv.Itoa(callTimes-1))
			if prevRI == nil {
				prevRI = ri
			}
			r.Prepare(retryCtx, prevRI, cRI)
			prevRI = cRI
		}
		return cRI, kerrors.ErrRPCTimeout
	}, ri, nil)
	test.Assert(t, err != nil, err)
	test.Assert(t, !ok)
}

func TestFailurePolicyBackOffCall(t *testing.T) {
	ctx := context.Background()
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
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ok, err := rc.WithRetryIfNeeded(ctx, func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			retryCtx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
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
	test.Assert(t, ok)
}

func TestBackupPolicyCall(t *testing.T) {
	ctx := context.Background()
	rc := NewRetryContainer()
	err := rc.Init(&Policy{
		Enable:       true,
		Type:         1,
		BackupPolicy: NewBackupPolicy(20),
	})
	test.Assert(t, err == nil, err)

	callTimes := int32(0)
	var prevRI rpcinfo.RPCInfo
	ri := genRPCInfo()
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ok, err := rc.WithRetryIfNeeded(ctx, func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		atomic.AddInt32(&callTimes, 1)
		retryCtx := ctx
		cRI := ri
		callTimesAtomic := atomic.LoadInt32(&callTimes)
		if callTimesAtomic > 1 {
			retryCtx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
			retryCtx = metainfo.WithPersistentValue(retryCtx, TransitKey, strconv.Itoa(int(callTimesAtomic-1)))
			if prevRI == nil {
				prevRI = ri
			}
			r.Prepare(retryCtx, prevRI, cRI)
			prevRI = cRI
		}
		time.Sleep(time.Millisecond * 100)
		return cRI, nil
	}, ri, nil)
	test.Assert(t, err == nil, err)
	test.Assert(t, !ok)
}

func TestPolicyInvalidCall(t *testing.T) {
	ctx := context.Background()
	rc := NewRetryContainer()

	// case 1(default): no retry policy
	// no retry policy, call success
	callTimes := 0
	var prevRI rpcinfo.RPCInfo
	ri := genRPCInfo()
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ok, err := rc.WithRetryIfNeeded(ctx, func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			retryCtx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
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
	test.Assert(t, ok)

	// no retry policy, call rpcTimeout
	callTimes = 0
	prevRI = nil
	ri = genRPCInfo()
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ok, err = rc.WithRetryIfNeeded(ctx, func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			retryCtx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
			retryCtx = metainfo.WithPersistentValue(retryCtx, TransitKey, strconv.Itoa(callTimes-1))
			if prevRI == nil {
				prevRI = ri
			}
			r.Prepare(retryCtx, prevRI, cRI)
			prevRI = cRI
		}
		return cRI, kerrors.ErrRPCTimeout
	}, ri, nil)
	test.Assert(t, kerrors.IsTimeoutError(err), err)
	test.Assert(t, !ok)

	// case 2: setup retry policy, but not satisfy retry condition eg: circuit, retry times == 0, chain stop, ddl
	// failurePolicy DDLStop rpcTimeOut
	failurePolicy := NewFailurePolicy()
	failurePolicy.WithDDLStop()
	RegisterDDLStop(func(ctx context.Context, policy StopPolicy) (bool, string) {
		return true, "TestDDLStop"
	})
	err = rc.Init(&Policy{
		Enable:        true,
		Type:          0,
		FailurePolicy: failurePolicy,
	})
	test.Assert(t, err == nil, err)
	callTimes = 0
	prevRI = nil
	ri = genRPCInfo()
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ok, err = rc.WithRetryIfNeeded(ctx, func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			retryCtx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
			retryCtx = metainfo.WithPersistentValue(retryCtx, TransitKey, strconv.Itoa(callTimes-1))
			if prevRI == nil {
				prevRI = ri
			}
			r.Prepare(retryCtx, prevRI, cRI)
			prevRI = cRI
		}
		return cRI, kerrors.ErrRPCTimeout
	}, ri, nil)
	test.Assert(t, kerrors.IsTimeoutError(err), err)
	test.Assert(t, !ok)

	// backupPolicy MaxRetryTimes = 0
	backupPolicy := NewBackupPolicy(20)
	backupPolicy.StopPolicy.MaxRetryTimes = 0
	err = rc.Init(&Policy{
		Enable:       true,
		Type:         1,
		BackupPolicy: backupPolicy,
	})
	test.Assert(t, err == nil, err)
	callTimes = 0
	prevRI = nil
	ri = genRPCInfo()
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ok, err = rc.WithRetryIfNeeded(ctx, func(ctx context.Context, r Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			retryCtx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
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
	test.Assert(t, ok)
}
