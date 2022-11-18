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
	"strings"
	"sync"

	"github.com/cloudwego/kitex/internal"
	"github.com/cloudwego/kitex/pkg/consts"
)

var rpcInfoPool sync.Pool

var _ RPCInfo = (*rpcInfo)(nil)

type rpcInfo struct {
	from       EndpointInfo
	to         EndpointInfo
	invocation Invocation
	config     RPCConfig
	stats      RPCStats
	key        string
}

// From implements the RPCInfo interface.
func (r *rpcInfo) From() EndpointInfo { return r.from }

// To implements the RPCInfo interface.
func (r *rpcInfo) To() EndpointInfo { return r.to }

// Config implements the RPCInfo interface.
func (r *rpcInfo) Config() RPCConfig { return r.config }

// Invocation implements the RPCInfo interface.
func (r *rpcInfo) Invocation() Invocation { return r.invocation }

// Stats implements the RPCInfo interface.
func (r *rpcInfo) Stats() RPCStats { return r.stats }

// Key implements the RPCInfo interface.
func (r *rpcInfo) Key() string {
	if r.key != "" {
		return r.key
	}
	if r == nil || r.From() == nil || r.To() == nil {
		return ""
	}
	fromService := r.From().ServiceName()
	fromCluster := r.From().DefaultTag(consts.Cluster, consts.DefaultCluster)
	toService := r.To().ServiceName()
	toCluster := r.To().DefaultTag(consts.Cluster, consts.DefaultCluster)
	method := r.To().Method()

	sum := len(fromService) + len(fromCluster) + len(toService) + len(toCluster) + len(method) + 4
	var buf strings.Builder
	buf.Grow(sum)
	buf.WriteString(fromService)
	buf.WriteByte('/')
	buf.WriteString(fromCluster)
	buf.WriteByte('/')
	buf.WriteString(toService)
	buf.WriteByte('/')
	buf.WriteString(toCluster)
	buf.WriteByte('/')
	buf.WriteString(method)
	r.key = buf.String()
	return r.key
}

func (r *rpcInfo) zero() {
	r.from = nil
	r.to = nil
	r.invocation = nil
	r.config = nil
	r.stats = nil
	r.key = ""
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
	r.key = ""
	return r
}

func newRPCInfo() interface{} {
	return &rpcInfo{}
}
