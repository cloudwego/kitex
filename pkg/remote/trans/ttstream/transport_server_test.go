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
	"testing"
	"time"

	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/internal/test"
)

func Test_parseStreamTimeout(t *testing.T) {
	// frame without Stream Timeout
	fr := &Frame{
		streamFrame: streamFrame{
			meta: IntHeader{},
		},
		typ: headerFrameType,
	}
	tm, ok := parseStreamTimeout(fr)
	test.Assert(t, !ok)
	test.Assert(t, tm == 0)

	// frame with valid Stream Timeout
	fr = &Frame{
		streamFrame: streamFrame{
			meta: IntHeader{
				ttheader.StreamTimeout: "1000",
			},
		},
		typ: headerFrameType,
	}
	tm, ok = parseStreamTimeout(fr)
	test.Assert(t, ok)
	test.Assert(t, tm == 1*time.Second, tm)

	// frame with invalid Stream Timeout
	fr = &Frame{
		streamFrame: streamFrame{
			meta: IntHeader{
				ttheader.StreamTimeout: "invalid",
			},
		},
		typ: headerFrameType,
	}
	tm, ok = parseStreamTimeout(fr)
	test.Assert(t, !ok)
	test.Assert(t, tm == 0)
}
