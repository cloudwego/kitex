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

package stream

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestTransmissionTimeout(t *testing.T) {
	ctx := context.Background()
	// without transmission timeout
	tm, ok := GetTransmissionTimeout(ctx)
	test.Assert(t, !ok)
	test.Assert(t, tm == 0, tm)
	ddl, ok := GetTransmissionDeadline(ctx)
	test.Assert(t, !ok)
	test.Assert(t, ddl.IsZero())
	// with transmission timeout
	transTm := 100 * time.Millisecond
	ctx = NewCtxWithTransmissionTimeout(ctx, transTm)
	tm, ok = GetTransmissionTimeout(ctx)
	test.Assert(t, ok)
	test.Assert(t, tm == transTm, tm)
	ddl, ok = GetTransmissionDeadline(ctx)
	test.Assert(t, ok)
	test.Assert(t, !ddl.IsZero(), ddl)
}
