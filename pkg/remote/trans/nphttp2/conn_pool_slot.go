/*
 * Copyright 2026 CloudWeGo Authors
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

package nphttp2

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
)

const (
	slotClosed int32 = 1
)

// transports holds a fixed-size array of independent transportSlots for a single address.
// Slots are selected by round-robin; each slot manages its own connection independently.
type transports struct {
	index uint32 // atomic round-robin counter
	size  uint32

	transportSlots []*transportSlot
}

func newTransports(size uint32) *transports {
	slots := make([]*transportSlot, size)
	for i := 0; i < int(size); i++ {
		slots[i] = &transportSlot{}
	}

	return &transports{
		size:           size,
		transportSlots: slots,
	}
}

// getNextTransportSlot returns the next slot by round-robin and its index.
func (t *transports) getNextTransportSlot() (*transportSlot, uint32) {
	idx := atomic.AddUint32(&t.index, 1) % t.size
	return t.transportSlots[idx], idx
}

func (t *transports) getTransportSlot(idx uint32) *transportSlot {
	return t.transportSlots[idx]
}

// getActiveTransport picks the next slot by round-robin and returns its transport if active.
// Returns (nil, idx) when the slot is empty or the transport is inactive.
func (t *transports) getActiveTransport() (grpc.ClientTransport, uint32) {
	slot, idx := t.getNextTransportSlot()
	trans := slot.load()
	if checkActive(trans) {
		return trans, idx
	}
	return nil, idx
}

// createTransport creates or reuses transport in the slot at idx.
func (t *transports) createTransport(idx uint32, remoteService string,
	dialer remote.Dialer, network, address string, connectTimeout time.Duration, opts grpc.ConnectOptions,
) (grpc.ClientTransport, error) {
	slot := t.getTransportSlot(idx)
	return slot.createTransport(remoteService, dialer, network, address, connectTimeout, opts)
}

// loadAll returns a snapshot of all non-nil transports across all slots.
func (t *transports) loadAll() []grpc.ClientTransport {
	result := make([]grpc.ClientTransport, 0, t.size)
	for _, slot := range t.transportSlots {
		if tr := slot.load(); tr != nil {
			result = append(result, tr)
		}
	}
	return result
}

// close shuts down all slots. Each slot is closed independently.
func (t *transports) close() {
	for i := 0; i < int(t.size); i++ {
		slot := t.transportSlots[i]
		slot.close()
	}
}

var errTransportsClosed = status.Err(codes.Aborted, "transports have been closed due to instance offline")

// transportRef wraps grpc.ClientTransport for use with atomic.Pointer,
// which requires a non-interface type parameter.
// A nil *transportRef means the slot is empty.
type transportRef struct {
	ct grpc.ClientTransport
}

// transportSlot manages a single gRPC connection with lock-free reads.
type transportSlot struct {
	mu    sync.Mutex // serializes createTransport to prevent redundant dials for the same slot.
	state int32      // closed flag, protected by mu. Once closed, createTransport is rejected

	ref atomic.Pointer[transportRef]
}

func (slot *transportSlot) load() grpc.ClientTransport {
	ref := slot.ref.Load()
	if ref == nil {
		return nil
	}
	return ref.ct
}

// createTransport creates a new gRPC transport and stores it in this slot.
// If the slot already has an active transport, it is returned directly.
// The mutex serializes concurrent callers; network I/O runs with the mutex held
// to prevent redundant dials for the same slot.
func (slot *transportSlot) createTransport(
	remoteService string,
	dialer remote.Dialer, network, address string, connectTimeout time.Duration, opts grpc.ConnectOptions,
) (grpc.ClientTransport, error) {
	slot.mu.Lock()
	if slot.state == slotClosed {
		slot.mu.Unlock()
		return nil, errTransportsClosed
	}

	// Double-check: another goroutine may have filled this slot.
	if ref := slot.ref.Load(); ref != nil && checkActive(ref.ct) {
		slot.mu.Unlock()
		return ref.ct, nil
	}

	newTrans, err := newTransport(remoteService, dialer, network, address, connectTimeout, opts,
		func(ctx context.Context, trans grpc.ClientTransport, reason grpc.GoAwayReason) {
			slot.removeTransport(trans)
		},
		func(ctx context.Context, trans grpc.ClientTransport, err error) {
			slot.removeTransport(trans)
		},
	)
	if err != nil {
		slot.mu.Unlock()
		return nil, err
	}

	oldRef := slot.ref.Swap(&transportRef{ct: newTrans})
	slot.mu.Unlock()

	if oldRef != nil && oldRef.ct != nil {
		oldRef.ct.GracefulClose()
	}

	return newTrans, nil
}

// removeTransport atomically clears this slot if it currently holds the given transport.
// Lock-free (CAS only). Called from onClose/onGoAway callbacks.
func (slot *transportSlot) removeTransport(trans grpc.ClientTransport) {
	ref := slot.ref.Load()
	if ref == nil || ref.ct != trans {
		return
	}
	slot.ref.CompareAndSwap(ref, nil)
}

// close marks this slot as permanently closed and removes any stored transport.
// Idempotent: only the first call performs cleanup.
func (slot *transportSlot) close() {
	slot.mu.Lock()
	if slot.state == slotClosed {
		slot.mu.Unlock()
		return
	}
	slot.state = slotClosed
	ref := slot.ref.Swap(nil)
	slot.mu.Unlock()

	if ref != nil && ref.ct != nil {
		ref.ct.GracefulClose()
	}
}
