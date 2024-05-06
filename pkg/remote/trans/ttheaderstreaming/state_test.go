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

import (
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func Test_state_setMeta(t *testing.T) {
	s := state{}
	test.Assert(t, s.setMeta() == stateNew)
	test.Assert(t, s.hasMeta())
	test.Assert(t, s.setMeta() == stateSet)
}

func Test_state_setHeader(t *testing.T) {
	s := state{}
	test.Assert(t, s.setHeader() == stateNew)
	test.Assert(t, s.hasHeader())
	test.Assert(t, s.setHeader() == stateSet)
}

func Test_state_setData(t *testing.T) {
	s := state{}
	test.Assert(t, s.setData() == stateNew)
	test.Assert(t, s.hasData())
	test.Assert(t, s.setData() == stateSet)
}

func Test_state_setTrailer(t *testing.T) {
	s := state{}
	test.Assert(t, s.setTrailer() == stateNew)
	test.Assert(t, s.hasTrailer())
	test.Assert(t, s.setTrailer() == stateSet)
}

func Test_state_setClosed(t *testing.T) {
	s := state{}
	test.Assert(t, s.setClosed() == stateNew)
	test.Assert(t, s.isClosed())
	test.Assert(t, s.setClosed() == stateSet)
}
