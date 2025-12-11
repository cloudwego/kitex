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

package types

import (
	"math"
	"strconv"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestIsStreamingTimeout(t *testing.T) {
	testcases := []struct {
		tmType    TimeoutType
		expectRes bool
	}{
		{
			tmType:    StreamTimeout,
			expectRes: true,
		},
		{
			tmType:    StreamRecvTimeout,
			expectRes: true,
		},
		{
			tmType:    StreamSendTimeout,
			expectRes: true,
		},
		{
			tmType:    TimeoutType(0),
			expectRes: false,
		},
		{
			tmType:    TimeoutType(math.MaxUint8),
			expectRes: false,
		},
	}

	for _, tc := range testcases {
		t.Run(strconv.Itoa(int(tc.tmType)), func(t *testing.T) {
			res := IsStreamingTimeout(tc.tmType)
			test.Assert(t, res == tc.expectRes, res)
		})
	}
}
