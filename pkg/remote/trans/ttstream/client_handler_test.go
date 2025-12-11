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
	"strconv"
	"testing"
	"time"

	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/internal/test"
)

func Test_injectStreamTimeout(t *testing.T) {
	// ctx without deadline
	ctx := context.Background()
	hd := make(IntHeader)
	tm := injectStreamTimeout(ctx, hd)
	test.Assert(t, tm == 0, tm)
	test.Assert(t, hd[ttheader.StreamTimeout] == "", hd)

	// ctx with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	hd = make(IntHeader)
	tm = injectStreamTimeout(ctx, hd)
	test.Assert(t, tm <= 1*time.Second, tm)
	tmStr, ok := hd[ttheader.StreamTimeout]
	test.Assert(t, ok)
	tmMs, err := strconv.Atoi(tmStr)
	test.Assert(t, err == nil, err)
	test.Assert(t, time.Duration(tmMs)*time.Millisecond < 1*time.Second, tmMs)
}
