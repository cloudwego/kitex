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

// TimeoutType identifies specific timeout types including Stream, Recv and Send Timeout.
// TTHeader Streaming and gRPC Streaming both support these timeout types.
type TimeoutType uint8

const (
	StreamTimeout TimeoutType = iota + 1
	StreamRecvTimeout
	StreamSendTimeout
)

// IsStreamingTimeout judges whether tmType is pre-defined Streaming TimeoutType
func IsStreamingTimeout(tmType TimeoutType) bool {
	if tmType < StreamTimeout || tmType > StreamSendTimeout {
		return false
	}
	return true
}
