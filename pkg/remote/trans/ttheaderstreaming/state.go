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

package ttheaderstreaming

import "sync/atomic"

const (
	stateNew uint32 = 0
	stateSet uint32 = 1
	received        = stateSet // alias
	sent            = stateSet // alias
	closed          = stateSet
)

type state struct {
	meta    uint32
	header  uint32
	data    uint32
	trailer uint32
	closed  uint32
}

func (s *state) setMeta() (old uint32) {
	return atomic.SwapUint32(&s.meta, stateSet)
}

func (s *state) setHeader() (old uint32) {
	return atomic.SwapUint32(&s.header, stateSet)
}

func (s *state) setData() (old uint32) {
	return atomic.SwapUint32(&s.data, stateSet)
}

func (s *state) setTrailer() (old uint32) {
	return atomic.SwapUint32(&s.trailer, stateSet)
}

func (s *state) setClosed() (old uint32) {
	return atomic.SwapUint32(&s.closed, stateSet)
}

func (s *state) hasMeta() bool {
	return atomic.LoadUint32(&s.meta) == stateSet
}

func (s *state) hasHeader() bool {
	return atomic.LoadUint32(&s.header) == stateSet
}

func (s *state) hasData() bool {
	return atomic.LoadUint32(&s.data) == stateSet
}

func (s *state) hasTrailer() bool {
	return atomic.LoadUint32(&s.trailer) == stateSet
}

func (s *state) isClosed() bool {
	return atomic.LoadUint32(&s.closed) == stateSet
}
