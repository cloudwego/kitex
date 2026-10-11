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

package grpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
)

type misleadingAsError struct{}

func (misleadingAsError) Error() string { return "misleading As" }

func (misleadingAsError) As(any) bool { return true }

type nonComparableError []byte

func (e nonComparableError) Error() string { return string(e) }

func TestContextWithCancelReason(t *testing.T) {
	ctx0, cancel0 := context.WithCancel(context.Background())
	ctx, cancel := newContextWithCancelReason(ctx0, cancel0)

	// cancel contextWithCancelReason
	expectErr := errors.New("testing")
	cancel(expectErr)
	test.Assert(t, ctx0.Err() == context.Canceled)
	test.Assert(t, ctx.Err() == expectErr)
	test.Assert(t, errors.Is(context.Cause(ctx), expectErr))
	var streamCause *streamCancelCause
	test.Assert(t, errors.As(context.Cause(ctx), &streamCause))

	// cancel underlying context
	ctx0, cancel0 = context.WithCancel(context.Background())
	ctx, _ = newContextWithCancelReason(ctx0, cancel0)
	cancel0()
	test.Assert(t, ctx0.Err() == context.Canceled)
	test.Assert(t, ctx.Err() == context.Canceled)
}

func TestContextWithCancelReasonFirstWinsConcurrently(t *testing.T) {
	for i := 0; i < 100; i++ {
		parent, parentCancel := context.WithTimeout(context.Background(), time.Hour)
		ctx, cancel := newContextWithCancelReason(parent, parentCancel)
		reasons := []error{
			status.Err(codes.Canceled, "status reason"),
			errors.New("plain reason"),
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		for j := 0; j < 8; j++ {
			reason := reasons[j%len(reasons)]
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				cancel(reason)
			}()
		}
		close(start)
		wg.Wait()

		cause, ok := context.Cause(ctx).(*streamCancelCause)
		if !ok || cause == nil {
			t.Fatalf("iteration %d: cascade cause was lost: %T(%v)", i, context.Cause(ctx), context.Cause(ctx))
		}
		if got := ctx.Err(); got != cause.err {
			t.Fatalf("iteration %d: Err and Cause disagree: Err=%T(%v), Cause=%T(%v)", i, got, got, cause.err, cause.err)
		}
		if cause.err != reasons[0] && cause.err != reasons[1] {
			t.Fatalf("iteration %d: unexpected winning reason: %T(%v)", i, cause.err, cause.err)
		}
		if parent.Err() != context.Canceled {
			t.Fatalf("iteration %d: cleanup cancel was not called: %v", i, parent.Err())
		}
	}
}

func TestContextWithCancelReasonFirstNilWins(t *testing.T) {
	ctx, cancel := newContextWithCancelReason(context.Background(), nil)
	cancel(nil)
	cancel(status.Err(codes.Canceled, "late reason"))

	<-ctx.Done()
	test.Assert(t, ctx.Err() == context.Canceled)
	test.Assert(t, context.Cause(ctx) == context.Canceled)
}

func TestContextWithCancelReasonAcceptsNonComparableError(t *testing.T) {
	ctx, cancel := newContextWithCancelReason(context.Background(), nil)
	cancel(nonComparableError("non-comparable reason"))
	cancel(errors.New("late reason of another type"))

	<-ctx.Done()
	test.Assert(t, ctx.Err().Error() == "non-comparable reason")
	cause, ok := context.Cause(ctx).(*streamCancelCause)
	test.Assert(t, ok)
	test.Assert(t, cause.err.Error() == "non-comparable reason")
}

func TestClientContextErrCascadeCancel(t *testing.T) {
	reason := status.Err(codes.Canceled, "inbound RPC terminated")

	// The accepted server RPC still observes an ordinary status. It becomes a
	// cascade cancellation only when it terminates a client RPC.
	test.Assert(t, !status.Convert(ContextErr(reason)).IsCascadeCancel())

	err := cascadeContextErr(reason)
	st := status.Convert(err)
	test.Assert(t, st.IsCascadeCancel())
	test.Assert(t, st.Code() == codes.Canceled)
	test.Assert(t, st.Message() == "inbound RPC terminated")

	test.Assert(t, !status.Convert(cascadeContextErr(context.Canceled)).IsCascadeCancel())
	test.Assert(t, !status.Convert(cascadeContextErr(context.DeadlineExceeded)).IsCascadeCancel())
	test.Assert(t, !status.Convert(cascadeContextErr(status.Err(codes.Unavailable, "transport closed"))).IsCascadeCancel())
}

func TestClientContextErrDerivedContext(t *testing.T) {
	for _, tc := range []struct {
		name   string
		derive func(context.Context) (context.Context, context.CancelFunc)
	}{
		{name: "WithCancel", derive: context.WithCancel},
		{name: "WithTimeout", derive: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithTimeout(ctx, time.Hour)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent, parentCancel := context.WithCancel(context.Background())
			serverCtx, cancelWithReason := newContextWithCancelReason(parent, parentCancel)
			derivedCtx, cancelDerived := tc.derive(serverCtx)
			defer cancelDerived()

			reason := status.Err(codes.Canceled, "inbound RPC terminated")
			cancelWithReason(reason)

			select {
			case <-derivedCtx.Done():
			case <-time.After(time.Second):
				t.Fatal("derived context was not canceled")
			}

			// Standard derived contexts keep Err compatible while Cause preserves
			// Kitex's private stream cancellation marker and original status.
			test.Assert(t, serverCtx.Err() == reason)
			test.Assert(t, derivedCtx.Err() == context.Canceled)
			test.Assert(t, errors.Is(context.Cause(derivedCtx), reason))
			var streamCause *streamCancelCause
			test.Assert(t, errors.As(context.Cause(derivedCtx), &streamCause))

			st := status.Convert(cascadeContextErr(contextErrForCascade(derivedCtx)))
			test.Assert(t, st.IsCascadeCancel())
			test.Assert(t, st.Message() == "inbound RPC terminated")
		})
	}
}

func TestClientContextErrUserCancelCauseCompatibility(t *testing.T) {
	t.Run("user status cause wins", func(t *testing.T) {
		serverCtx, cancelServer := newContextWithCancelReason(context.Background(), nil)
		userCtx, cancelUser := context.WithCancelCause(serverCtx)

		userReason := status.Err(codes.Canceled, "user cancellation")
		cancelUser(userReason)
		cancelServer(status.Err(codes.Canceled, "inbound RPC terminated"))

		<-userCtx.Done()
		test.Assert(t, userCtx.Err() == context.Canceled)
		test.Assert(t, context.Cause(userCtx) == userReason)

		// A raw user-provided *status.Error is deliberately ignored. Preserve
		// the pre-change context.Canceled result and do not mark it as cascade.
		st := status.Convert(cascadeContextErr(contextErrForCascade(userCtx)))
		test.Assert(t, !st.IsCascadeCancel())
		test.Assert(t, st.Code() == codes.Canceled)
		test.Assert(t, st.Message() == context.Canceled.Error())
	})

	t.Run("server stream cause wins", func(t *testing.T) {
		serverCtx, cancelServer := newContextWithCancelReason(context.Background(), nil)
		userCtx, cancelUser := context.WithCancelCause(serverCtx)

		serverReason := status.Err(codes.Canceled, "inbound RPC terminated")
		cancelServer(serverReason)
		cancelUser(status.Err(codes.Canceled, "late user cancellation"))

		<-userCtx.Done()
		var streamCause *streamCancelCause
		test.Assert(t, errors.As(context.Cause(userCtx), &streamCause))
		test.Assert(t, errors.Is(context.Cause(userCtx), serverReason))

		st := status.Convert(cascadeContextErr(contextErrForCascade(userCtx)))
		test.Assert(t, st.IsCascadeCancel())
		test.Assert(t, st.Message() == "inbound RPC terminated")
	})

	t.Run("derived deadline wins", func(t *testing.T) {
		serverCtx, cancelServer := newContextWithCancelReason(context.Background(), nil)
		derivedCtx, cancelDerived := context.WithTimeout(serverCtx, 0)
		defer cancelDerived()

		<-derivedCtx.Done()
		cancelServer(status.Err(codes.Canceled, "inbound RPC terminated"))

		test.Assert(t, derivedCtx.Err() == context.DeadlineExceeded)
		test.Assert(t, context.Cause(derivedCtx) == context.DeadlineExceeded)
		st := status.Convert(cascadeContextErr(contextErrForCascade(derivedCtx)))
		test.Assert(t, !st.IsCascadeCancel())
		test.Assert(t, st.Code() == codes.DeadlineExceeded)
	})

	t.Run("custom As cause is ignored", func(t *testing.T) {
		userCtx, cancelUser := context.WithCancelCause(context.Background())
		cancelUser(misleadingAsError{})

		<-userCtx.Done()
		test.Assert(t, contextErrForCascade(userCtx) == context.Canceled)
		st := status.Convert(cascadeContextErr(contextErrForCascade(userCtx)))
		test.Assert(t, !st.IsCascadeCancel())
		test.Assert(t, st.Code() == codes.Canceled)
	})

	t.Run("wrapped stream cause is ignored", func(t *testing.T) {
		serverCtx, cancelServer := newContextWithCancelReason(context.Background(), nil)
		cancelServer(status.Err(codes.Canceled, "inbound RPC terminated"))

		userCtx, cancelUser := context.WithCancelCause(context.Background())
		cancelUser(fmt.Errorf("user wrapper: %w", context.Cause(serverCtx)))

		<-userCtx.Done()
		test.Assert(t, contextErrForCascade(userCtx) == context.Canceled)
		st := status.Convert(cascadeContextErr(contextErrForCascade(userCtx)))
		test.Assert(t, !st.IsCascadeCancel())
		test.Assert(t, st.Code() == codes.Canceled)
	})
}
