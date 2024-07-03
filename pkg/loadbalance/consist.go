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

package loadbalance

import (
	"context"
	"github.com/cloudwego/kitex/pkg/loadbalance/newconsist"
	"sort"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/xxhash3"
	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/discovery"
)

/*
	Benchmark results with different instance numbers when weight = 10 and virtual factor = 100:
	BenchmarkNewConsistPicker_NoCache/10ins-16         	    6565	    160670 ns/op	  164750 B/op	       5 allocs/op
	BenchmarkNewConsistPicker_NoCache/100ins-16        	     571	   1914666 ns/op	 1611803 B/op	       6 allocs/op
	BenchmarkNewConsistPicker_NoCache/1000ins-16       	      45	  23485916 ns/op	16067720 B/op	      10 allocs/op
	BenchmarkNewConsistPicker_NoCache/10000ins-16      	       4	 251160920 ns/op	160405632 B/op	      41 allocs/op

	When there's 10000 instances which weight = 10 and virtual factor = 100, the time need to build is 251 ms.
*/

/*
  type hints for sync.Map：
	consistBalancer -> sync.Map[entry.CacheKey]*consistInfo
	consistInfo -> sync.Map[hashed key]*consistResult
	consistResult -> Primary, Replicas
	consistPicker -> consistResult
*/

// KeyFunc should return a non-empty string that stands for the request within the given context.
type KeyFunc func(ctx context.Context, request interface{}) string

// ConsistentHashOption .
type ConsistentHashOption struct {
	GetKey KeyFunc

	// If it is set, replicas will be used when connect to the primary node fails.
	// This brings extra mem and cpu cost.
	// If it is not set, error will be returned immediately when connect fails.
	Replica uint32

	// The number of virtual nodes corresponding to each real node
	// The larger the value, the higher the memory and computational cost, and the more balanced the load
	// When the number of nodes is large, it can be set smaller; conversely, it can be set larger
	// The median VirtualFactor * Weight (if Weighted is true) is recommended to be around 1000
	// The recommended total number of virtual nodes is within 2000W (it takes 250ms to build once in the 1000W case, but it is theoretically fine to build in the background within 3s)
	VirtualFactor uint32

	// Whether to follow Weight for load balancing
	// If false, Weight is ignored for each instance, and VirtualFactor virtual nodes are generated for indiscriminate load balancing
	// if true, Weight() * VirtualFactor virtual nodes are generated for each instance
	// Note that for instance with weight 0, no virtual nodes will be generated regardless of the VirtualFactor number
	// It is recommended to set it to true, but be careful to reduce the VirtualFactor appropriately
	Weighted bool

	// Deprecated: This implementation will not cache all the keys anymore, ExpireDuration will not take effect
	// Whether or not to perform expiration processing
	// The implementation will cache all the keys
	// If never expired it may cause memory to keep growing and eventually OOM
	// Setting expiration will result in additional performance overhead
	// Current implementations scan for deletions every minute, and delete once when the instance changes rebuild
	// It is recommended to always set the value not less than two minutes
	ExpireDuration time.Duration
}

// NewConsistentHashOption creates a default ConsistentHashOption.
func NewConsistentHashOption(f KeyFunc) ConsistentHashOption {
	return ConsistentHashOption{
		GetKey:        f,
		Replica:       0,
		VirtualFactor: 100,
		Weighted:      true,
	}
}

var consistPickerPool sync.Pool

func init() {
	consistPickerPool.New = newConsistPicker
}

type virtualNode struct {
	hash     uint64
	RealNode *realNode
}

type realNode struct {
	Ins discovery.Instance
}

type consistResult struct {
	Primary  discovery.Instance
	Replicas []discovery.Instance
}

type consistInfo struct {
	realNodes    []realNode
	virtualNodes []virtualNode
}

type vNodeType struct {
	s []virtualNode
}

func (v *vNodeType) Len() int {
	return len(v.s)
}

func (v *vNodeType) Less(i, j int) bool {
	return v.s[i].hash < v.s[j].hash
}

func (v *vNodeType) Swap(i, j int) {
	v.s[i], v.s[j] = v.s[j], v.s[i]
}

type consistPicker struct {
	cb   *consistBalancer
	info *newconsist.ConsistInfo
	// index  int
	// result *consistResult
}

func newConsistPicker() interface{} {
	return &consistPicker{}
}

func (cp *consistPicker) zero() {
	cp.info = nil
	cp.cb = nil
	// cp.index = 0
	// cp.result = nil
}

func (cp *consistPicker) Recycle() {
	cp.zero()
	consistPickerPool.Put(cp)
}

// Next is not concurrency safe.
func (cp *consistPicker) Next(ctx context.Context, request interface{}) discovery.Instance {
	if cp.info.IsEmpty() {
		return nil
	}
	key := cp.cb.opt.GetKey(ctx, request)
	if key == "" {
		return nil
	}
	return cp.info.BuildConsistentResult(xxhash3.HashString(key))
	// Todo(DMwangnima): Optimise Replica-related logic
	// This comment part is previous implementation considering connecting to Replica
	// Since we would create a new picker each time, the Replica logic is unreachable, so just comment it out for now

	//if cp.result == nil {
	//	key := cp.cb.opt.GetKey(ctx, request)
	//	if key == "" {
	//		return nil
	//	}
	//	cp.result = buildConsistResult(cp.cb, cp.info, xxhash3.HashString(key))
	//	//cp.index = 0
	//	return cp.result.Primary
	//}
	//if cp.index < len(cp.result.Replicas) {
	//	cp.index++
	//	return cp.result.Replicas[cp.index-1]
	//}
	//return nil
}

func buildConsistResult(info *consistInfo, key uint64) *consistResult {
	cr := &consistResult{}
	index := sort.Search(len(info.virtualNodes), func(i int) bool {
		return info.virtualNodes[i].hash > key
	})
	// Back to the ring head (although a ring does not have a head)
	if index == len(info.virtualNodes) {
		index = 0
	}
	cr.Primary = info.virtualNodes[index].RealNode.Ins
	return cr
	// Todo(DMwangnima): Optimise Replica-related logic
	// This comment part is previous implementation considering connecting to Replica
	// Since we would create a new picker each time, the Replica logic is unreachable, so just comment it out
	// for better performance

	//replicas := int(cb.opt.Replica)
	//// remove the primary node
	//if len(info.realNodes)-1 < replicas {
	//	replicas = len(info.realNodes) - 1
	//}
	//if replicas > 0 {
	//	used := make(map[discovery.Instance]struct{}, replicas) // should be 1 + replicas - 1
	//	used[cr.Primary] = struct{}{}
	//	cr.Replicas = make([]discovery.Instance, replicas)
	//	for i := 0; i < replicas; i++ {
	//		// find the next instance which is not used
	//		// replicas are adjusted before so we can guarantee that we can find one
	//		for {
	//			index++
	//			if index == len(info.virtualNodes) {
	//				index = 0
	//			}
	//			ins := info.virtualNodes[index].RealNode.Ins
	//			if _, ok := used[ins]; !ok {
	//				used[ins] = struct{}{}
	//				cr.Replicas[i] = ins
	//				break
	//			}
	//		}
	//	}
	//}
	//return cr
}

type consistBalancer struct {
	cachedConsistInfo sync.Map
	opt               ConsistentHashOption
	sfg               singleflight.Group
}

// NewConsistBalancer creates a new consist balancer with the given option.
func NewConsistBalancer(opt ConsistentHashOption) Loadbalancer {
	if opt.GetKey == nil {
		panic("loadbalancer: new consistBalancer failed, getKey func cannot be nil")
	}
	if opt.VirtualFactor == 0 {
		panic("loadbalancer: new consistBalancer failed, virtual factor must > 0")
	}
	cb := &consistBalancer{
		opt: opt,
	}
	return cb
}

// GetPicker implements the Loadbalancer interface.
func (cb *consistBalancer) GetPicker(e discovery.Result) Picker {
	var ci *newconsist.ConsistInfo
	if e.Cacheable {
		cii, ok := cb.cachedConsistInfo.Load(e.CacheKey)
		if !ok {
			cii, _, _ = cb.sfg.Do(e.CacheKey, func() (interface{}, error) {
				return cb.newConsistInfo(e), nil
			})
			cb.cachedConsistInfo.Store(e.CacheKey, cii)
		}
		ci = cii.(*newconsist.ConsistInfo)
	} else {
		ci = cb.newConsistInfo(e)
	}
	picker := consistPickerPool.Get().(*consistPicker)
	picker.cb = cb
	picker.info = ci
	return picker
}

func (cb *consistBalancer) newConsistInfo(e discovery.Result) *newconsist.ConsistInfo {
	return newconsist.NewConsistInfo(e, newconsist.ConsistInfoConfig{VirtualFactor: cb.opt.VirtualFactor, Weighted: cb.opt.Weighted})
}

// get virtual node number from one realNode.
// if cb.opt.Weighted option is false, multiplier is 1, virtual node number is equal to VirtualFactor.
func (cb *consistBalancer) getVirtualNodeLen(rNode realNode) int {
	if cb.opt.Weighted {
		return rNode.Ins.Weight() * int(cb.opt.VirtualFactor)
	}
	return int(cb.opt.VirtualFactor)
}

func (cb *consistBalancer) updateConsistInfo(e discovery.Result) {
	newInfo := cb.newConsistInfo(e)
	cb.cachedConsistInfo.Store(e.CacheKey, newInfo)
}

// Rebalance implements the Rebalancer interface.
func (cb *consistBalancer) Rebalance(change discovery.Change) {
	if !change.Result.Cacheable {
		return
	}
	if ci, ok := cb.cachedConsistInfo.Load(change.Result.CacheKey); ok {
		ci.(*newconsist.ConsistInfo).Rebalance(change)
	} else {
		cb.updateConsistInfo(change.Result)
	}
}

// Delete implements the Rebalancer interface.
func (cb *consistBalancer) Delete(change discovery.Change) {
	if !change.Result.Cacheable {
		return
	}
	// FIXME: If Delete and Rebalance occur together (Discovery OnDelete and OnChange are triggered at the same time),
	// it may cause the delete to fail and eventually lead to a resource leak.
	cb.cachedConsistInfo.Delete(change.Result.CacheKey)
}

func (cb *consistBalancer) Name() string {
	return "consist"
}
