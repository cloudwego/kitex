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
	"fmt"
	"time"

	"github.com/bytedance/gopkg/lang/fastrand"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

const maxFailureRetryTimes = 5

// NewFailurePolicy init default failure retry policy
func NewFailurePolicy() *FailurePolicy {
	p := &FailurePolicy{
		StopPolicy: StopPolicy{
			MaxRetryTimes:    2,
			DisableChainStop: false,
			CBPolicy: CBPolicy{
				ErrorRate: defaultCBErrRate,
			},
		},
		BackOffPolicy: &BackOffPolicy{BackOffType: NoneBackOffType},
	}
	return p
}

// NewFailurePolicyWithResultRetry init failure retry policy with ShouldResultRetry
func NewFailurePolicyWithResultRetry(rr *ShouldResultRetry) *FailurePolicy {
	fp := NewFailurePolicy()
	fp.ShouldResultRetry = rr
	return fp
}

// WithMaxRetryTimes default is 2, not include first call
func (p *FailurePolicy) WithMaxRetryTimes(retryTimes int) {
	if retryTimes < 0 || retryTimes > maxFailureRetryTimes {
		panic(fmt.Errorf("maxFailureRetryTimes is %d", maxFailureRetryTimes))
	}
	p.StopPolicy.MaxRetryTimes = retryTimes
}

// WithMaxDurationMS include first call and retry call, if the total duration exceed limit then stop retry
func (p *FailurePolicy) WithMaxDurationMS(maxMS uint32) {
	p.StopPolicy.MaxDurationMS = maxMS
}

// DisableChainRetryStop disables chain stop
func (p *FailurePolicy) DisableChainRetryStop() {
	p.StopPolicy.DisableChainStop = true
}

// WithDDLStop sets ddl stop to true.
func (p *FailurePolicy) WithDDLStop() {
	p.StopPolicy.DDLStop = true
}

// WithFixedBackOff set fixed time.
func (p *FailurePolicy) WithFixedBackOff(fixMS int) {
	if err := checkFixedBackOff(fixMS); err != nil {
		panic(err)
	}
	p.BackOffPolicy = &BackOffPolicy{FixedBackOffType, make(map[BackOffCfgKey]float64, 1)}
	p.BackOffPolicy.CfgItems[FixMSBackOffCfgKey] = float64(fixMS)
}

// WithRandomBackOff sets the random time.
func (p *FailurePolicy) WithRandomBackOff(minMS, maxMS int) {
	if err := checkRandomBackOff(minMS, maxMS); err != nil {
		panic(err)
	}
	p.BackOffPolicy = &BackOffPolicy{RandomBackOffType, make(map[BackOffCfgKey]float64, 2)}
	p.BackOffPolicy.CfgItems[MinMSBackOffCfgKey] = float64(minMS)
	p.BackOffPolicy.CfgItems[MaxMSBackOffCfgKey] = float64(maxMS)
}

// WithRetryBreaker sets the error rate.
func (p *FailurePolicy) WithRetryBreaker(errRate float64) {
	p.StopPolicy.CBPolicy.ErrorRate = errRate
	if err := checkCBErrorRate(&p.StopPolicy.CBPolicy); err != nil {
		panic(err)
	}
}

// WithRetrySameNode sets to retry the same node.
func (p *FailurePolicy) WithRetrySameNode() {
	p.RetrySameNode = true
}

// WithSpecifiedResultRetry sets customized ShouldResultRetry.
func (p *FailurePolicy) WithSpecifiedResultRetry(rr *ShouldResultRetry) {
	if rr != nil {
		p.ShouldResultRetry = rr
	}
}

// String prints human readable information.
func (p FailurePolicy) String() string {
	return fmt.Sprintf("{StopPolicy:%+v BackOffPolicy:%+v RetrySameNode:%+v "+
		"ShouldResultRetry:{ErrorRetry:%t, RespRetry:%t}}", p.StopPolicy, p.BackOffPolicy, p.RetrySameNode, p.IsErrorRetryNonNil(), p.IsRespRetryNonNil())
}

// AllErrorRetry is common choice for ShouldResultRetry.
func AllErrorRetry() *ShouldResultRetry {
	return &ShouldResultRetry{ErrorRetry: func(err error, ri rpcinfo.RPCInfo) bool {
		return err != nil
	}}
}

// BackOff is the interface of back off implements
type BackOff interface {
	Wait(callTimes int)
}

// NoneBackOff means there's no backoff.
var NoneBackOff = &noneBackOff{}

func newFixedBackOff(fixMS int) BackOff {
	return &fixedBackOff{
		fixMS: fixMS,
	}
}

func newRandomBackOff(minMS, maxMS int) BackOff {
	return &randomBackOff{
		minMS: minMS,
		maxMS: maxMS,
	}
}

type noneBackOff struct{}

// String prints human readable information.
func (p noneBackOff) String() string {
	return "NoneBackOff"
}

// Wait implements the BackOff interface.
func (p *noneBackOff) Wait(callTimes int) {
}

type fixedBackOff struct {
	fixMS int
}

// Wait implements the BackOff interface.
func (p *fixedBackOff) Wait(callTimes int) {
	time.Sleep(time.Duration(p.fixMS) * time.Millisecond)
}

// String prints human readable information.
func (p fixedBackOff) String() string {
	return fmt.Sprintf("FixedBackOff(%dms)", p.fixMS)
}

type randomBackOff struct {
	minMS int
	maxMS int
}

func (p *randomBackOff) randomDuration() time.Duration {
	return time.Duration(p.minMS+fastrand.Intn(p.maxMS-p.minMS)) * time.Millisecond
}

// Wait implements the BackOff interface.
func (p *randomBackOff) Wait(callTimes int) {
	time.Sleep(p.randomDuration())
}

// String prints human readable information.
func (p randomBackOff) String() string {
	return fmt.Sprintf("RandomBackOff(%dms-%dms)", p.minMS, p.maxMS)
}

func checkFixedBackOff(fixMS int) error {
	if fixMS == 0 {
		return fmt.Errorf("invalid FixedBackOff, fixMS=%d", fixMS)
	}
	return nil
}

func checkRandomBackOff(minMS, maxMS int) error {
	if maxMS <= minMS {
		return fmt.Errorf("invalid RandomBackOff, minMS=%d, maxMS=%d", minMS, maxMS)
	}
	return nil
}
