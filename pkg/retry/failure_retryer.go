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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func newFailureRetryer(policy Policy, cbC *cbContainer, logger klog.FormatLogger) (Retryer, error) {
	fr := &failureRetryer{cbContainer: cbC, logger: logger}
	if err := fr.UpdatePolicy(policy); err != nil {
		return nil, fmt.Errorf("newfailureRetryer failed, err=%w", err)
	}
	return fr, nil
}

type failureRetryer struct {
	enable      bool
	policy      *FailurePolicy
	backOff     BackOff
	cbContainer *cbContainer
	logger      klog.FormatLogger
	sync.RWMutex
	errMsg string
}

// ShouldRetry implements the Retryer interface.
func (r *failureRetryer) ShouldRetry(ctx context.Context, err error, callTimes int, request interface{}, cbKey string) (string, bool) {
	r.RLock()
	defer r.RUnlock()
	if !r.enable || !r.isRetryErr(err) {
		return "", false
	}
	if stop, msg := circuitBreakerStop(ctx, r.policy.StopPolicy, r.cbContainer, request, cbKey); stop {
		return msg, false
	}
	if stop, msg := ddlStop(ctx, r.policy.StopPolicy, r.logger); stop {
		return msg, false
	}
	r.backOff.Wait(callTimes)
	return "", true
}

// AllowRetry implements the Retryer interface.
func (r *failureRetryer) AllowRetry(ctx context.Context) (string, bool) {
	r.RLock()
	defer r.RUnlock()
	if !r.enable || r.policy.StopPolicy.MaxRetryTimes == 0 {
		return "", false
	}
	if stop, msg := chainStop(ctx, r.policy.StopPolicy); stop {
		return msg, false
	}
	if stop, msg := ddlStop(ctx, r.policy.StopPolicy, r.logger); stop {
		return msg, false
	}
	return "", true
}

// Do implements the Retryer interface.
func (r *failureRetryer) Do(ctx context.Context, rpcCall RPCCallFunc, firstRI rpcinfo.RPCInfo, request interface{}) (recycleRI bool, err error) {
	r.RLock()
	var maxDuration time.Duration
	if r.policy.StopPolicy.MaxDurationMS > 0 {
		maxDuration = time.Duration(r.policy.StopPolicy.MaxDurationMS) * time.Millisecond
	}
	retryTimes := r.policy.StopPolicy.MaxRetryTimes
	r.RUnlock()

	var callTimes int32
	var callCosts strings.Builder
	var cRI rpcinfo.RPCInfo
	cbKey, _ := r.cbContainer.cbCtl.GetKey(ctx, request)
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = panicToErr(ctx, panicInfo, firstRI, r.logger)
		}
	}()
	startTime := time.Now()
	for i := 0; i <= retryTimes; i++ {
		var callStart time.Time
		if i == 0 {
			callStart = startTime
		} else if i > 0 {
			if maxDuration > 0 && time.Since(startTime) > maxDuration {
				err = makeRetryErr(ctx, "exceed max duration", callTimes)
				break
			}
			if msg, ok := r.ShouldRetry(ctx, err, i, request, cbKey); !ok {
				if msg != "" {
					appendMsg := fmt.Sprintf("retried %d, %s", i-1, msg)
					appendErrMsg(err, appendMsg)
				}
				break
			}
			callStart = time.Now()
			callCosts.WriteByte(',')
			if respOp, ok := ctx.Value(CtxRespOp).(*int32); ok {
				atomic.StoreInt32(respOp, OpNo)
			}
		}
		callTimes++
		cRI, err = rpcCall(ctx, r)
		callCosts.WriteString(strconv.FormatInt(time.Since(callStart).Microseconds(), 10))

		if r.cbContainer.cbStat {
			circuitbreak.RecordStat(ctx, request, nil, err, cbKey, r.cbContainer.cbCtl, r.cbContainer.cbPanel)
		}
		if err == nil {
			if i > 0 {
				// monitor report just use firstRI
				rpcinfo.PutRPCInfo(cRI)
			}
			break
		} else {
			if i == retryTimes {
				// retry error
				err = kerrors.ErrRetry.WithCause(err)
			}
		}
	}
	recordRetryInfo(firstRI, callTimes, callCosts.String())
	if err == nil && callTimes == 1 {
		return true, nil
	}
	return false, err
}

// UpdatePolicy implements the Retryer interface.
func (r *failureRetryer) UpdatePolicy(rp Policy) (err error) {
	if !rp.Enable {
		r.Lock()
		r.enable = rp.Enable
		r.Unlock()
		return nil
	}
	var errMsg string
	if rp.FailurePolicy == nil || rp.Type != FailureType {
		errMsg = "FailurePolicy is nil or retry type not match, cannot do update in failureRetryer"
		err = errors.New(errMsg)
	}
	rt := rp.FailurePolicy.StopPolicy.MaxRetryTimes
	if errMsg == "" && (rt < 0 || rt > maxFailureRetryTimes) {
		errMsg = fmt.Sprintf("invalid failure MaxRetryTimes[%d]", rt)
		err = errors.New(errMsg)
	}
	if errMsg == "" {
		if e := checkCBErrorRate(&rp.FailurePolicy.StopPolicy.CBPolicy); e != nil {
			rp.FailurePolicy.StopPolicy.CBPolicy.ErrorRate = defaultCBErrRate
			errMsg = fmt.Sprintf("failureRetryer %s, use default %0.2f", e.Error(), defaultCBErrRate)
			r.logger.Warnf(errMsg)
		}
	}
	r.Lock()
	defer r.Unlock()
	r.enable = rp.Enable
	if err != nil {
		r.errMsg = errMsg
		return err
	}
	r.policy = rp.FailurePolicy
	if bo, e := initBackOff(rp.FailurePolicy.BackOffPolicy); e != nil {
		r.errMsg = fmt.Sprintf("failureRetryer update BackOffPolicy failed, err=%s", e.Error())
		r.logger.Warnf(r.errMsg)
	} else {
		r.backOff = bo
	}
	return nil
}

// AppendErrMsgIfNeeded implements the Retryer interface.
func (r *failureRetryer) AppendErrMsgIfNeeded(err error, msg string) {
	if r.isRetryErr(err) {
		// Add additional reason when retry is not applied.
		appendErrMsg(err, msg)
	}
}

// Prepare implements the Retryer interface.
func (r *failureRetryer) Prepare(ctx context.Context, prevRI, retryRI rpcinfo.RPCInfo) {
	handleRetryInstance(r.policy.RetrySameNode, prevRI, retryRI)
}

// Dump implements the Retryer interface.
func (r *failureRetryer) Dump() map[string]interface{} {
	r.RLock()
	defer r.RUnlock()
	if r.errMsg != "" {
		return map[string]interface{}{
			"enable":       r.enable,
			"failureRetry": r.policy,
			"errMsg":       r.errMsg,
		}
	}
	return map[string]interface{}{"enable": r.enable, "failureRetry": r.policy}
}

func (r *failureRetryer) isRetryErr(err error) bool {
	// Consider timeout error only.
	return kerrors.IsTimeoutError(err)
}

func initBackOff(policy *BackOffPolicy) (bo BackOff, err error) {
	bo = NoneBackOff
	if policy == nil {
		return
	}
	switch policy.BackOffType {
	case NoneBackOffType:
	case FixedBackOffType:
		if policy.CfgItems == nil {
			return bo, errors.New("invalid FixedBackOff, CfgItems is nil")
		}
		fixMS := policy.CfgItems[FixMSBackOffCfgKey]
		fixMSInt, _ := strconv.Atoi(fmt.Sprintf("%1.0f", fixMS))
		if err = checkFixedBackOff(fixMSInt); err != nil {
			return
		}
		bo = newFixedBackOff(fixMSInt)
	case RandomBackOffType:
		if policy.CfgItems == nil {
			return bo, errors.New("invalid FixedBackOff, CfgItems is nil")
		}
		minMS := policy.CfgItems[MinMSBackOffCfgKey]
		maxMS := policy.CfgItems[MaxMSBackOffCfgKey]
		minMSInt, _ := strconv.Atoi(fmt.Sprintf("%1.0f", minMS))
		maxMSInt, _ := strconv.Atoi(fmt.Sprintf("%1.0f", maxMS))
		if err = checkRandomBackOff(minMSInt, maxMSInt); err != nil {
			return
		}
		bo = newRandomBackOff(minMSInt, maxMSInt)
	default:
		return bo, fmt.Errorf("invalid backoffType=%v", policy.BackOffType)
	}
	return
}

// Type implements the Retryer interface.
func (r *failureRetryer) Type() Type {
	return FailureType
}
