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
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
)

func TestShareQueue(t *testing.T) {
	// check success
	var (
		sum     int32
		flushed int32
	)
	var deal DealBufferGetters = func(gts []BufferGetter) {
		for _, gt := range gts {
			buf, isNil := gt()
			test.Assert(t, isNil == false)
			test.Assert(t, buf.MallocLen() == 11, buf.MallocLen())
		}
		atomic.AddInt32(&sum, -int32(len(gts)))
	}
	var flush FlushBufferGetters = func() {
		atomic.StoreInt32(&flushed, 1)
	}

	var count int32 = 9
	queue := newSharedQueue(4, deal, flush)
	atomic.AddInt32(&sum, count)
	for i := 0; i < int(count); i++ {
		var getter BufferGetter = func() (buf remote.ByteBuffer, isNil bool) {
			buf = remote.NewWriterBuffer(11)
			buf.Malloc(11)
			return buf, false
		}
		queue.Add(getter)
	}
	// wait for deal all
	for atomic.LoadInt32(&sum) > 0 {
		runtime.Gosched()
	}
	test.Assert(t, atomic.LoadInt32(&flushed) == 1)

	// check fail
	var deal2 DealBufferGetters = func(gts []BufferGetter) {
		for _, gt := range gts {
			test.Assert(t, gt == nil)
		}
		atomic.AddInt32(&sum, -int32(len(gts)))
	}
	queue2 := newSharedQueue(4, deal2, nil)
	atomic.AddInt32(&sum, 1)
	queue2.Add(nil)
	for atomic.LoadInt32(&sum) > 0 {
		runtime.Gosched()
	}
}

func BenchmarkShareQueue(b *testing.B) {
	var sum int32
	conn := remote.NewReaderWriterBuffer(0)
	var testGetter BufferGetter = func() (buf remote.ByteBuffer, isNil bool) {
		return remote.NewWriterBuffer(1024), false
	}
	var deal DealBufferGetters = func(gts []BufferGetter) {
		for _, gt := range gts {
			buf, isNil := gt()
			if !isNil {
				_, err := conn.AppendBuffer(buf)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
		atomic.AddInt32(&sum, -int32(len(gts)))
	}
	var flush FlushBufferGetters = func() {
		conn.Flush()
	}
	queue := newSharedQueue(32, deal, flush)

	// benchmark
	b.ReportAllocs()
	b.SetParallelism(1024)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atomic.AddInt32(&sum, 1)
			queue.Add(testGetter)
		}
	})
	for atomic.LoadInt32(&sum) > 0 {
		runtime.Gosched()
	}
}
