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
	"time"
)

type transmissionTimeoutKey struct{}

type transmissionTimeout struct {
	dur time.Duration
	ddl time.Time
}

// NewCtxWithTransmissionTimeout is used to inject a transmitted timeout into ctx
// to identify the source of timeout
//
// scenarios:
// - gRPC handler
// - TTHeader Streaming handler
// - thrift handler when server.WithEnableContextTimeout is configured
func NewCtxWithTransmissionTimeout(ctx context.Context, dur time.Duration) context.Context {
	ddl := time.Now().Add(dur)
	return context.WithValue(ctx, transmissionTimeoutKey{}, transmissionTimeout{
		dur: dur,
		ddl: ddl,
	})
}

func GetTransmissionTimeout(ctx context.Context) (time.Duration, bool) {
	tm, ok := ctx.Value(transmissionTimeoutKey{}).(transmissionTimeout)
	return tm.dur, ok
}

func GetTransmissionDeadline(ctx context.Context) (time.Time, bool) {
	tm, ok := ctx.Value(transmissionTimeoutKey{}).(transmissionTimeout)
	return tm.ddl, ok
}
