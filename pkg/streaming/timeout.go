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
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/cloudwego/kitex/pkg/kerrors"
)

type TimeoutConfig struct {
	Timeout time.Duration
	// DisableCancelRemote controls whether to cancel the remote peer when Recv/Send timeout.
	// By default, it cancels the remote peer to prevent stream leakage.
	//
	// However, in certain scenarios (e.g., resume-enabled transfers: A → B → C), A may not wish to cancel B and C upon detecting a timeout.
	// Instead, it expects B and C to complete one round of streaming communication and cache the results.
	// This allows A to resume the request from the disconnected point on the next attempt, completing the resume-from-breakpoint process.
	DisableCancelRemote bool
}

// CallWithTimeout executes a function with timeout.
// If timeout is 0, the function will be executed without timeout; panic is not recovered in this case;
// If time runs out, it will return a kerrors.ErrRPCTimeout;
// If your function panics, it will also return a kerrors.ErrRPCTimeout with the panic details;
// Other kinds of errors are always returned by your function.
//
// NOTE: the `cancel` function is necessary to cancel the underlying transport, otherwise the recv/send call
// will block until the transport is closed, which may cause surge of goroutines.
func CallWithTimeout(timeout time.Duration, cancel context.CancelFunc, f func() (err error)) error {
	if timeout <= 0 {
		return f()
	}
	if cancel == nil {
		panic("nil cancel func. Please pass in the cancel func in the context (set in context BEFORE " +
			"the client method call, NOT just before recv/send), to avoid blocking recv/send calls")
	}
	begin := time.Now()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	finishChan := make(chan error, 1) // non-blocking channel to avoid goroutine leak
	gopool.Go(func() {
		var bizErr error
		defer func() {
			if r := recover(); r != nil {
				cancel()
				timeoutErr := fmt.Errorf(
					"CallWithTimeout: panic in business code, timeout_config=%v, panic=%v, stack=%s",
					timeout, r, debug.Stack())
				bizErr = kerrors.ErrRPCTimeout.WithCause(timeoutErr)
			}
			finishChan <- bizErr
		}()
		bizErr = f()
	})
	select {
	case <-timer.C:
		cancel()
		timeoutErr := fmt.Errorf("CallWithTimeout: timeout in business code, timeout_config=%v, actual=%v",
			timeout, time.Since(begin))
		return kerrors.ErrRPCTimeout.WithCause(timeoutErr)
	case bizErr := <-finishChan:
		return bizErr
	}
}
