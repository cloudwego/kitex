/*
 * Copyright 2025 CloudWeGo Authors
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
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func newTestClientStream(ctx context.Context) *clientStream {
	return newClientStream(ctx, mockStreamWriter{}, streamFrame{})
}

func Test_clientStreamStateChange(t *testing.T) {
	t.Run("serverStream close, then RecvMsg/SendMsg returning exception", func(t *testing.T) {
		cliSt := newTestClientStream(context.Background())
		testException := errors.New("test")
		cliSt.close(testException, false, "")
		rErr := cliSt.RecvMsg(cliSt.ctx, nil)
		test.Assert(t, rErr == testException, rErr)
		sErr := cliSt.SendMsg(cliSt.ctx, nil)
		test.Assert(t, sErr == testException, sErr)
	})
	t.Run("serverStream close twice with different exception, RecvMsg/SendMsg returning the first time exception", func(t *testing.T) {
		cliSt := newTestClientStream(context.Background())
		testException1 := errors.New("test1")
		cliSt.close(testException1, false, "")
		testException2 := errors.New("test2")
		cliSt.close(testException2, false, "")

		rErr := cliSt.RecvMsg(cliSt.ctx, nil)
		test.Assert(t, rErr == testException1, rErr)
		sErr := cliSt.SendMsg(cliSt.ctx, nil)
		test.Assert(t, sErr == testException1, sErr)
	})
	t.Run("serverStream close concurrently with RecvMsg/SendMsg", func(t *testing.T) {
		cliSt := newTestClientStream(context.Background())
		var wg sync.WaitGroup
		wg.Add(2)
		testException := errors.New("test")
		go func() {
			time.Sleep(100 * time.Millisecond)
			cliSt.close(testException, false, "")
		}()
		go func() {
			defer wg.Done()
			rErr := cliSt.RecvMsg(cliSt.ctx, nil)
			test.Assert(t, rErr == testException, rErr)
		}()
		go func() {
			defer wg.Done()
			for {
				sErr := cliSt.SendMsg(cliSt.ctx, &testRequest{})
				if sErr == nil {
					continue
				}
				test.Assert(t, sErr == testException, sErr)
				break
			}
		}()
		wg.Wait()
	})
}
