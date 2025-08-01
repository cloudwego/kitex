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

package singleflightutil

import "golang.org/x/sync/singleflight"

// Group is a wrapper around singleflight.Group to provide a CheckAndDo functionality.
// It is used to ensure that `fn` is executed only once for a given key.
// Note: make sure `checkFn` returns the expected value which should be the value set by previous `fn`.
type Group struct {
	singleflight.Group
}

// CheckAndDo implements a double-checked pattern to ensure that `fn` is executed only once for a given key.
// It performs an initial check with `checkFn` before calling `singleflight.Do`. If the value is found, return.
// Otherwise, call `Do` and execute `checkFn` again before executing `fn`.
func (g *Group) CheckAndDo(key string, checkFn func() (any, bool), fn func() (any, error)) (v any, err error, shared bool) {
	if value, loaded := checkFn(); loaded {
		return value, nil, true
	}
	return g.Do(key, func() (any, error) {
		if value, loaded := checkFn(); loaded {
			return value, nil
		}
		return fn()
	})
}
