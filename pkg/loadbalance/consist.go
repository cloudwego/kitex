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
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/util/xxhash3"
	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/utils"
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
		GetKey:         f,
		Replica:        0,
		VirtualFactor:  100,
		Weighted:       true,
		ExpireDuration: 2 * time.Minute,
	}
}

var (
	consistPickerPool         sync.Pool
	consistBalancers          []*consistBalancer
	consistBalancersLock      sync.RWMutex
	consistBalancerDaemonOnce sync.Once
)

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
	Touch    atomic.Value
}

type consistInfo struct {
	cachedConsistResult sync.Map
	sfg                 singleflight.Group // To prevent multiple builds on the first request for the same key

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
	cb     *consistBalancer
	info   *consistInfo
	index  int
	result *consistResult
}

func newConsistPicker() interface{} {
	return &consistPicker{}
}

func (cp *consistPicker) zero() {
	cp.info = nil
	cp.cb = nil
	cp.index = 0
	cp.result = nil
}

func (cp *consistPicker) Recycle() {
	cp.zero()
	consistPickerPool.Put(cp)
}

// Next is not concurrency safe.
func (cp *consistPicker) Next(ctx context.Context, request interface{}) discovery.Instance {
	if len(cp.info.realNodes) == 0 {
		return nil
	}
	if cp.result == nil {
		key := cp.cb.opt.GetKey(ctx, request)
		if key == "" {
			return nil
		}
		cp.result = cp.getConsistResult(xxhash3.HashString(key))
		cp.index = 0
		return cp.result.Primary
	}
	if cp.index < len(cp.result.Replicas) {
		cp.index++
		return cp.result.Replicas[cp.index-1]
	}
	return nil
}

func (cp *consistPicker) getConsistResult(key uint64) *consistResult {
	var cr *consistResult
	cri, ok := cp.info.cachedConsistResult.Load(key)
	if !ok {
		cri, _, _ = cp.info.sfg.Do(strconv.FormatUint(key, 10), func() (interface{}, error) {
			cr := buildConsistResult(cp.cb, cp.info, key)
			if cp.cb.opt.ExpireDuration > 0 {
				cr.Touch.Store(time.Now())
			}
			return cr, nil
		})
		cp.info.cachedConsistResult.Store(key, cri)
	}
	cr = cri.(*consistResult)
	if cp.cb.opt.ExpireDuration > 0 {
		cr.Touch.Store(time.Now())
	}
	return cr
}

func buildConsistResult(cb *consistBalancer, info *consistInfo, key uint64) *consistResult {
	cr := &consistResult{}
	index := sort.Search(len(info.virtualNodes), func(i int) bool {
		return info.virtualNodes[i].hash > key
	})
	// Back to the ring head (although a ring does not have a head)
	if index == len(info.virtualNodes) {
		index = 0
	}
	cr.Primary = info.virtualNodes[index].RealNode.Ins
	replicas := int(cb.opt.Replica)
	// remove the primary node
	if len(info.realNodes)-1 < replicas {
		replicas = len(info.realNodes) - 1
	}
	if replicas > 0 {
		used := make(map[discovery.Instance]struct{}, replicas) // should be 1 + replicas - 1
		used[cr.Primary] = struct{}{}
		cr.Replicas = make([]discovery.Instance, replicas)
		for i := 0; i < replicas; i++ {
			// find the next instance which is not used
			// replicas are adjusted before so we can guarantee that we can find one
			for {
				index++
				if index == len(info.virtualNodes) {
					index = 0
				}
				ins := info.virtualNodes[index].RealNode.Ins
				if _, ok := used[ins]; !ok {
					used[ins] = struct{}{}
					cr.Replicas[i] = ins
					break
				}
			}
		}
	}
	return cr
}

type consistBalancer struct {
	cachedConsistInfo sync.Map
	// The main purpose of this lock is to improve performance and prevent Change from being performed while expire
	// which may cause Change to do a lot of extra computation and memory allocation
	updateLock sync.Mutex
	opt        ConsistentHashOption
	sfg        singleflight.Group
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
	if cb.opt.ExpireDuration > 0 {
		cb.AddToDaemon()
	}
	return cb
}

// AddToDaemon adds a balancer to the daemon expire routine.
func (cb *consistBalancer) AddToDaemon() {
	// do delete func
	consistBalancerDaemonOnce.Do(func() {
		gofunc.GoFunc(context.Background(), func() {
			for range time.Tick(2 * time.Minute) {
				consistBalancersLock.RLock()
				now := time.Now()
				for _, lb := range consistBalancers {
					if lb.opt.ExpireDuration > 0 {
						lb.updateLock.Lock()
						lb.cachedConsistInfo.Range(func(key, value interface{}) bool {
							ci := value.(*consistInfo)
							ci.cachedConsistResult.Range(func(key, value interface{}) bool {
								t := value.(*consistResult).Touch.Load().(time.Time)
								if now.After(t.Add(cb.opt.ExpireDuration)) {
									ci.cachedConsistResult.Delete(key)
								}
								return true
							})
							return true
						})
						lb.updateLock.Unlock()
					}
				}
				consistBalancersLock.RUnlock()
			}
		})
	})

	consistBalancersLock.Lock()
	consistBalancers = append(consistBalancers, cb)
	consistBalancersLock.Unlock()
}

// GetPicker implements the Loadbalancer interface.
func (cb *consistBalancer) GetPicker(e discovery.Result) Picker {
	var ci *consistInfo
	if e.Cacheable {
		cii, ok := cb.cachedConsistInfo.Load(e.CacheKey)
		if !ok {
			cii, _, _ = cb.sfg.Do(e.CacheKey, func() (interface{}, error) {
				return cb.newConsistInfo(e), nil
			})
			cb.cachedConsistInfo.Store(e.CacheKey, cii)
		}
		ci = cii.(*consistInfo)
	} else {
		ci = cb.newConsistInfo(e)
	}
	picker := consistPickerPool.Get().(*consistPicker)
	picker.cb = cb
	picker.info = ci
	return picker
}

func (cb *consistBalancer) newConsistInfo(e discovery.Result) *consistInfo {
	ci := &consistInfo{}
	ci.realNodes, ci.virtualNodes = cb.buildNodes(e.Instances)
	return ci
}

func (cb *consistBalancer) buildNodes(ins []discovery.Instance) ([]realNode, []virtualNode) {
	ret := make([]realNode, len(ins))
	for i := range ins {
		ret[i].Ins = ins[i]
	}
	return ret, cb.buildVirtualNodes(ret)
}

func (cb *consistBalancer) buildVirtualNodes(rNodes []realNode) []virtualNode {
	totalLen := 0
	for i := range rNodes {
		totalLen += cb.getVirtualNodeLen(rNodes[i])
	}

	ret := make([]virtualNode, totalLen)
	if totalLen == 0 {
		return ret
	}
	maxLen, maxSerial := 0, 0
	for i := range rNodes {
		if len(rNodes[i].Ins.Address().String()) > maxLen {
			maxLen = len(rNodes[i].Ins.Address().String())
		}
		if vNodeLen := cb.getVirtualNodeLen(rNodes[i]); vNodeLen > maxSerial {
			maxSerial = vNodeLen
		}
	}
	l := maxLen + 1 + utils.GetUIntLen(uint64(maxSerial)) // "$address + # + itoa(i)"
	// pre-allocate []byte here, and reuse it to prevent memory allocation.
	b := make([]byte, l)

	// record the start index.
	cur := 0
	for i := range rNodes {
		bAddr := utils.StringToSliceByte(rNodes[i].Ins.Address().String())
		// Assign the first few bits of b to string.
		copy(b, bAddr)

		// Initialize the last few bits, skipping '#'.
		for j := len(bAddr) + 1; j < len(b); j++ {
			b[j] = 0
		}
		b[len(bAddr)] = '#'

		vLen := cb.getVirtualNodeLen(rNodes[i])
		for j := 0; j < vLen; j++ {
			k := j
			cnt := 0
			// Assign values to b one by one, starting with the last one.
			for k > 0 {
				b[l-1-cnt] = byte(k % 10)
				k /= 10
				cnt++
			}
			// At this point, the index inside ret should be cur + j.
			index := cur + j
			ret[index].hash = xxhash3.Hash(b)
			ret[index].RealNode = &rNodes[i]
		}
		cur += vLen
	}
	sort.Sort(&vNodeType{s: ret})
	return ret
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
	infoI, loaded := cb.cachedConsistInfo.LoadOrStore(e.CacheKey, newInfo)
	if !loaded {
		return
	}
	info := infoI.(*consistInfo)
	// Warm up.
	// The reason for not modifying info directly is that there is no guarantee of concurrency security.
	info.cachedConsistResult.Range(func(key, value interface{}) bool {
		cr := buildConsistResult(cb, newInfo, key.(uint64))
		if cb.opt.ExpireDuration > 0 {
			t := value.(*consistResult).Touch.Load().(time.Time)
			if time.Now().After(t.Add(cb.opt.ExpireDuration)) {
				return true
			}
			cr.Touch.Store(t)
		}
		newInfo.cachedConsistResult.Store(key, cr)
		return true
	})
	cb.cachedConsistInfo.Store(e.CacheKey, newInfo)
}

// Rebalance implements the Rebalancer interface.
func (cb *consistBalancer) Rebalance(change discovery.Change) {
	if !change.Result.Cacheable {
		return
	}
	// TODO: Use TreeMap to optimize performance when updating.
	// Now, due to the lack of a good red-black tree implementation, we can only build the full amount once per update.
	cb.updateLock.Lock()
	cb.updateConsistInfo(change.Result)
	cb.updateLock.Unlock()
}

// Delete implements the Rebalancer interface.
func (cb *consistBalancer) Delete(change discovery.Change) {
	if !change.Result.Cacheable {
		return
	}
	// FIXME: If Delete and Rebalance occur together (Discovery OnDelete and OnChange are triggered at the same time),
	// it may cause the delete to fail and eventually lead to a resource leak.
	cb.updateLock.Lock()
	cb.cachedConsistInfo.Delete(change.Result.CacheKey)
	cb.updateLock.Unlock()
}

func (cb *consistBalancer) Name() string {
	return "consist"
}
