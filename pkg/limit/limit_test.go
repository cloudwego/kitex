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

package limit

import (
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestValid(t *testing.T) {
	test.Assert(t, (&Option{MaxConnections: 0, MaxQPS: 0}).Valid() == false)
	test.Assert(t, (&Option{MaxConnections: 1, MaxQPS: 0}).Valid() == true)
	test.Assert(t, (&Option{MaxConnections: 0, MaxQPS: 1}).Valid() == true)
}

type fakeUpdater struct {
	connectionLimit int
	qpsLimit        int
}

func (fu *fakeUpdater) UpdateLimit(opt *Option) (updated bool) {
	if opt == nil {
		return false
	}
	fu.connectionLimit = opt.MaxConnections
	fu.qpsLimit = opt.MaxQPS
	return true
}

func TestUpdateLimitConfig(t *testing.T) {
	fu := &fakeUpdater{}
	opt := DefaultOption()
	opt.UpdateControl(fu)

	updated := opt.UpdateLimitConfig(10, 100)
	test.Assert(t, updated == true)
	test.Assert(t, fu.connectionLimit == 10)
	test.Assert(t, fu.qpsLimit == 100)
}
