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

package bthrift

import (
	"math/bits"
	"sync"
	"sync/atomic"
)

const (
	cachedSpans    = 10
	minCachedBytes = 32 // [32,64) bytes in a same span class
	maxCachedBytes = (minCachedBytes << cachedSpans) - 1
	cachedSpanSize = (minCachedBytes << cachedSpans) * 4
)

var spanClassStart = spanClass(minCachedBytes)

var BCache = newBCache(cachedSpanSize)
var BSpan = newSpan(cachedSpanSize)

type bCache struct {
	spans [cachedSpans]*span
}

func newBCache(spanSize int) *bCache {
	c := new(bCache)
	for i := 0; i < len(c.spans); i++ {
		c.spans[i] = newSpan(spanSize)
	}
	return c
}

func (c *bCache) Make(n int) (p []byte) {
	if n < minCachedBytes || n > maxCachedBytes {
		return make([]byte, n)
	}
	sclass := spanClass(n) - spanClassStart
	p = c.spans[sclass].Make(n)
	return p
}

func (c *bCache) Copy(buf []byte) (p []byte) {
	size := len(buf)
	if size < minCachedBytes || size > maxCachedBytes {
		// make-and-copy should put together to let compiler optimize
		p = make([]byte, size)
		copy(p, buf)
		return p
	}
	sclass := spanClass(size) - spanClassStart
	p = c.spans[sclass].Make(len(buf))
	copy(p, buf)
	return p
}

func newSpan(size int) *span {
	sp := new(span)
	sp.size = uint64(size)
	sp.buffer = make([]byte, 0, size)
	return sp
}

type span struct {
	lock   sync.Mutex
	buffer []byte
	read   uint64 // read index of buffer
	size   uint64 // size of buffer
}

func (b *span) Make(n int) []byte {
	if n < minCachedBytes || n > maxCachedBytes {
		return make([]byte, n)
	}
START:
	read := atomic.AddUint64(&b.read, uint64(n))
	// fast path
	if read <= b.size {
		return b.buffer[read-uint64(n) : read]
	}
	// slow path
	if !b.lock.TryLock() {
		// fallback to make a new byte slice if current goroutine cannot get the lock
		return make([]byte, n)
	}
	// create a buffer
	b.buffer = make([]byte, b.size)
	// reset read index to let other goroutine could consume new buffer
	atomic.StoreUint64(&b.read, 0)
	b.lock.Unlock()

	goto START
}

func (b *span) Copy(buf []byte) (p []byte) {
	size := len(buf)
	p = b.Make(size)
	copy(p, buf)
	return p
}

func spanClass(size int) int {
	if size == 0 {
		return 0
	}
	return bits.Len(uint(size)) // minimum number of bits required to represent x
}
