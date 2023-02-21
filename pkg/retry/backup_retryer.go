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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func newBackupRetryer(policy Policy, cbC *cbContainer) (Retryer, error) {
	br := &backupRetryer{cbContainer: cbC}
	if err := br.UpdatePolicy(policy); err != nil {
		return nil, fmt.Errorf("newBackupRetryer failed, err=%w", err)
	}
	return br, nil
}

type backupRetryer struct {
	enable      bool
	policy      *BackupPolicy
	cbContainer *cbContainer
	retryDelay  time.Duration
	sync.RWMutex
	errMsg string
}

type resultWrapper struct {
	ri  rpcinfo.RPCInfo
	err error
}

// ShouldRetry implements the Retryer interface.
func (r *backupRetryer) ShouldRetry(ctx context.Context, err error, callTimes int, req interface{}, cbKey string) (string, bool) {
	r.RLock()
	defer r.RUnlock()
	if !r.enable {
		return "", false
	}
	if stop, msg := circuitBreakerStop(ctx, r.policy.StopPolicy, r.cbContainer, req, cbKey); stop {
		return msg, false
	}
	return "", true
}

// AllowRetry implements the Retryer interface.
func (r *backupRetryer) AllowRetry(ctx context.Context) (string, bool) {
	r.RLock()
	defer r.RUnlock()
	if !r.enable || r.policy.StopPolicy.MaxRetryTimes == 0 {
		return "", false
	}
	if stop, msg := chainStop(ctx, r.policy.StopPolicy); stop {
		return msg, false
	}
	return "", true
}

// Do implement the Retryer interface.
func (r *backupRetryer) Do(ctx context.Context, rpcCall RPCCallFunc, firstRI rpcinfo.RPCInfo, req interface{}) (recycleRI bool, err error) {
	r.RLock()
	retryTimes := r.policy.StopPolicy.MaxRetryTimes
	retryDelay := r.retryDelay
	r.RUnlock()
	var callTimes int32 = 0
	var callCosts strings.Builder
	callCosts.Grow(32)
	var recordCostDoing int32 = 0
	var abort int32 = 0
	// notice: buff num of chan is very important here, it cannot less than call times, or the below chan receive will block
	done := make(chan *resultWrapper, retryTimes+1)
	cbKey, _ := r.cbContainer.cbCtl.GetKey(ctx, req)
	timer := time.NewTimer(retryDelay)
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = panicToErr(ctx, panicInfo, firstRI)
		}
		timer.Stop()
	}()
	// include first call, max loop is retryTimes + 1
	doCall := true
	for i := 0; ; {
		if doCall {
			doCall = false
			i++
			gofunc.GoFunc(ctx, func() {
				if atomic.LoadInt32(&abort) == 1 {
					return
				}
				var (
					e   error
					cRI rpcinfo.RPCInfo
				)
				defer func() {
					if panicInfo := recover(); panicInfo != nil {
						e = panicToErr(ctx, panicInfo, firstRI)
					}
					done <- &resultWrapper{cRI, e}
				}()
				ct := atomic.AddInt32(&callTimes, 1)
				callStart := time.Now()
				cRI, _, e = rpcCall(ctx, r)
				recordCost(ct, callStart, &recordCostDoing, &callCosts, &abort, e)
				if r.cbContainer.cbStat {
					circuitbreak.RecordStat(ctx, req, nil, e, cbKey, r.cbContainer.cbCtl, r.cbContainer.cbPanel)
				}
			})
		}
		select {
		case <-timer.C:
			if _, ok := r.ShouldRetry(ctx, nil, i, req, cbKey); ok && i <= retryTimes {
				doCall = true
				timer.Reset(retryDelay)
			}
		case res := <-done:
			if res.err != nil && errors.Is(res.err, kerrors.ErrRPCFinish) {
				// To ignore resp concurrent write, the later response won't do decode and return ErrRPCFinish.
				// But if the cost of decode is long, ErrRPCFinish will return before previous normal call.
				continue
			}
			atomic.StoreInt32(&abort, 1)
			recordRetryInfo(firstRI, res.ri, atomic.LoadInt32(&callTimes), callCosts.String())
			return false, res.err
		}
	}
}

// Prepare implements the Retryer interface.
func (r *backupRetryer) Prepare(ctx context.Context, prevRI, retryRI rpcinfo.RPCInfo) {
	handleRetryInstance(r.policy.RetrySameNode, prevRI, retryRI)
}

// UpdatePolicy implements the Retryer interface.
func (r *backupRetryer) UpdatePolicy(rp Policy) (err error) {
	if !rp.Enable {
		r.Lock()
		r.enable = rp.Enable
		r.Unlock()
		return nil
	}
	var errMsg string
	if rp.BackupPolicy == nil || rp.Type != BackupType {
		errMsg = "BackupPolicy is nil or retry type not match, cannot do update in backupRetryer"
		err = errors.New(errMsg)
	}
	if errMsg == "" && (rp.BackupPolicy.RetryDelayMS == 0 || rp.BackupPolicy.StopPolicy.MaxRetryTimes < 0 ||
		rp.BackupPolicy.StopPolicy.MaxRetryTimes > maxBackupRetryTimes) {
		errMsg = "invalid backup request delay duration or retryTimes"
		err = errors.New(errMsg)
	}
	if errMsg == "" {
		if e := checkCBErrorRate(&rp.BackupPolicy.StopPolicy.CBPolicy); e != nil {
			rp.BackupPolicy.StopPolicy.CBPolicy.ErrorRate = defaultCBErrRate
			errMsg = fmt.Sprintf("backupRetryer %s, use default %0.2f", e.Error(), defaultCBErrRate)
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
	r.policy = rp.BackupPolicy
	r.retryDelay = time.Duration(rp.BackupPolicy.RetryDelayMS) * time.Millisecond
	return nil
}

// AppendErrMsgIfNeeded implements the Retryer interface.
func (r *backupRetryer) AppendErrMsgIfNeeded(err error, ri rpcinfo.RPCInfo, msg string) {
	if kerrors.IsTimeoutError(err) {
		// Add additional reason to the error message when timeout occurs but the backup request is not sent.
		appendErrMsg(err, msg)
	}
}

// Dump implements the Retryer interface.
func (r *backupRetryer) Dump() map[string]interface{} {
	r.RLock()
	defer r.RUnlock()
	if r.errMsg != "" {
		return map[string]interface{}{
			"enable":        r.enable,
			"backupRequest": r.policy,
			"errMsg":        r.errMsg,
		}
	}
	return map[string]interface{}{"enable": r.enable, "backupRequest": r.policy}
}

// Type implements the Retryer interface.
func (r *backupRetryer) Type() Type {
	return BackupType
}

// record request cost, it may execute concurrent
func recordCost(ct int32, start time.Time, recordCostDoing *int32, sb *strings.Builder, abort *int32, err error) {
	if atomic.LoadInt32(abort) == 1 {
		return
	}
	for !atomic.CompareAndSwapInt32(recordCostDoing, 0, 1) {
		runtime.Gosched()
	}
	if sb.Len() > 0 {
		sb.WriteByte(',')
	}
	sb.WriteString(strconv.Itoa(int(ct)))
	sb.WriteByte('-')
	sb.WriteString(strconv.FormatInt(time.Since(start).Microseconds(), 10))
	if err != nil && errors.Is(err, kerrors.ErrRPCFinish) {
		// ErrRPCFinish means previous call returns first but is decoding.
		// Add ignore to distinguish.
		sb.WriteString("(ignore)")
	}
	atomic.StoreInt32(recordCostDoing, 0)
}
