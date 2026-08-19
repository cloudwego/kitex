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
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
)

func TestContextWithCancelReason(t *testing.T) {
	ctx0, cancel0 := context.WithCancel(context.Background())
	ctx, cancel := newContextWithCancelReason(ctx0, cancel0)

	// cancel contextWithCancelReason
	expectErr := errors.New("testing")
	cancel(expectErr)
	test.Assert(t, ctx0.Err() == context.Canceled)
	test.Assert(t, ctx.Err() == expectErr)

	// cancel underlying context
	ctx0, cancel0 = context.WithCancel(context.Background())
	ctx, _ = newContextWithCancelReason(ctx0, cancel0)
	cancel0()
	test.Assert(t, ctx0.Err() == context.Canceled)
	test.Assert(t, ctx.Err() == context.Canceled)
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

func TestClientContextErrDerivedContextLimitation(t *testing.T) {
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

			// Standard derived contexts expose context.Canceled instead of the
			// process-local status error.
			test.Assert(t, serverCtx.Err() == reason)
			test.Assert(t, derivedCtx.Err() == context.Canceled)
			test.Assert(t, !status.Convert(cascadeContextErr(derivedCtx.Err())).IsCascadeCancel())
		})
	}
}
