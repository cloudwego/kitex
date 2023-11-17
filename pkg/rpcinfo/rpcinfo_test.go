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

package rpcinfo

import (
	"testing"
)

func Test_rpcInfo_checkGid(t *testing.T) {
	ri := NewRPCInfo(nil, nil, nil, nil, nil)
	wait := make(chan interface{}, 1)
	go func() {
		defer func() {
			wait <- recover()
		}()
		ri.To()
	}()
	if err := <-wait; err == nil {
		t.Errorf("no panic, err=%v", err)
	}
}
