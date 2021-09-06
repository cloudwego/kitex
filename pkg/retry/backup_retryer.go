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

	"github.com/jackedelic/kitex/internal/pkg/circuitbreak"
	"github.com/jackedelic/kitex/internal/pkg/gofunc"
	"github.com/jackedelic/kitex/internal/pkg/kerrors"
	"github.com/jackedelic/kitex/internal/pkg/klog"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
)

func newBackupRetryer(policy Policy, cbC *cbContainer, logger klog.FormatLogger) (Retryer, error) {
	br := &backupRetryer{cbContainer: cbC, logger: logger}
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
	logger      klog.FormatLogger
	sync.RWMutex
	errMsg string
}

// ShouldRetry implements the Retryer interface.
func (r *backupRetryer) ShouldRetry(ctx context.Context, err error, callTimes int, request interface{}, cbKey string) (string, bool) {
	r.RLock()
	defer r.RUnlock()
	if !r.enable {
		return "", false
	}
	if stop, msg := circuitBreakerStop(ctx, r.policy.StopPolicy, r.cbContainer, request, cbKey); stop {
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

// Do implements the Retryer interface.
func (r *backupRetryer) Do(ctx context.Context, rpcCall RPCCallFunc, firstRI rpcinfo.RPCInfo, request interface{}) (recycleRI bool, err error) {
	r.RLock()
	retryTimes := r.policy.StopPolicy.MaxRetryTimes
	retryDelay := r.retryDelay
	r.RUnlock()
	var callTimes int32 = 0
	var callCosts strings.Builder
	callCosts.Grow(16)
	var recordCostDoing int32 = 0
	var abort int32 = 0
	// notice: buff num of chan is very important here, it cannot less than call times, or the below chan receive will block
	done := make(chan error, retryTimes+1)
	cbKey, _ := r.cbContainer.cbCtl.GetKey(ctx, request)
	timer := time.NewTimer(retryDelay)
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = panicToErr(ctx, panicInfo, firstRI, r.logger)
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
				var e error
				defer func() {
					if panicInfo := recover(); panicInfo != nil {
						e = panicToErr(ctx, panicInfo, firstRI, r.logger)
					}
					done <- e
				}()
				ct := atomic.AddInt32(&callTimes, 1)
				callStart := time.Now()
				_, e = rpcCall(ctx, r)
				recordCost(ct, callStart, &recordCostDoing, &callCosts)
				if r.cbContainer.cbStat {
					circuitbreak.RecordStat(ctx, request, nil, e, cbKey, r.cbContainer.cbCtl, r.cbContainer.cbPanel)
				}
			})
		}
		select {
		case <-timer.C:
			if _, ok := r.ShouldRetry(ctx, nil, i, request, cbKey); ok && i <= retryTimes {
				doCall = true
				timer.Reset(retryDelay)
			}
		case e := <-done:
			if e != nil && errors.Is(e, kerrors.ErrRPCFinish) {
				// To ignore resp concurrent write, the later response won't do decode and return ErrRPCFinish.
				// But if the cost of decode is long, ErrRPCFinish will return before previous normal call.
				continue
			}
			atomic.StoreInt32(&abort, 1)
			recordRetryInfo(firstRI, atomic.LoadInt32(&callTimes), callCosts.String())
			return false, e
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
	r.policy = rp.BackupPolicy
	r.retryDelay = time.Duration(rp.BackupPolicy.RetryDelayMS) * time.Millisecond
	return nil
}

// AppendErrMsgIfNeeded implements the Retryer interface.
func (r *backupRetryer) AppendErrMsgIfNeeded(err error, msg string) {
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
func recordCost(ct int32, start time.Time, costRecordDoing *int32, sb *strings.Builder) {
	for !atomic.CompareAndSwapInt32(costRecordDoing, 0, 1) {
		runtime.Gosched()
	}
	if sb.Len() > 0 {
		sb.WriteByte(',')
	}
	sb.WriteString(strconv.Itoa(int(ct)))
	sb.WriteByte('-')
	sb.WriteString(strconv.FormatInt(time.Since(start).Microseconds(), 10))
	atomic.StoreInt32(costRecordDoing, 0)
}
