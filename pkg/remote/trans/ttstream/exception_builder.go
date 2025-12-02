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
	"fmt"
	"time"

	"github.com/cloudwego/kitex/pkg/remote"
	streaming_types "github.com/cloudwego/kitex/pkg/streaming/types"
)

// NewTimeoutException builds ttstream timeout Exception based on
// errStreamTimeout, errStreamRecvTimeout or errStreamSendTimeout.
// If passing non-defined streaming_types.TimeoutType or remote.RPCRole, nil result would be returned.
func NewTimeoutException(tmType streaming_types.TimeoutType, role remote.RPCRole, tm time.Duration) *Exception {
	var baseEx *Exception
	switch tmType {
	case streaming_types.StreamRecvTimeout:
		baseEx = errStreamRecvTimeout
	case streaming_types.StreamSendTimeout:
		baseEx = errStreamSendTimeout
	case streaming_types.StreamTimeout:
		baseEx = errStreamTimeout
	default:
		return nil
	}

	var side sideType
	switch role {
	case remote.Client:
		side = clientSide
	case remote.Server:
		side = serverSide
	default:
		return nil
	}

	return newTimeoutException(baseEx, side, tm)
}

func newTimeoutException(baseEx *Exception, side sideType, tm time.Duration) *Exception {
	return baseEx.newBuilder().withSide(side).withCause(fmt.Errorf("%s, timeout config=%v", baseEx.message, tm))
}
