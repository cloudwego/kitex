// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package netpoll

import (
	"sync/atomic"

	"github.com/bytedance/gopkg/lang/fastrand"
)

// LoadBalance sets the load balancing method.
type LoadBalance int

const (
	// Random requests that connections are randomly distributed.
	Random LoadBalance = iota
	// RoundRobin requests that connections are distributed to a Poll
	// in a round-robin fashion.
	RoundRobin
)

// loadbalance sets the load balancing method for []*polls
type loadbalance interface {
	LoadBalance() LoadBalance
	// Choose the most qualified Poll
	Pick() (poll Poll)

	Rebalance(polls []Poll)
}

func newLoadbalance(lb LoadBalance, polls []Poll) loadbalance {
	switch lb {
	case Random:
		return newRandomLB(polls)
	case RoundRobin:
		return newRoundRobinLB(polls)
	}
	return newRoundRobinLB(polls)
}

func newRandomLB(polls []Poll) loadbalance {
	return &randomLB{polls: polls, pollSize: len(polls)}
}

type randomLB struct {
	polls    []Poll
	pollSize int
}

func (b *randomLB) LoadBalance() LoadBalance {
	return Random
}

func (b *randomLB) Pick() (poll Poll) {
	idx := fastrand.Intn(b.pollSize)
	return b.polls[idx]
}

func (b *randomLB) Rebalance(polls []Poll) {
	b.polls, b.pollSize = polls, len(polls)
}

func newRoundRobinLB(polls []Poll) loadbalance {
	return &roundRobinLB{polls: polls, pollSize: len(polls)}
}

type roundRobinLB struct {
	polls    []Poll
	accepted uintptr // accept counter
	pollSize int
}

func (b *roundRobinLB) LoadBalance() LoadBalance {
	return RoundRobin
}

func (b *roundRobinLB) Pick() (poll Poll) {
	idx := int(atomic.AddUintptr(&b.accepted, 1)) % b.pollSize
	return b.polls[idx]
}

func (b *roundRobinLB) Rebalance(polls []Poll) {
	b.polls, b.pollSize = polls, len(polls)
}
