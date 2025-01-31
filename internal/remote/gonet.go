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

package remote

// SetState only used to set the state to connection in gonet
type SetState interface {
	SetState(inactive bool)
}

// IsGonet returns if the remote component is using gonet
type IsGonet interface {
	IsGonet() bool
}

// OnceExecutor defines an interface for an operation that can only be executed once.
type OnceExecutor interface {
	Done() bool // Returns true if the operation has already been executed
	Do() bool   // Executes the operation once; returns false if execution fails
}
