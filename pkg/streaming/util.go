/*
 * Copyright 2023 CloudWeGo Authors
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

package streaming

import (
	"fmt"
	"sync"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// KitexUnusedProtection may be anonymously referenced in another package to avoid build error
const KitexUnusedProtection = 0

var userStreamNotImplementingWithDoFinish sync.Once

// UnaryCompatibleMiddleware returns whether to use compatible middleware for unary.
func UnaryCompatibleMiddleware(mode serviceinfo.StreamingMode, allow bool) bool {
	return allow && (mode == serviceinfo.StreamingUnary || mode == serviceinfo.StreamingNone)
}

// FinishStream records the end of stream
// you can call it manually when all business logic is done, and you don't want to call Recv/Send
// for the io.EOF (which triggers the DoFinish automatically).
// Note: if you're to wrap the original stream in a Client middleware, you should also implement
// WithDoFinish in your Stream implementation.
// Deprecated: use FinishClientStream instead, this requires enabling the streamx feature.
func FinishStream(s Stream, err error) {
	if st, ok := s.(WithDoFinish); ok {
		st.DoFinish(err)
		return
	}
	// A gentle (only once) warning for existing implementation of streaming.Stream(s)
	userStreamNotImplementingWithDoFinish.Do(func() {
		klog.Warnf("Failed to record the RPCFinish event, due to"+
			" the stream type [%T] does not implement streaming.WithDoFinish", s)
	})
}

// FinishClientStream records the end of stream
// you can call it manually when all business logic is done, and you don't want to call Recv/Send
// for the io.EOF (which triggers the DoFinish automatically).
// Note: if you're to wrap the original stream in a Client middleware, you should also implement
// WithDoFinish in your ClientStream implementation.
func FinishClientStream(s ClientStream, err error) {
	if st, ok := s.(WithDoFinish); ok {
		st.DoFinish(err)
		return
	}
	// A gentle (only once) warning for existing implementation of streaming.Stream(s)
	userStreamNotImplementingWithDoFinish.Do(func() {
		klog.Warnf("Failed to record the RPCFinish event, due to"+
			" the stream type [%T] does not implement streaming.WithDoFinish", s)
	})
}

// GetServerStream gets streamx.ServerStream from the arg. It's for kitex gen code ONLY.
func GetServerStreamFromArg(arg interface{}) (ServerStream, error) {
	var st ServerStream
	switch rarg := arg.(type) {
	case *Args:
		st = rarg.ServerStream
	case ServerStream:
		st = rarg
	default:
		return nil, fmt.Errorf("cannot get streamx.ServerStream from type: %T", arg)
	}
	return st, nil
}
