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

package streamcall

import (
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/streaming"
)

func TestWithRecvTimeout(t *testing.T) {
	var sb strings.Builder
	callOpts := callopt.CallOptions{}
	testTimeout := 1 * time.Second
	WithRecvTimeout(testTimeout).f(&callOpts, &sb)
	test.Assert(t, callOpts.StreamOptions.RecvTimeout == testTimeout)
	test.Assert(t, sb.String() == "WithRecvTimeout(1s)")
}

func TestWithRecvTimeoutConfig(t *testing.T) {
	// config timeout
	sb := strings.Builder{}
	callOpts := callopt.CallOptions{}
	WithRecvTimeoutConfig(streaming.TimeoutConfig{Timeout: 1 * time.Second}).f(&callOpts, &sb)
	test.Assert(t, callOpts.StreamOptions.RecvTimeoutConfig.Timeout == 1*time.Second, callOpts)
	test.Assert(t, !callOpts.StreamOptions.RecvTimeoutConfig.DisableCancelRemote, callOpts)
	// config DisableCancelRemote
	sb = strings.Builder{}
	callOpts = callopt.CallOptions{}
	WithRecvTimeoutConfig(streaming.TimeoutConfig{DisableCancelRemote: true}).f(&callOpts, &sb)
	test.Assert(t, callOpts.StreamOptions.RecvTimeoutConfig.Timeout == 0, callOpts)
	test.Assert(t, callOpts.StreamOptions.RecvTimeoutConfig.DisableCancelRemote, callOpts)
	// config timeout and DisableCancelRemote
	sb = strings.Builder{}
	callOpts = callopt.CallOptions{}
	WithRecvTimeoutConfig(streaming.TimeoutConfig{Timeout: 1 * time.Second, DisableCancelRemote: true}).f(&callOpts, &sb)
	test.Assert(t, callOpts.StreamOptions.RecvTimeoutConfig.Timeout == 1*time.Second, callOpts)
	test.Assert(t, callOpts.StreamOptions.RecvTimeoutConfig.DisableCancelRemote, callOpts)
}
