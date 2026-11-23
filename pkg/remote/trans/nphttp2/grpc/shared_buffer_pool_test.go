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

package grpc

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestReceiveBufferPoolScope(t *testing.T) {
	privatePools := make(map[*bufferPool]bool)
	check := func(pool *bufferPool, shared bool) {
		t.Helper()
		if shared {
			test.Assert(t, pool == sharedBufferPool, "enabled transport must use the shared pool")
			return
		}
		test.Assert(t, pool != nil && pool != sharedBufferPool && !privatePools[pool], "default transport must own its pool")
		privatePools[pool] = true
	}
	for _, serverShared := range []bool{false, true} {
		server := setUpServerOnly(t, 0, &ServerConfig{SharedReceiveBufferPool: serverShared}, suspended)
		t.Cleanup(server.stop)
		for _, clientShared := range []bool{false, true} {
			client := setUpClient(t, server, ConnectOptions{SharedReceiveBufferPool: clientShared})
			t.Cleanup(func() { client.Close(errSelfCloseForTest) })
			check(client.bufferPool, clientShared)
		}
		waitWhileTrue(t, func() (bool, error) {
			server.mu.Lock()
			defer server.mu.Unlock()
			if len(server.conns) != 2 {
				return true, fmt.Errorf("waiting for both server transports, got %d", len(server.conns))
			}
			for conn := range server.conns {
				check(conn.(*http2Server).bufferPool, serverShared)
			}
			return false, nil
		})
	}
}

func TestSharedBufferPoolConcurrentOwnership(t *testing.T) {
	const connections = 32
	var wg sync.WaitGroup
	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pool := receiveBufferPool(true)
			payload := bytes.Repeat([]byte{byte(id)}, 4096)
			for j := 0; j < 100; j++ {
				b := pool.get()
				// The receive path resets its exclusively owned buffer;
				// bufferPool itself does not promise to return empty data.
				b.Reset()
				b.Write(payload)
				if !bytes.Equal(b.Bytes(), payload) {
					t.Errorf("connection %d received another connection's data", id)
				}
				pool.put(b)
			}
		}(i)
	}
	wg.Wait()
}
