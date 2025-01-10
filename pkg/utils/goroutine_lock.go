/*
 * Copyright 2021 CloudWeGo Authors
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

package utils

import goroutinelock "github.com/cloudwego/kitex/internal/utils/goroutine_lock"

// GoroutineLock locks the goroutine so that graceful shutdown will wait until the lock is released
func GoroutineLock() {
	goroutinelock.GoroutineWg.Add(1)
}

// GoroutineUnlock unlocks the goroutine to allow graceful shutdown to continue.
// NOTE: This function should be executed using defer to avoid panic in the middle and causing the lock to not be
// released.
func GoroutineUnlock() {
	goroutinelock.GoroutineWg.Done()
}
