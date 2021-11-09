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

package netpollmux

import (
	"sync"

	"github.com/cloudwego/kitex/pkg/remote"
)

// EventHandler is used to handle events
type EventHandler interface {
	Recv(bufReader remote.ByteBuffer, err error) error
}

// A concurrent safe <seqID,EventHandler> map
// To avoid lock bottlenecks this map is dived to several (SHARD_COUNT) map shards.
type sharedMap struct {
	size   int32
	shared []*shared
}

// A "thread" safe string to anything map.
type shared struct {
	msgs map[int32]EventHandler
	sync.RWMutex
}

// Creates a new concurrent map.
func newSharedMap(size int32) *sharedMap {
	m := &sharedMap{
		size:   size,
		shared: make([]*shared, size),
	}
	for i := range m.shared {
		m.shared[i] = &shared{
			msgs: make(map[int32]EventHandler),
		}
	}
	return m
}

// getShard returns shard under given seq id
func (m *sharedMap) getShard(seqID int32) *shared {
	return m.shared[abs(seqID)%m.size]
}

// store stores msg under given seq id.
func (m *sharedMap) store(seqID int32, msg EventHandler) {
	if seqID == 0 {
		return
	}
	// Get map shard.
	shard := m.getShard(seqID)
	shard.Lock()
	shard.msgs[seqID] = msg
	shard.Unlock()
}

// load loads the msg under the seq id.
func (m *sharedMap) load(seqID int32) (msg EventHandler, ok bool) {
	if seqID == 0 {
		return nil, false
	}
	shard := m.getShard(seqID)
	shard.RLock()
	msg, ok = shard.msgs[seqID]
	shard.RUnlock()
	return msg, ok
}

// delete deletes the msg under the given seq id.
func (m *sharedMap) delete(seqID int32) {
	if seqID == 0 {
		return
	}
	shard := m.getShard(seqID)
	shard.Lock()
	delete(shard.msgs, seqID)
	shard.Unlock()
}

// rangeMap iterates over the map.
func (m *sharedMap) rangeMap(fn func(seqID int32, msg EventHandler)) {
	for _, shard := range m.shared {
		shard.Lock()
		for k, v := range shard.msgs {
			fn(k, v)
		}
		shard.Unlock()
	}
}

func abs(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}
