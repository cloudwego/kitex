/*
 * Copyright 2022 CloudWeGo Authors
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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestControlBuf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cb := newControlBuffer(ctx.Done())

	// test put()
	testItem := &ping{}
	err := cb.put(testItem)
	test.Assert(t, err == nil, err)

	// test get() with block
	item, err := cb.get(true)
	test.Assert(t, err == nil, err)
	test.Assert(t, item == testItem, err)

	// test get() with no block
	item, err = cb.get(false)
	test.Assert(t, err == nil, err)
	test.Assert(t, item == nil, err)

	// test executeAndPut()
	success, err := cb.execute(func(it interface{}) bool {
		return false
	}, &ping{})

	test.Assert(t, err == nil, err)
	test.Assert(t, !success, err)

	// test throttle() mock a lot of response frame so throttle() will block current goroutine
	exceedSize := 5
	for i := 0; i < maxQueuedTransportResponseFrames+exceedSize; i++ {
		err := cb.put(&ping{})
		test.Assert(t, err == nil, err)
	}

	// start a new goroutine to consume response frame
	go func() {
		time.Sleep(time.Millisecond * 100)
		for i := 0; i < exceedSize+1; i++ {
			it, err := cb.get(false)
			test.Assert(t, err == nil, err)
			test.Assert(t, it != nil)
		}
	}()

	cb.throttle()
	// consumes all of the frames
	for {
		it, err := cb.get(false)
		if err != nil || it == nil {
			break
		}
	}

	finishErr := errors.New("finish")
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			var block bool
			cb.mu.Lock()
			block = cb.consumerWaiting
			cb.mu.Unlock()
			if block {
				cb.finish(finishErr)
				cancel()
				return
			}
		}
	}()
	item, err = cb.get(true)
	test.Assert(t, err == finishErr, err)
	test.Assert(t, item == nil, item)

	err = cb.put(testItem)
	test.Assert(t, err == finishErr, err)
	_, err = cb.get(false)
	test.Assert(t, err == finishErr, err)
}
