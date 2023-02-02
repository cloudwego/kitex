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
	"math/rand"
	"sync"

	"github.com/bytedance/gopkg/lang/fastrand"
	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
)

var weightedPickerPool, randomPickerPool sync.Pool

func init() {
	weightedPickerPool.New = newWeightedPicker
	randomPickerPool.New = newRandomPicker
}

type entry = int

type weightedPicker struct {
	immutableInstances []discovery.Instance
	immutableEntries   []entry
	weightSum          int

	copiedInstances []discovery.Instance
	copiedEntries   []entry
	firstIndex      int
}

func newWeightedPicker() interface{} {
	return &weightedPicker{
		firstIndex: -1,
	}
}

// Next implements the Picker interface.
func (wp *weightedPicker) Next(ctx context.Context, request interface{}) (ins discovery.Instance) {
	defer func() {
		if ins != nil {
			klog.Infof("KITEX: weighted picker pick: %s", ins.Address())
		}
	}()

	if wp.firstIndex >= len(wp.immutableInstances) {
		wp.firstIndex = -1
	}

	weightSum := wp.weightSum
	if wp.firstIndex >= 0 {
		weightSum -= wp.immutableInstances[wp.firstIndex].Weight()
	}
	//rand.Seed(time.Now().UnixNano() + int64(wp.firstIndex))
	weight := int(rand.Int31n(int32(weightSum)))
	//weight := fastrand.Intn(weightSum)
	//n, _ := rand.Int(rand.Reader, big.NewInt(int64(weightSum)))
	//weight := int(n.Int64())
	//fmt.Printf("random weight: %d, weightSum: %d, lastIndex: %d\n", weight, weightSum, wp.firstIndex)
	for i := 0; i < len(wp.immutableEntries); i++ {
		if i == wp.firstIndex {
			continue
		}
		weight -= wp.immutableEntries[i]
		if weight < 0 {
			wp.firstIndex = i
			break
		}
	}
	if wp.firstIndex < 0 {
		return nil
	}

	ins = wp.immutableInstances[wp.firstIndex]
	//fmt.Printf("immutableInstances: %d, immutableEntries: %d, firstIndex: %d\n",
	//	len(wp.immutableInstances), len(wp.immutableEntries), wp.firstIndex)
	return ins
}

func (wp *weightedPicker) zero() {
	wp.immutableInstances = nil
	wp.immutableEntries = nil
	wp.weightSum = 0
	wp.copiedInstances = nil
	wp.copiedEntries = nil
}

func (wp *weightedPicker) Recycle() {
	wp.zero()
	weightedPickerPool.Put(wp)
}

type randomPicker struct {
	immutableInstances []discovery.Instance
	firstIndex         int
	copiedInstances    []discovery.Instance
}

func newRandomPicker() interface{} {
	return &randomPicker{
		firstIndex: -1,
	}
}

// Next implements the Picker interface.
func (rp *randomPicker) Next(ctx context.Context, request interface{}) (ins discovery.Instance) {
	defer func() {
		if ins != nil {
			klog.Infof("KITEX: random picker pick: %s", ins.Address())
		}
	}()

	if rp.firstIndex < 0 {
		rp.firstIndex = fastrand.Intn(len(rp.immutableInstances))
		ins = rp.immutableInstances[rp.firstIndex]
		return ins
	}

	if rp.copiedInstances == nil {
		rp.copiedInstances = make([]discovery.Instance, len(rp.immutableInstances)-1)
		copy(rp.copiedInstances, rp.immutableInstances[:rp.firstIndex])
		copy(rp.copiedInstances[rp.firstIndex:], rp.immutableInstances[rp.firstIndex+1:])
	}

	n := len(rp.copiedInstances)
	if n > 0 {
		index := fastrand.Intn(n)
		ins = rp.copiedInstances[index]
		rp.copiedInstances[index] = rp.copiedInstances[n-1]
		rp.copiedInstances = rp.copiedInstances[:n-1]
		return ins
	}

	return nil
}

func (rp *randomPicker) zero() {
	rp.copiedInstances = nil
	rp.immutableInstances = nil
	rp.firstIndex = -1
}

func (rp *randomPicker) Recycle() {
	rp.zero()
	randomPickerPool.Put(rp)
}

type weightInfo struct {
	balance   bool
	instances []discovery.Instance
	entries   []entry
	weightSum int
	picker    *RoundPicker
}

type weightedBalancer struct {
	cachedWeightInfo sync.Map
	sfg              singleflight.Group
}

// NewWeightedBalancer creates a loadbalancer using weighted-round-robin algorithm.
func NewWeightedBalancer() Loadbalancer {
	lb := &weightedBalancer{}
	return lb
}

// GetPicker implements the Loadbalancer interface.
func (wb *weightedBalancer) GetPicker(e discovery.Result) Picker {
	var w *weightInfo
	if e.Cacheable {
		wi, ok := wb.cachedWeightInfo.Load(e.CacheKey)
		if !ok {
			wi, _, _ = wb.sfg.Do(e.CacheKey, func() (interface{}, error) {
				return wb.calcWeightInfo(e), nil
			})
			wb.cachedWeightInfo.Store(e.CacheKey, wi)
		}
		w = wi.(*weightInfo)
	} else {
		w = wb.calcWeightInfo(e)
	}

	if w.weightSum == 0 {
		return new(DummyPicker)
	}

	return w.picker
	//if w.balance {
	//	picker := randomPickerPool.Get().(*randomPicker)
	//	picker.immutableInstances = w.instances
	//	picker.firstIndex = -1
	//	return picker
	//}
	//picker := weightedPickerPool.Get().(*weightedPicker)
	//picker.immutableInstances = w.instances
	//picker.immutableEntries = w.entries
	//picker.weightSum = w.weightSum
	//return picker
}

func (wb *weightedBalancer) calcWeightInfo(e discovery.Result) *weightInfo {
	w := &weightInfo{
		balance:   true,
		instances: make([]discovery.Instance, len(e.Instances)),
		entries:   make([]entry, len(e.Instances)),
		weightSum: 0,
		picker:    &RoundPicker{},
	}

	var cnt int
	for idx := range e.Instances {
		weight := e.Instances[idx].Weight()
		if weight > 0 {
			w.entries[cnt] = weight
			w.instances[cnt] = e.Instances[idx]
			if cnt > 0 && w.entries[cnt-1] != weight {
				w.balance = false
			}
			w.weightSum += weight
			cnt++
		} else {
			klog.Warnf("KITEX: invalid weight, weight=%d instance=%s", weight, e.Instances[idx].Address())
		}
	}
	w.instances = w.instances[:cnt]
	w.entries = w.entries[:cnt]
	w.picker.immutableInstances = w.instances
	return w
}

func (wb *weightedBalancer) calcWeightInfoExcludeInst(e discovery.Result, addr string) *weightInfo {
	if len(e.Instances) <= 1 || addr == "" {
		return wb.calcWeightInfo(e)
	}
	w := &weightInfo{
		balance:   true,
		entries:   make([]entry, len(e.Instances)),
		instances: make([]discovery.Instance, len(e.Instances)),
		weightSum: 0,
	}
	var cnt int
	for idx := range e.Instances {
		weight := e.Instances[idx].Weight()
		if weight > 0 {
			if e.Instances[idx].Address().String() == addr {
				continue
			}
			w.entries[cnt] = weight
			w.instances[cnt] = e.Instances[idx]
			w.weightSum += weight
			if cnt > 0 && w.entries[cnt-1] != weight {
				w.balance = false
			}
			cnt++
		} else {
			klog.Warnf("KITEX: invalid weight: weight=%d instance=%s", weight, e.Instances[idx].Address())
		}
	}
	w.instances = w.instances[:cnt]
	w.entries = w.entries[:cnt]
	return w
}

// Rebalance implements the Rebalancer interface.
func (wb *weightedBalancer) Rebalance(change discovery.Change) {
	if !change.Result.Cacheable {
		return
	}
	wb.cachedWeightInfo.Store(change.Result.CacheKey, wb.calcWeightInfo(change.Result))
}

// Delete implements the Rebalancer interface.
func (wb *weightedBalancer) Delete(change discovery.Change) {
	if !change.Result.Cacheable {
		return
	}
	wb.cachedWeightInfo.Delete(change.Result.CacheKey)
}

func (wb *weightedBalancer) Name() string {
	return "weight_random"
}
