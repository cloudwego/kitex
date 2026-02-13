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

package stream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func Test_contextWithCancelReason(t *testing.T) {
	ctx := context.Background()
	newCtx := NewContextWithCancelReason(ctx)
	test.Assert(t, newCtx.Err() == nil, newCtx.Err())

	ctx, cancel := context.WithCancel(context.Background())
	newCtx = NewContextWithCancelReason(ctx)
	cancel()
	test.Assert(t, newCtx.Err() == context.Canceled, newCtx.Err())

	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	newCtx = NewContextWithCancelReason(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
	test.Assert(t, newCtx.Err() == context.DeadlineExceeded, newCtx.Err())

	ctx, cancelCause := context.WithCancelCause(context.Background())
	newCtx = NewContextWithCancelReason(ctx)
	err := errors.New("test")
	cancelCause(err)
	test.Assert(t, newCtx.Err() == err, newCtx.Err())

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	ctx, cancelCause = context.WithCancelCause(ctx)
	newCtx = NewContextWithCancelReason(ctx)
	cancelCause(err)
	cancel()
	test.Assert(t, newCtx.Err() == err, newCtx.Err())
}
