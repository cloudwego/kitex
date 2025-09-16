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

package rpcinfo

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	// InvocationServiceInfoKey is the extraInfo key of invocation which stores the ServiceInfo of the rpc call.
	// The reason for adding the extraInfo key is to shield ServiceInfo from users in the Invocation interface definition,
	// as it is a pointer type and may be modified insecurely if obtained by users.
	InvocationServiceInfoKey = RegisterInvocationExtraKey("service_info", false)
)

var (
	_              Invocation       = (*invocation)(nil)
	_              InvocationSetter = (*invocation)(nil)
	invocationPool sync.Pool
	globalSeqID    int32 = 0
)

func init() {
	invocationPool.New = newInvocation
}

// InvocationSetter is used to set information about an RPC.
type InvocationSetter interface {
	SetPackageName(name string)
	SetServiceName(name string)
	SetMethodName(name string)
	SetMethodInfo(methodInfo serviceinfo.MethodInfo)
	SetStreamingMode(mode serviceinfo.StreamingMode)
	SetSeqID(seqID int32)
	SetBizStatusErr(err kerrors.BizStatusErrorIface)
	SetExtraInfo(key *InvocationExtraIndex, value interface{})
	Reset()

	// Deprecated, use SetExtraInfo instead
	SetExtra(key string, value interface{})
}

type invocation struct {
	packageName   string
	serviceName   string
	methodInfo    serviceinfo.MethodInfo
	methodName    string
	streamingMode serviceinfo.StreamingMode
	seqID         int32
	// bizErr and extra should be protected by atomic operation, because they might be read by the client calling goroutine,
	// but at the same time, written by the real rpc goroutine which is started by timeout middleware.
	bizErr          atomic.Pointer[kerrors.BizStatusErrorIface]
	extraInfo       []interface{}
	atomicExtraInfo []atomic.Pointer[interface{}] // of type *interface{}

	// deprecated
	extra map[string]interface{}
}

// NewInvocation creates a new Invocation with the given service, method and optional package.
func NewInvocation(service, method string, pkgOpt ...string) *invocation {
	ivk := invocationPool.Get().(*invocation)
	ivk.seqID = genSeqID()
	ivk.serviceName = service
	ivk.methodName = method
	if len(pkgOpt) > 0 {
		ivk.packageName = pkgOpt[0]
	}
	return ivk
}

// NewServerInvocation to get Invocation for new request in server side
func NewServerInvocation() Invocation {
	ivk := invocationPool.Get().(*invocation)
	return ivk
}

func genSeqID() int32 {
	id := atomic.AddInt32(&globalSeqID, 1)
	if id == 0 {
		// seqID is non-0 to avoid potential default value judgments leading to error handling
		id = atomic.AddInt32(&globalSeqID, 1)
	}
	return id
}

func newInvocation() interface{} {
	return &invocation{}
}

// SeqID implements the Invocation interface.
func (i *invocation) SeqID() int32 {
	return i.seqID
}

// SetSeqID implements the InvocationSetter interface.
func (i *invocation) SetSeqID(seqID int32) {
	i.seqID = seqID
}

func (i *invocation) PackageName() string {
	return i.packageName
}

func (i *invocation) SetPackageName(name string) {
	i.packageName = name
}

func (i *invocation) ServiceName() string {
	return i.serviceName
}

// SetServiceName implements the InvocationSetter interface.
func (i *invocation) SetServiceName(name string) {
	i.serviceName = name
}

// MethodName implements the Invocation interface.
func (i *invocation) MethodName() string {
	return i.methodName
}

// SetMethodName implements the InvocationSetter interface.
func (i *invocation) SetMethodName(name string) {
	i.methodName = name
}

// MethodInfo implements the Invocation interface.
func (i *invocation) MethodInfo() serviceinfo.MethodInfo {
	return i.methodInfo
}

// SetMethodInfo implements the InvocationSetter interface.
func (i *invocation) SetMethodInfo(methodInfo serviceinfo.MethodInfo) {
	i.methodInfo = methodInfo
}

// StreamingMode implements the Invocation interface.
func (i *invocation) StreamingMode() serviceinfo.StreamingMode {
	return i.streamingMode
}

// SetStreamingMode implements the InvocationSetter interface.
func (i *invocation) SetStreamingMode(mode serviceinfo.StreamingMode) {
	i.streamingMode = mode
}

// BizStatusErr implements the Invocation interface.
func (i *invocation) BizStatusErr() kerrors.BizStatusErrorIface {
	bizErr := i.bizErr.Load()
	if bizErr == nil {
		return nil
	}
	return *bizErr
}

// SetBizStatusErr implements the InvocationSetter interface.
func (i *invocation) SetBizStatusErr(err kerrors.BizStatusErrorIface) {
	if err == nil {
		i.bizErr.Store(nil)
		return
	}
	i.bizErr.Store(&err)
}

func (i *invocation) SetExtraInfo(key *InvocationExtraIndex, value interface{}) {
	if key.Atomic() {
		if i.atomicExtraInfo == nil {
			i.atomicExtraInfo = make([]atomic.Pointer[interface{}], len(indexedAtomicExtraKeys))
		}
		i.atomicExtraInfo[key.Index()].Store(&value)
	} else {
		if i.extraInfo == nil {
			i.extraInfo = make([]interface{}, len(indexedExtraKeys))
		}
		i.extraInfo[key.Index()] = value
	}
}

func (i *invocation) ExtraInfo(key *InvocationExtraIndex) interface{} {
	if key.Atomic() {
		val := i.atomicExtraInfo[key.Index()].Load()
		if val == nil {
			return nil
		}
		return *val
	} else {
		return i.extraInfo[key.Index()]
	}
}

func (i *invocation) SetExtra(key string, value interface{}) {
	if i.extra == nil {
		i.extra = map[string]interface{}{}
	}
	i.extra[key] = value
}

func (i *invocation) Extra(key string) interface{} {
	if i.extra == nil {
		return nil
	}
	return i.extra[key]
}

// Reset implements the InvocationSetter interface.
func (i *invocation) Reset() {
	i.zero()
}

// Recycle reuses the invocation.
func (i *invocation) Recycle() {
	i.zero()
	invocationPool.Put(i)
}

func (i *invocation) zero() {
	i.seqID = 0
	i.packageName = ""
	i.serviceName = ""
	i.methodName = ""
	i.methodInfo = nil
	i.bizErr.Store(nil)
	for idx := range i.extraInfo {
		i.extraInfo[idx] = nil
	}
	for idx := range i.atomicExtraInfo {
		i.atomicExtraInfo[idx].Store(nil)
	}
	if i.extra != nil {
		for k := range i.extra {
			delete(i.extra, k)
		}
	}
}

var (
	invocationExtraMu      sync.Mutex
	extraKeysMap           = make(map[string]*InvocationExtraIndex, 32)
	indexedExtraKeys       = make([]*InvocationExtraIndex, 0, 32)
	indexedAtomicExtraKeys = make([]*InvocationExtraIndex, 0, 32)
)

// RegisterInvocationExtraKey register a new extraInfo key of invocation.
// The name of the extraInfo key must be unique, and the atomic parameter indicates whether the extraInfo key is atomic.
// Note: MUST be called in init function.
func RegisterInvocationExtraKey(name string, atomic bool) *InvocationExtraIndex {
	invocationExtraMu.Lock()
	defer invocationExtraMu.Unlock()
	if _, ok := extraKeysMap[name]; ok {
		panic(fmt.Sprintf("duplicate extraInfo key name: %s", name))
	}
	var index *InvocationExtraIndex
	if atomic {
		index = &InvocationExtraIndex{
			name:   name,
			atomic: true,
			index:  uint32(len(indexedAtomicExtraKeys)),
		}
		indexedAtomicExtraKeys = append(indexedAtomicExtraKeys, index)
	} else {
		index = &InvocationExtraIndex{
			name:  name,
			index: uint32(len(indexedExtraKeys)),
		}
		indexedExtraKeys = append(indexedExtraKeys, index)
	}
	extraKeysMap[name] = index
	return index
}

type InvocationExtraIndex struct {
	name   string
	index  uint32
	atomic bool
}

func (i *InvocationExtraIndex) String() string {
	return i.name
}

func (i *InvocationExtraIndex) Index() uint32 {
	return i.index
}

func (i *InvocationExtraIndex) Atomic() bool {
	return i.atomic
}
