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

package streamxcallopt

import (
	"fmt"
	"strings"
	"time"
)

type CallOptions struct {
	RPCTimeout time.Duration
}

type CallOption struct {
	f func(o *CallOptions, di *strings.Builder)
}

type WithCallOption func(o *CallOption)

func WithRPCTimeout(rpcTimeout time.Duration) CallOption {
	return CallOption{f: func(o *CallOptions, di *strings.Builder) {
		di.WriteString(fmt.Sprintf("WithRPCTimeout(%d)", rpcTimeout))
		o.RPCTimeout = rpcTimeout
	}}
}
