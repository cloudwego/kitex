/*
 * Copyright 2024 CloudWeGo Authors
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

package ttstream

import (
	"net"
	"testing"
)

func TestMuxConnTransPoolGetAfterDialError(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	pool := newMuxConnTransPool(MuxConnConfig{PoolSize: 1}).(*muxConnTransPool)
	if trans, err := pool.Get("tcp4", addr); err == nil {
		_ = trans.Close(nil)
		t.Fatal("expected dial to a closed TCP listener to fail")
	}

	// Assert before retrying so a regression fails rather than leaving a
	// goroutine blocked on the mutex indefinitely.
	value, ok := pool.pool.Load(addr)
	if !ok {
		t.Fatal("missing transport list after failed dial")
	}
	list := value.(*muxConnTransList)
	if !list.L.TryLock() {
		t.Fatal("transport list remains locked after a failed dial")
	}
	list.L.Unlock()

	listener, err = net.Listen("tcp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	trans, err := pool.Get("tcp4", addr)
	if err != nil {
		t.Fatalf("retry after the listener started: %v", err)
	}
	t.Cleanup(func() { _ = trans.Close(nil) })
	if !trans.IsActive() {
		t.Fatal("retry returned an inactive transport")
	}
}
