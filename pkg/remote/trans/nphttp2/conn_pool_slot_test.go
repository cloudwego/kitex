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
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
)

var slotTestOpts = grpc.ConnectOptions{
	WriteBufferSize: defaultMockReadWriteBufferSize,
	ReadBufferSize:  defaultMockReadWriteBufferSize,
}

func TestTransportSlot(t *testing.T) {
	t.Run("load() empty slot", func(t *testing.T) {
		slot := &transportSlot{}
		test.Assert(t, slot.load() == nil, slot)
	})
	t.Run("createTransport() and load()", func(t *testing.T) {
		slot := &transportSlot{}
		t.Cleanup(slot.close)
		tr, err := slot.createTransport("test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)
		test.Assert(t, tr != nil)
		test.Assert(t, slot.load() == tr)
	})
	t.Run("createTransport() returns existing active transport", func(t *testing.T) {
		slot := &transportSlot{}
		t.Cleanup(slot.close)
		dialer := newMockDialer()
		tr1, err := slot.createTransport("test", dialer, "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)

		tr2, err := slot.createTransport("test", dialer, "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)
		test.Assert(t, tr1 == tr2, tr1, tr2)
	})
	t.Run("createTransport() after close()", func(t *testing.T) {
		slot := &transportSlot{}
		slot.close()

		_, err := slot.createTransport("test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == errTransportsClosed, err)
	})
	t.Run("removeTransport() clears matching transport", func(t *testing.T) {
		slot := &transportSlot{}
		t.Cleanup(slot.close)
		tr, err := slot.createTransport("test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)

		slot.removeTransport(tr)
		// After removeTransport the slot is empty but the transport itself
		// is still alive. Close it explicitly so the test does not leak it.
		tr.GracefulClose()
		test.Assert(t, slot.load() == nil)
	})
	t.Run("removeTransport() ignores non-matching transport", func(t *testing.T) {
		slot1 := &transportSlot{}
		t.Cleanup(slot1.close)
		tr1, err := slot1.createTransport("test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)

		slot2 := &transportSlot{}
		t.Cleanup(slot2.close)
		tr2, err := slot2.createTransport("test", newMockDialer(), "tcp", mockAddr1, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)

		slot1.removeTransport(tr2)
		test.Assert(t, slot1.load() == tr1, "should not remove non-matching transport")
	})
	t.Run("close() is idempotent", func(t *testing.T) {
		slot := &transportSlot{}
		_, err := slot.createTransport("test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)
		test.Assert(t, slot.load() != nil)

		slot.close()
		test.Assert(t, slot.load() == nil)

		slot.close()
		test.Assert(t, slot.load() == nil)
	})
	t.Run("close() then removeTransport()", func(t *testing.T) {
		slot := &transportSlot{}
		tr, err := slot.createTransport("test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)

		slot.close()
		slot.removeTransport(tr)
	})
	t.Run("concurrent createTransport() and close()", func(t *testing.T) {
		nums := 50
		var wg sync.WaitGroup
		wg.Add(nums + 1)

		slot := &transportSlot{}
		t.Cleanup(slot.close)

		var nilTr, unexpectedErrs atomic.Int32
		for i := 0; i < nums; i++ {
			go func() {
				defer wg.Done()
				tr, err := slot.createTransport("test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
				if err == nil {
					if tr == nil {
						nilTr.Add(1)
					}
				} else if err != errTransportsClosed {
					unexpectedErrs.Add(1)
				}
			}()
		}

		// concurrent close
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			slot.close()
		}()

		wg.Wait()
		test.Assert(t, slot.load() == nil)
		test.Assert(t, nilTr.Load() == 0, nilTr.Load())
		test.Assert(t, unexpectedErrs.Load() == 0, unexpectedErrs.Load())
	})
}

func TestTransports(t *testing.T) {
	t.Run("newTransports()", func(t *testing.T) {
		ts := newTransports(4)
		test.Assert(t, ts.size == 4)
		test.Assert(t, len(ts.transportSlots) == 4)
		test.Assert(t, len(ts.loadAll()) == 0)
	})
	t.Run("getActiveTransport() on empty transports", func(t *testing.T) {
		ts := newTransports(4)
		tr, _ := ts.getActiveTransport()
		test.Assert(t, tr == nil)
	})
	t.Run("createTransport() and loadAll()", func(t *testing.T) {
		ts := newTransports(4)
		t.Cleanup(ts.close)
		tr, err := ts.createTransport(0, "test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)
		test.Assert(t, tr != nil)

		all := ts.loadAll()
		test.Assert(t, len(all) == 1)
		test.Assert(t, all[0] == tr)
	})
	t.Run("close clears all slots", func(t *testing.T) {
		ts := newTransports(4)
		for i := uint32(0); i < 2; i++ {
			tr, err := ts.createTransport(i, "test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
			test.Assert(t, err == nil, err)
			test.Assert(t, tr != nil)
		}
		test.Assert(t, len(ts.loadAll()) == 2)

		ts.close()
		test.Assert(t, len(ts.loadAll()) == 0)

		// createTransport after close should fail
		_, err := ts.createTransport(0, "test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == errTransportsClosed, err)
	})
	t.Run("onClose callback removes transport from slot", func(t *testing.T) {
		ts := newTransports(4)
		t.Cleanup(ts.close)
		tr, err := ts.createTransport(0, "test", newMockDialer(), "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)
		test.Assert(t, len(ts.loadAll()) == 1)

		// simulate transport close (triggers onClose -> removeTransport)
		tr.Close(errors.New("test close"))

		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if len(ts.loadAll()) == 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		test.Assert(t, len(ts.loadAll()) == 0, len(ts.loadAll()))
	})
	t.Run("onGoAway callback removes transport from slot", func(t *testing.T) {
		// Dialer queues a GOAWAY frame after the initial SETTINGS frame and
		// enables autoStartReader so the http2Client reader goroutine runs.
		// The reader parses GOAWAY, handleGoAway fires onGoAway, and onGoAway
		// calls slot.removeTransport, clearing the slot.
		dialFunc := func(network, address string, timeout time.Duration) (net.Conn, error) {
			npConn := newMockNpConn(address)
			npConn.autoStartReader = true
			npConn.mockSettingFrame()
			npConn.mockGoAwayFrame()
			return npConn, nil
		}
		dialer := newMockDialerWithDialFunc(dialFunc)

		ts := newTransports(4)
		t.Cleanup(ts.close)
		_, err := ts.createTransport(0, "test", dialer, "tcp", mockAddr0, time.Second, slotTestOpts)
		test.Assert(t, err == nil, err)

		// Wait for the reader goroutine to parse GOAWAY and fire onGoAway.
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if len(ts.loadAll()) == 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		test.Assert(t, len(ts.loadAll()) == 0, "onGoAway should have removed the transport")
	})
}
