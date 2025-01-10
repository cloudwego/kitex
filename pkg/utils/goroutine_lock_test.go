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

package utils_test

import (
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/utils/goroutine_lock"
	"github.com/cloudwego/kitex/pkg/utils"
)

func TestGoroutineLockAndUnlock(t *testing.T) {
	t.Parallel()
	startTime := time.Now()
	go func() {
		utils.GoroutineLock()
		time.Sleep(time.Second)
		utils.GoroutineUnlock()
	}()
	goroutinelock.GoroutineWg.Wait()
	diff := time.Since(startTime)
	if diff < time.Second {
		t.Errorf("Expect diff >= 1s, get %v", diff)
	}
}
