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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/streaming"
)

func TestStreamReader(t *testing.T) {
	ctx := context.Background()
	msg := []byte("hello world")
	round := 10000

	// basic IOs
	sio := newStreamReader(ctx, nil)
	doneCh := make(chan struct{})
	go func() {
		for i := 0; i < round; i++ {
			sio.input(msg)
		}
		close(doneCh)
	}()
	for i := 0; i < round; i++ {
		payload, err := sio.output()
		test.Assert(t, err == nil, err)
		test.DeepEqual(t, msg, payload)
	}
	// wait for goroutine exited
	<-doneCh
	// exception IOs
	sio.input(msg)
	targetErr := errors.New("test")
	sio.close(targetErr)
	payload, err := sio.output()
	test.Assert(t, err == nil, err)
	test.DeepEqual(t, msg, payload)
	payload, err = sio.output()
	test.Assert(t, payload == nil, payload)
	test.Assert(t, errors.Is(err, targetErr), err)

	// ctx canceled IOs
	ctx2, cancel := context.WithCancel(context.Background())
	sio = newStreamReader(ctx2, nil)
	sio.input(msg)
	go func() {
		time.Sleep(time.Millisecond * 10)
		cancel()
	}()
	payload, err = sio.output()
	test.Assert(t, err == nil, err)
	test.DeepEqual(t, msg, payload)
	payload, err = sio.output()
	test.Assert(t, payload == nil, payload)
	test.Assert(t, errors.Is(err, context.Canceled), err)
}

func TestStreamReaderOutputWithCtx(t *testing.T) {
	t.Run("per-read ctx timeout triggers callback", func(t *testing.T) {
		ctx := context.Background()
		sio := newStreamReader(ctx, nil)

		readCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		callbackErr := errors.New("per-read timeout")
		payload, err := sio.outputWithCtx(readCtx, func(ctx context.Context) error {
			return callbackErr
		})
		test.Assert(t, payload == nil)
		test.Assert(t, err == callbackErr, err)
	})
	t.Run("per-read ctx timeout without callback returns ctx err", func(t *testing.T) {
		ctx := context.Background()
		sio := newStreamReader(ctx, nil)

		readCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		payload, err := sio.outputWithCtx(readCtx, nil)
		test.Assert(t, payload == nil)
		test.Assert(t, errors.Is(err, context.DeadlineExceeded), err)
	})
	t.Run("canRetry error does not persist - subsequent recv succeeds", func(t *testing.T) {
		ctx := context.Background()
		sio := newStreamReader(ctx, nil)

		canRetryErr := newStreamRecvTimeoutException(streaming.TimeoutConfig{
			Timeout:             50 * time.Millisecond,
			DisableCancelRemote: true,
		})
		test.Assert(t, checkCanRetry(canRetryErr))

		readCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		<-readCtx.Done()

		payload, err := sio.outputWithCtx(readCtx, func(ctx context.Context) error {
			return canRetryErr
		})
		test.Assert(t, payload == nil)
		test.Assert(t, errors.Is(err, kerrors.ErrStreamingTimeout), err)
		test.Assert(t, checkCanRetry(err))

		// After canRetry error, sio.exception should NOT be set
		// so we can still recv data
		msg := []byte("hello after retry")
		sio.input(msg)
		payload, err = sio.output()
		test.Assert(t, err == nil, err)
		test.DeepEqual(t, msg, payload)
	})
	t.Run("canRetry error allows multiple consecutive retries", func(t *testing.T) {
		ctx := context.Background()
		sio := newStreamReader(ctx, nil)

		canRetryErr := newStreamRecvTimeoutException(streaming.TimeoutConfig{
			Timeout:             50 * time.Millisecond,
			DisableCancelRemote: true,
		})

		for i := 0; i < 3; i++ {
			readCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			<-readCtx.Done()

			payload, err := sio.outputWithCtx(readCtx, func(ctx context.Context) error {
				return canRetryErr
			})
			test.Assert(t, payload == nil)
			test.Assert(t, checkCanRetry(err))
			cancel()
		}

		msg := []byte("finally got data")
		sio.input(msg)
		payload, err := sio.output()
		test.Assert(t, err == nil, err)
		test.DeepEqual(t, msg, payload)
	})
	t.Run("non-canRetry error persists - subsequent recv returns same error", func(t *testing.T) {
		ctx := context.Background()
		sio := newStreamReader(ctx, nil)

		normalErr := newStreamRecvTimeoutException(streaming.TimeoutConfig{
			Timeout: 50 * time.Millisecond,
		})
		test.Assert(t, !checkCanRetry(normalErr))

		readCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		<-readCtx.Done()

		payload, err := sio.outputWithCtx(readCtx, func(ctx context.Context) error {
			return normalErr
		})
		test.Assert(t, payload == nil)
		test.Assert(t, errors.Is(err, kerrors.ErrStreamingTimeout), err)

		// After non-canRetry error, sio.exception IS set
		// subsequent recv should return same error even with new data
		msg := []byte("hello after error")
		sio.input(msg)
		payload, err = sio.output()
		test.Assert(t, payload == nil)
		test.Assert(t, errors.Is(err, kerrors.ErrStreamingTimeout), err)
	})
}
