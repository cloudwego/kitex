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
	"os"
	"sync"

	goid "github.com/choleraehyq/pid"

	"github.com/cloudwego/kitex/internal"
)

var (
	rpcInfoPool sync.Pool
	forceCheck  = true
)

func init() {
	if os.Getenv("KITEX_RPCINFO_DISABLE_CHECK") != "" {
		forceCheck = false
	}
}

type rpcInfo struct {
	from       EndpointInfo
	to         EndpointInfo
	invocation Invocation
	config     RPCConfig
	stats      RPCStats
	gid        int64
}

func (r *rpcInfo) checkGid() {
	if forceCheck && goid.Get() != r.gid {
		panic("unexpected reference of RPCInfo in another goroutine. " +
			"Please use the return value of FreezeRPCInfo(rpcinfo) before passing rpcinfo to new goroutines, " +
			"or disable the pool by `rpcinfo.EnablePool(false)`")
	}
}

// From implements the RPCInfo interface.
func (r *rpcInfo) From() EndpointInfo {
	r.checkGid()
	return r.from
}

// To implements the RPCInfo interface.
func (r *rpcInfo) To() EndpointInfo {
	r.checkGid()
	return r.to
}

// Config implements the RPCInfo interface.
func (r *rpcInfo) Config() RPCConfig {
	r.checkGid()
	return r.config
}

// Invocation implements the RPCInfo interface.
func (r *rpcInfo) Invocation() Invocation {
	r.checkGid()
	return r.invocation
}

// Stats implements the RPCInfo interface.
func (r *rpcInfo) Stats() RPCStats {
	r.checkGid()
	return r.stats
}

func (r *rpcInfo) zero() {
	r.from = nil
	r.to = nil
	r.invocation = nil
	r.config = nil
	r.stats = nil
}

// Recycle reuses the rpcInfo.
func (r *rpcInfo) Recycle() {
	if v, ok := r.from.(internal.Reusable); ok {
		v.Recycle()
	}
	if v, ok := r.to.(internal.Reusable); ok {
		v.Recycle()
	}
	if v, ok := r.invocation.(internal.Reusable); ok {
		v.Recycle()
	}
	if v, ok := r.config.(internal.Reusable); ok {
		v.Recycle()
	}
	if v, ok := r.stats.(internal.Reusable); ok {
		v.Recycle()
	}
	r.zero()
	rpcInfoPool.Put(r)
}

func init() {
	rpcInfoPool.New = newRPCInfo
}

// NewRPCInfo creates a new RPCInfo using the given information.
func NewRPCInfo(from, to EndpointInfo, ink Invocation, config RPCConfig, stats RPCStats) RPCInfo {
	r := rpcInfoPool.Get().(*rpcInfo)
	r.from = from
	r.to = to
	r.invocation = ink
	r.config = config
	r.stats = stats
	r.gid = goid.Get()
	return r
}

func newRPCInfo() interface{} {
	return &rpcInfo{}
}
