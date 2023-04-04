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

func newFailureRetryer(policy Policy, r *ShouldResultRetry, cbC *cbContainer) (Retryer, error) {
	fr := &failureRetryer{specifiedResultRetry: r, cbContainer: cbC}
	if err := fr.UpdatePolicy(policy); err != nil {
		return nil, fmt.Errorf("newfailureRetryer failed, err=%w", err)
	}
	return fr, nil
}

type failureRetryer struct {
	enable               bool
	policy               *FailurePolicy
	backOff              BackOff
	cbContainer          *cbContainer
	specifiedResultRetry *ShouldResultRetry
	sync.RWMutex
	errMsg string
}

// ShouldRetry implements the Retryer interface.
func (r *failureRetryer) ShouldRetry(ctx context.Context, err error, callTimes int, req interface{}, cbKey string) (string, bool) {
	r.RLock()
	defer r.RUnlock()
	if !r.enable {
		return "", false
	}
	if stop, msg := circuitBreakerStop(ctx, r.policy.StopPolicy, r.cbContainer, req, cbKey); stop {
		return msg, false
	}
	if stop, msg := ddlStop(ctx, r.policy.StopPolicy); stop {
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
	if stop, msg := ddlStop(ctx, r.policy.StopPolicy); stop {
		return msg, false
	}
	return "", true
}

// Do implement the Retryer interface.
func (r *failureRetryer) Do(ctx context.Context, rpcCall RPCCallFunc, firstRI rpcinfo.RPCInfo, req interface{}) (recycleRI bool, err error) {
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
	cbKey, _ := r.cbContainer.cbCtl.GetKey(ctx, req)
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = panicToErr(ctx, panicInfo, firstRI)
		}
	}()
	startTime := time.Now()
	for i := 0; i <= retryTimes; i++ {
		var resp interface{}
		var callStart time.Time
		if i == 0 {
			callStart = startTime
		} else if i > 0 {
			if maxDuration > 0 && time.Since(startTime) > maxDuration {
				err = makeRetryErr(ctx, "exceed max duration", callTimes)
				break
			}
			if msg, ok := r.ShouldRetry(ctx, err, i, req, cbKey); !ok {
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
		cRI, resp, err = rpcCall(ctx, r)
		callCosts.WriteString(strconv.FormatInt(time.Since(callStart).Microseconds(), 10))

		if r.cbContainer.cbStat {
			circuitbreak.RecordStat(ctx, req, nil, err, cbKey, r.cbContainer.cbCtl, r.cbContainer.cbPanel)
		}
		if err == nil {
			if r.policy.IsRespRetryNonNil() && r.policy.ShouldResultRetry.RespRetry(resp, cRI) {
				// user specified resp to do retry
				continue
			}
			break
		} else {
			if i == retryTimes {
				// stop retry then wrap error
				err = kerrors.ErrRetry.WithCause(err)
			} else if !r.isRetryErr(err, cRI) {
				// not timeout or user specified error won't do retry
				break
			}
		}
	}
	recordRetryInfo(firstRI, cRI, callTimes, callCosts.String())
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
			klog.Warnf(errMsg)
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
	r.setSpecifiedResultRetryIfNeeded(r.specifiedResultRetry)
	if bo, e := initBackOff(rp.FailurePolicy.BackOffPolicy); e != nil {
		r.errMsg = fmt.Sprintf("failureRetryer update BackOffPolicy failed, err=%s", e.Error())
		klog.Warnf(r.errMsg)
	} else {
		r.backOff = bo
	}
	return nil
}

// AppendErrMsgIfNeeded implements the Retryer interface.
func (r *failureRetryer) AppendErrMsgIfNeeded(err error, ri rpcinfo.RPCInfo, msg string) {
	if r.isRetryErr(err, ri) {
		// Add additional reason when retry is not applied.
		appendErrMsg(err, msg)
	}
}

// Prepare implements the Retryer interface.
func (r *failureRetryer) Prepare(ctx context.Context, prevRI, retryRI rpcinfo.RPCInfo) {
	handleRetryInstance(r.policy.RetrySameNode, prevRI, retryRI)
}

func (r *failureRetryer) isRetryErr(err error, ri rpcinfo.RPCInfo) bool {
	if err == nil {
		return false
	}
	// Logic Notice:
	// some kinds of error cannot be retried, eg: ServiceCircuitBreak.
	// But CircuitBreak has been checked in ShouldRetry, it doesn't need to filter ServiceCircuitBreak.
	// If there are some other specified errors that cannot be retried, it should be filtered here.

	if r.policy.IsRetryForTimeout() && kerrors.IsTimeoutError(err) {
		return true
	}
	if r.policy.IsErrorRetryNonNil() && r.policy.ShouldResultRetry.ErrorRetry(err, ri) {
		return true
	}
	return false
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

// Dump implements the Retryer interface.
func (r *failureRetryer) Dump() map[string]interface{} {
	r.RLock()
	defer r.RUnlock()
	dm := make(map[string]interface{})
	dm["enable"] = r.enable
	dm["failure_retry"] = r.policy
	dm["specified_result_retry"] = map[string]bool{
		"error_retry": r.policy.IsErrorRetryNonNil(),
		"resp_retry":  r.policy.IsRespRetryNonNil(),
	}
	if r.errMsg != "" {
		dm["errMsg"] = r.errMsg
	}
	return dm
}

func (r *failureRetryer) setSpecifiedResultRetryIfNeeded(rr *ShouldResultRetry) {
	if rr != nil {
		r.specifiedResultRetry = rr
	}
	if r.policy != nil && r.policy.ShouldResultRetry == nil {
		// The priority of FailurePolicy.ShouldResultRetry is higher, so only update it when it's nil
		r.policy.ShouldResultRetry = r.specifiedResultRetry
	}
}
