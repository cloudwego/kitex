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

// Package retry implements rpc retry
package retry

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/cloud/circuitbreaker"

	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// RPCCallFunc is the definition with wrap rpc call
type RPCCallFunc func(context.Context, Retryer) (rpcinfo rpcinfo.RPCInfo, resp interface{}, err error)

// Retryer is the interface for Retry implements
type Retryer interface {
	// AllowRetry to check if current request satisfy retry condition[eg: circuit, retry times == 0, chain stop, ddl].
	// If not satisfy won't execute Retryer.Do and return the reason message
	// Execute anyway for the first time regardless of able to retry.
	AllowRetry(ctx context.Context) (msg string, ok bool)

	// ShouldRetry to check if retry request can be called, it is checked in retryer.Do.
	// If not satisfy will return the reason message
	ShouldRetry(ctx context.Context, err error, callTimes int, req interface{}, cbKey string) (msg string, ok bool)
	UpdatePolicy(policy Policy) error

	// Retry policy execute func. recycleRI is to decide if the firstRI can be recycled.
	Do(ctx context.Context, rpcCall RPCCallFunc, firstRI rpcinfo.RPCInfo, request interface{}) (recycleRI bool, err error)
	AppendErrMsgIfNeeded(err error, ri rpcinfo.RPCInfo, msg string)

	// Prepare to do something needed before retry call.
	Prepare(ctx context.Context, prevRI, retryRI rpcinfo.RPCInfo)
	Dump() map[string]interface{}
	Type() Type
}

// NewRetryContainerWithCB build Container that doesn't do circuit breaker statistic but get statistic result.
// Which is used in case that circuit breaker is enable.
// eg:
//
//	   cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
//	   retryC := retry.NewRetryContainerWithCB(cbs.ServiceControl(), cbs.ServicePanel())
//		  var opts []client.Option
//		  opts = append(opts, client.WithRetryContainer(retryC))
//	   // enable service circuit breaker
//		  opts = append(opts, client.WithMiddleware(cbs.ServiceCBMW()))
func NewRetryContainerWithCB(cc *circuitbreak.Control, cp circuitbreaker.Panel) *Container {
	return &Container{
		cbContainer: &cbContainer{cbCtl: cc, cbPanel: cp}, retryerMap: sync.Map{},
	}
}

// NewRetryContainerWithCBStat build Container that need to do circuit breaker statistic.
// Which is used in case that the service CB key is customized.
// eg:
//
//	cbs := circuitbreak.NewCBSuite(YourGenServiceCBKeyFunc)
//	retry.NewRetryContainerWithCBStat(cbs.ServiceControl(), cbs.ServicePanel())
func NewRetryContainerWithCBStat(cc *circuitbreak.Control, cp circuitbreaker.Panel) *Container {
	return &Container{
		cbContainer: &cbContainer{cbCtl: cc, cbPanel: cp, cbStat: true}, retryerMap: sync.Map{},
	}
}

// NewRetryContainer build Container that need to build circuit breaker and do circuit breaker statistic.
func NewRetryContainer() *Container {
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	return NewRetryContainerWithCBStat(cbs.ServiceControl(), cbs.ServicePanel())
}

// Container is a wrapper for Retryer.
type Container struct {
	hasCodeCfg  bool
	retryerMap  sync.Map // <method: retryer>
	cbContainer *cbContainer
	msg         string
	sync.RWMutex

	// shouldResultRetry is only used with FailureRetry
	shouldResultRetry *ShouldResultRetry
}

type cbContainer struct {
	cbCtl   *circuitbreak.Control
	cbPanel circuitbreaker.Panel
	cbStat  bool
}

// InitWithPolicies to init Retryer with methodPolicies
// Notice, InitWithPolicies is export func, the lock should be added inside
func (rc *Container) InitWithPolicies(methodPolicies map[string]Policy) error {
	if methodPolicies == nil {
		return nil
	}
	rc.Lock()
	defer rc.Unlock()
	var inited bool
	for m := range methodPolicies {
		if methodPolicies[m].Enable {
			inited = true
			if _, ok := rc.retryerMap.Load(m); ok {
				// NotifyPolicyChange may happen before
				continue
			}
			if err := rc.initRetryer(m, methodPolicies[m]); err != nil {
				rc.msg = err.Error()
				return err
			}
		}
	}
	rc.hasCodeCfg = inited
	return nil
}

// NotifyPolicyChange to receive policy when it changes
func (rc *Container) NotifyPolicyChange(method string, p Policy) {
	rc.Lock()
	defer rc.Unlock()
	rc.msg = ""
	if rc.hasCodeCfg {
		// the priority of user setup code policy is higher than remote config
		return
	}
	r, ok := rc.retryerMap.Load(method)
	if ok && r != nil {
		retryer, ok := r.(Retryer)
		if ok {
			if retryer.Type() == p.Type {
				retryer.UpdatePolicy(p)
				rc.msg = fmt.Sprintf("update retryer[%s-%s] at %s", method, retryer.Type(), time.Now())
				return
			}
			rc.retryerMap.Delete(method)
			rc.msg = fmt.Sprintf("delete retryer[%s-%s] at %s", method, retryer.Type(), time.Now())
		}
	}
	rc.initRetryer(method, p)
}

// Init to build Retryer with code config.
func (rc *Container) Init(mp map[string]Policy, rr *ShouldResultRetry) (err error) {
	// NotifyPolicyChange func may execute before Init func.
	// Because retry Container is built before Client init, NotifyPolicyChange can be triggered first
	rc.updateRetryer(rr)
	if err = rc.InitWithPolicies(mp); err != nil {
		return fmt.Errorf("NewRetryer in Init failed, err=%w", err)
	}
	return nil
}

// WithRetryIfNeeded to check if there is a retryer can be used and if current call can retry.
// When the retry condition is satisfied, use retryer to call
func (rc *Container) WithRetryIfNeeded(ctx context.Context, callOptRetry *Policy, rpcCall RPCCallFunc, ri rpcinfo.RPCInfo, request interface{}) (recycleRI bool, err error) {
	var retryer Retryer
	if callOptRetry != nil && callOptRetry.Enable {
		// build retryer for call level if retry policy is set up with callopt
		if retryer, err = NewRetryer(*callOptRetry, nil, rc.cbContainer); err != nil {
			klog.Warnf("KITEX: new callopt retryer[%s] failed, err=%w", callOptRetry.Type, err)
		}
	} else {
		retryer = rc.getRetryer(ri)
	}

	// case 1(default): no retry policy
	if retryer == nil {
		if _, _, err = rpcCall(ctx, nil); err == nil {
			return true, nil
		}
		return false, err
	}

	// case 2: setup retry policy, but not satisfy retry condition eg: circuit, retry times == 0, chain stop, ddl
	if msg, ok := retryer.AllowRetry(ctx); !ok {
		if _, _, err = rpcCall(ctx, retryer); err == nil {
			return true, err
		}
		if msg != "" {
			retryer.AppendErrMsgIfNeeded(err, ri, msg)
		}
		return false, err
	}

	// case 3: retry
	// reqOp is used to ignore req concurrent write
	reqOp := OpNo
	// respOp is used to ignore resp concurrent write and read, especially in backup request
	respOp := OpNo
	ctx = context.WithValue(ctx, CtxReqOp, &reqOp)
	ctx = context.WithValue(ctx, CtxRespOp, &respOp)

	// do rpc call with retry policy
	recycleRI, err = retryer.Do(ctx, rpcCall, ri, request)

	// the rpc call has finished, modify respOp to done state.
	atomic.StoreInt32(&respOp, OpDone)
	return
}

// NewRetryer build a retryer with policy
func NewRetryer(p Policy, r *ShouldResultRetry, cbC *cbContainer) (retryer Retryer, err error) {
	if !p.Enable {
		return nil, nil
	}
	// just one retry policy can be enabled at same time
	if p.Type == BackupType {
		retryer, err = newBackupRetryer(p, cbC)
	} else {
		retryer, err = newFailureRetryer(p, r, cbC)
	}
	return
}

func (rc *Container) getRetryer(ri rpcinfo.RPCInfo) Retryer {
	// the priority of specific method is high
	r, ok := rc.retryerMap.Load(ri.To().Method())
	if ok {
		return r.(Retryer)
	}
	r, ok = rc.retryerMap.Load(Wildcard)
	if ok {
		return r.(Retryer)
	}
	return nil
}

// Dump is used to show current retry policy
func (rc *Container) Dump() interface{} {
	rc.RLock()
	dm := make(map[string]interface{})
	dm["has_code_cfg"] = rc.hasCodeCfg
	rc.retryerMap.Range(func(key, value interface{}) bool {
		if r, ok := value.(Retryer); ok {
			dm[key.(string)] = r.Dump()
		}
		return true
	})
	if rc.msg != "" {
		dm["msg"] = rc.msg
	}
	rc.RUnlock()
	return dm
}

func (rc *Container) initRetryer(method string, p Policy) error {
	retryer, err := NewRetryer(p, rc.shouldResultRetry, rc.cbContainer)
	if err != nil {
		errMsg := fmt.Sprintf("new retryer[%s-%s] failed, err=%s, at %s", method, p.Type, err.Error(), time.Now())
		rc.msg = errMsg
		klog.Warnf(errMsg)
		return err
	}
	// NewRetryer can return nil if policy is nil
	if retryer != nil {
		rc.retryerMap.Store(method, retryer)
		rc.msg = fmt.Sprintf("new retryer[%s-%s] at %s", method, retryer.Type(), time.Now())
	} else {
		rc.msg = fmt.Sprintf("disable retryer[%s-%s](enable=%t) %s", method, p.Type, p.Enable, time.Now())
	}
	return nil
}

func (rc *Container) updateRetryer(rr *ShouldResultRetry) {
	rc.Lock()
	defer rc.Unlock()

	rc.shouldResultRetry = rr
	if rc.shouldResultRetry != nil {
		rc.retryerMap.Range(func(key, value interface{}) bool {
			if fr, ok := value.(*failureRetryer); ok {
				fr.setSpecifiedResultRetryIfNeeded(rc.shouldResultRetry)
			}
			return true
		})
	}
}
