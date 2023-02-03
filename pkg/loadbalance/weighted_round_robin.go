/*
 * Copyright 2023 CloudWeGo Authors
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
	"sync"
	"sync/atomic"

	"github.com/bytedance/gopkg/lang/fastrand"

	"github.com/cloudwego/kitex/pkg/discovery"
)

var _ Picker = &WeightedRoundRobinPicker{}
var _ Picker = &RoundRobinPicker{}

const wrrVNodesBatchSize = 500 // it will calculate wrrVNodesBatchSize vnodes when insufficient

type wrrNode struct {
	discovery.Instance
	current int
}

func newWeightedRoundRobinPicker(instances []discovery.Instance) Picker {
	wrrp := new(WeightedRoundRobinPicker)

	// shuffle nodes
	wrrp.size = uint64(len(instances))
	wrrp.nodes = make([]*wrrNode, wrrp.size)
	offset := fastrand.Uint64n(wrrp.size)
	for idx := uint64(0); idx < wrrp.size; idx++ {
		wrrp.nodes[idx] = &wrrNode{
			Instance: instances[(idx+offset)%wrrp.size],
			current:  0,
		}
	}

	// init vnodes
	totalWeight := 0
	for _, node := range instances {
		totalWeight += node.Weight()
	}
	wrrp.vcapacity = uint64(totalWeight)
	wrrp.vnodes = make([]discovery.Instance, wrrp.vcapacity)
	wrrp.buildVirtualWrrNodes(wrrVNodesBatchSize)
	return wrrp
}

// WeightedRoundRobinPicker implement smooth weighted round-robin algorithm.
// Refer from https://github.com/phusion/nginx/commit/27e94984486058d73157038f7950a0a36ecc6e35
type WeightedRoundRobinPicker struct {
	nodes []*wrrNode
	size  uint64

	vindex    uint64
	vsize     uint64
	vcapacity uint64
	vnodes    []discovery.Instance
	vlock     sync.Mutex
}

// Next implements the Picker interface.
func (wp *WeightedRoundRobinPicker) Next(ctx context.Context, request interface{}) (ins discovery.Instance) {
	vindex := atomic.AddUint64(&wp.vindex, 1)
	idx := vindex % wp.vcapacity
	// fast path
	if wp.vnodes[idx] != nil {
		return wp.vnodes[idx]
	}

	// slow path
	wp.vlock.Lock()
	defer wp.vlock.Unlock()
	if wp.vnodes[idx] != nil { // other goroutine has filled the vnodes
		return wp.vnodes[idx]
	}
	vtarget := wp.vsize + wrrVNodesBatchSize
	if idx > vtarget {
		vtarget = idx
	}
	wp.buildVirtualWrrNodes(vtarget)
	return wp.vnodes[idx]
}

func (wp *WeightedRoundRobinPicker) buildVirtualWrrNodes(vtarget uint64) {
	if vtarget > wp.vcapacity {
		vtarget = wp.vcapacity
	}
	for i := wp.vsize; i < vtarget; i++ {
		wp.vnodes[i] = nextWrrNode(wp.nodes).Instance
	}
	wp.vsize = vtarget
}

func nextWrrNode(nodes []*wrrNode) (selected *wrrNode) {
	maxCurrent := 0
	totalWeight := 0
	for _, node := range nodes {
		node.current += node.Weight()
		totalWeight += node.Weight()
		if selected == nil || node.current > maxCurrent {
			selected = node
			maxCurrent = node.current
		}
	}
	if selected == nil {
		return nil
	}
	selected.current -= totalWeight
	return selected
}

// RoundRobinPicker .
type RoundRobinPicker struct {
	cursor    uint64
	size      uint64
	instances []discovery.Instance
}

func newRoundRobinPicker(instances []discovery.Instance) Picker {
	size := uint64(len(instances))
	return &RoundRobinPicker{
		cursor:    fastrand.Uint64n(size),
		size:      size,
		instances: instances,
	}
}

// Next implements the Picker interface.
func (rp *RoundRobinPicker) Next(ctx context.Context, request interface{}) (ins discovery.Instance) {
	if rp.size == 0 {
		return nil
	}
	idx := (atomic.AddUint64(&rp.cursor, 1)) % rp.size
	ins = rp.instances[idx]
	return ins
}
