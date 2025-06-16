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

package env

import (
	"os"
	"strconv"
)

// UseProtoc returns true if we'd like to use protoc instead of prutal.
//
// protoc will be deprecated in the future, and this func will be removed.
func UseProtoc() bool {
	v := os.Getenv("KITEX_TOOL_USE_PROTOC")
	if v == "" {
		return false // false by default
	}
	ok, _ := strconv.ParseBool(v)
	return ok
}

func UsePrutalMarshal() bool {
	if !UseProtoc() {
		return true // if not using protoc, can only use prutal
	}
	v := os.Getenv("KITEX_TOOL_USE_PRUTAL_MARSHAL")
	if v == "" {
		return false // false by default
	}
	ok, _ := strconv.ParseBool(v)
	return ok
}
