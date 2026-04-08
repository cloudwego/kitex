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

import "context"

// contextWithCancelReason implements context.Context
// with Err() retrieving cause err with context.Cause automatically.
// Whether using gRPC or ttstream, the ctx.Err() returns protocol-specific errors rather than context.Canceled or context.DeadlineExceeded.
// When using context.WithCancelCause, an additional layer of encapsulation is still required to avoid breaking changes.
type contextWithCancelReason struct {
	context.Context
}

func (c *contextWithCancelReason) Err() error {
	return context.Cause(c.Context)
}

func NewContextWithCancelReason(ctx context.Context) context.Context {
	return &contextWithCancelReason{Context: ctx}
}
