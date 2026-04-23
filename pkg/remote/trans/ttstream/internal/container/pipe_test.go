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

package container

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestPipeRead(t *testing.T) {
	t.Run("concurrent write and read", func(t *testing.T) {
		pipe := NewPipe[int](context.Background(), nil)
		var recv int
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			items := make([]int, 10)
			for {
				n, err := pipe.Read(items)
				if err != nil {
					test.Assert(t, err == io.EOF, err)
					return
				}
				for i := 0; i < n; i++ {
					recv += items[i]
				}
			}
		}()
		round := 10000
		itemsPerRound := []int{1, 1, 1, 1, 1}
		for i := 0; i < round; i++ {
			pipe.Write(itemsPerRound...)
		}
		pipe.Close()
		wg.Wait()
		test.Assert(t, recv == len(itemsPerRound)*round, recv)
	})
	t.Run("write then close then read drains all", func(t *testing.T) {
		pipe := NewPipe[int](context.Background(), nil)
		pipeSize := 10
		var pipeRead int32
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			readBuf := make([]int, 1)
			for {
				n, err := pipe.Read(readBuf)
				if err != nil {
					test.Assert(t, errors.Is(err, io.EOF), err)
					break
				}
				atomic.AddInt32(&pipeRead, int32(n))
			}
		}()
		time.Sleep(10 * time.Millisecond)
		for i := 0; i < pipeSize; i++ {
			pipe.Write(i)
		}
		for atomic.LoadInt32(&pipeRead) != int32(pipeSize) {
			runtime.Gosched()
		}
		pipe.Close()
		wg.Wait()
	})
	t.Run("pipe ctx cancel without callback", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		pipe := NewPipe[int](ctx, nil)

		var readErr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, readErr = pipe.Read(make([]int, 10))
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()
		wg.Wait()
		test.Assert(t, errors.Is(readErr, context.Canceled), readErr)
	})
	t.Run("pipe ctx cancel with callback", func(t *testing.T) {
		pipeWrapper := struct{ pipe *Pipe[int] }{}
		finalNum := 100
		firstNum := 1
		ctx, cancel := context.WithCancel(context.Background())
		pipe := NewPipe[int](ctx, func(ctx context.Context) error {
			pipeWrapper.pipe.Write(finalNum)
			pipeWrapper.pipe.Close()
			return ctx.Err()
		})
		pipeWrapper.pipe = pipe

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			var canceled bool
			readBuf := make([]int, 1)
			for {
				n, rErr := pipe.Read(readBuf)
				if canceled {
					if rErr != nil {
						test.Assert(t, rErr == context.Canceled, rErr)
						return
					}
					test.Assert(t, n == 1, n)
					test.Assert(t, readBuf[0] == finalNum, readBuf[0])
					continue
				}
				test.Assert(t, n == 1, n)
				test.Assert(t, readBuf[0] == firstNum, readBuf[0])
				cancel()
				canceled = true
			}
		}()
		pipe.Write(firstNum)
		wg.Wait()
	})
}

func TestPipeReadCtx(t *testing.T) {
	t.Run("read available data", func(t *testing.T) {
		pipe := NewPipe[int](context.Background(), nil)
		pipe.Write(1, 2, 3, 4, 5)

		items := make([]int, 10)
		n, err := pipe.ReadCtx(context.Background(), nil, items)
		test.Assert(t, err == nil, err)
		test.Assert(t, n == 5, n)
		test.Assert(t, items[0] == 1, items[0])
		test.Assert(t, items[4] == 5, items[4])
		pipe.Close()
	})
	t.Run("perReadCtx cancel without callback", func(t *testing.T) {
		pipe := NewPipe[int](context.Background(), nil)
		readCtx, cancel := context.WithCancel(context.Background())

		var readErr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, readErr = pipe.ReadCtx(readCtx, nil, make([]int, 10))
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()
		wg.Wait()
		test.Assert(t, errors.Is(readErr, context.Canceled), readErr)
		pipe.Close()
	})
	t.Run("perReadCtx cancel with callback", func(t *testing.T) {
		pipe := NewPipe[int](context.Background(), nil)
		readCtx, cancel := context.WithCancel(context.Background())
		var callbackCalled bool
		callback := func(ctx context.Context) error {
			callbackCalled = true
			test.Assert(t, ctx == readCtx, ctx)
			return errors.New("custom callback error")
		}

		var readErr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, readErr = pipe.ReadCtx(readCtx, callback, make([]int, 10))
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()
		wg.Wait()
		test.Assert(t, callbackCalled)
		test.Assert(t, readErr.Error() == "custom callback error", readErr)
		pipe.Close()
	})
	t.Run("pipe ctx cancel triggers pipe callback", func(t *testing.T) {
		var pipeCallbackCalled int32
		pipeCtx, cancelPipe := context.WithCancel(context.Background())
		pipe := NewPipe[int](pipeCtx, func(ctx context.Context) error {
			atomic.AddInt32(&pipeCallbackCalled, 1)
			return errors.New("pipe canceled")
		})

		var perReadCallbackCalled int32
		perReadCallback := func(ctx context.Context) error {
			atomic.AddInt32(&perReadCallbackCalled, 1)
			return errors.New("should not be called")
		}

		var readErr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			// perReadCtx never cancels
			_, readErr = pipe.ReadCtx(context.Background(), perReadCallback, make([]int, 10))
		}()

		time.Sleep(50 * time.Millisecond)
		cancelPipe()
		wg.Wait()
		test.Assert(t, readErr.Error() == "pipe canceled", readErr)
		test.Assert(t, atomic.LoadInt32(&pipeCallbackCalled) == 1, atomic.LoadInt32(&pipeCallbackCalled))
		test.Assert(t, atomic.LoadInt32(&perReadCallbackCalled) == 0, atomic.LoadInt32(&perReadCallbackCalled))
	})
	t.Run("perReadCtx timeout triggers perReadCallback", func(t *testing.T) {
		var pipeCallbackCalled int32
		// pipe ctx never cancels
		pipe := NewPipe[int](context.Background(), func(ctx context.Context) error {
			atomic.AddInt32(&pipeCallbackCalled, 1)
			return errors.New("should not be called")
		})

		var perReadCallbackCalled int32
		perReadCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		perReadCallback := func(ctx context.Context) error {
			atomic.AddInt32(&perReadCallbackCalled, 1)
			return errors.New("per-read timeout")
		}

		_, err := pipe.ReadCtx(perReadCtx, perReadCallback, make([]int, 10))
		test.Assert(t, err.Error() == "per-read timeout", err)
		test.Assert(t, atomic.LoadInt32(&perReadCallbackCalled) == 1, atomic.LoadInt32(&perReadCallbackCalled))
		test.Assert(t, atomic.LoadInt32(&pipeCallbackCalled) == 0, atomic.LoadInt32(&pipeCallbackCalled))
		pipe.Close()
	})
	t.Run("write and close atomically drains data before EOF", func(t *testing.T) {
		testCases := []struct {
			name     string
			writeBuf []int
			readSize int
			expect   int32
		}{
			{
				name:     "single item",
				writeBuf: []int{42},
				readSize: 10,
				expect:   42,
			},
			{
				name:     "multiple items",
				writeBuf: []int{1, 2, 3},
				readSize: 1,
				expect:   6,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Verify that when Write + Close happen while reader sees state=closed,
				// the reader still drains remaining items before returning EOF.
				for i := 0; i < 1000; i++ {
					pipe := NewPipe[int](context.Background(), nil)
					var total int32
					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer wg.Done()
						items := make([]int, tc.readSize)
						for {
							n, err := pipe.Read(items)
							for j := 0; j < n; j++ {
								atomic.AddInt32(&total, int32(items[j]))
							}
							if err != nil {
								test.Assert(t, err == io.EOF, err)
								return
							}
						}
					}()

					// Write and Close back-to-back to maximize the chance of triggering the race:
					// reader's first p.read() returns 0, then sees state=closed,
					// but the data written here must still be drained by the second p.read().
					pipe.Write(tc.writeBuf...)
					pipe.Close()
					wg.Wait()
					test.Assert(t, atomic.LoadInt32(&total) == tc.expect, atomic.LoadInt32(&total))
				}
			})
		}
	})
	t.Run("close pipe while ReadCtx is blocked returns EOF", func(t *testing.T) {
		pipe := NewPipe[int](context.Background(), nil)

		var readErr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, readErr = pipe.ReadCtx(context.Background(), nil, make([]int, 10))
		}()

		time.Sleep(50 * time.Millisecond)
		pipe.Close()
		wg.Wait()
		test.Assert(t, readErr == io.EOF, readErr)
	})
}

func BenchmarkPipeline(b *testing.B) {
	pipe := NewPipe[int](context.Background(), nil)
	readCache := make([]int, 8)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(readCache); j++ {
			go pipe.Write(1)
		}
		total := 0
		for total < len(readCache) {
			n, _ := pipe.Read(readCache)
			total += n
		}
	}
}
