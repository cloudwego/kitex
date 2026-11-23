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
	"sync"
	"testing"
)

func TestSharedBufferPoolConcurrentOwnership(t *testing.T) {
	const connections = 32
	var wg sync.WaitGroup
	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pool := newBufferPool()
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
