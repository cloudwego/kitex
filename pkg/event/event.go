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

package event

import "time"

// Event represents an event.
type Event struct {
	Name   string
	Time   time.Time
	Detail string
	Extra  interface{}
}

// lazyExtra is an interface for deferred materialization of Event.Extra.
// Implementations MUST be safe for concurrent read access from multiple goroutines,
// because KitexDumpLazyExtra() is called outside the queue lock.
// In practice this means the struct should be immutable after construction.
//
// Use KitexDumpLazyExtra as the method name to minimize the risk of accidental interface satisfaction.
type lazyExtra interface {
	KitexDumpLazyExtra() interface{}
}
