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

package args

import (
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func Test_versionSatisfied(t *testing.T) {
	t.Run("lower", func(t *testing.T) {
		test.Assert(t, !versionSatisfied("v1.0.0", "v1.0.1"))
		test.Assert(t, !versionSatisfied("v1.0.1", "v1.1.0"))
		test.Assert(t, !versionSatisfied("v1.1.1", "v2.0.0"))
	})

	t.Run("equal", func(t *testing.T) {
		test.Assert(t, versionSatisfied("v1.2.3", "v1.2.3"))
	})

	t.Run("higher", func(t *testing.T) {
		test.Assert(t, versionSatisfied("v1.2.4", "v1.2.3"))
		test.Assert(t, versionSatisfied("v1.2.3", "v1.1.4"))
		test.Assert(t, versionSatisfied("v2.2.3", "v1.3.4"))
	})

	t.Run("suffix", func(t *testing.T) {
		test.Assert(t, !versionSatisfied("v1.2.3", "v1.2.3-rc1")) // required shouldn't have suffix
		test.Assert(t, !versionSatisfied("v1.2.3-rc1", "v1.2.3")) // suffix means lower
	})
	t.Run("no prefix", func(t *testing.T) {
		test.Assert(t, versionSatisfied("v1.2.3", "1.2.3"))
		test.Assert(t, versionSatisfied("1.2.3", "v1.2.3"))
		test.Assert(t, versionSatisfied("1.2.3", "1.2.3"))
		test.Assert(t, !versionSatisfied("v1.2.3", "1.2.4"))
		test.Assert(t, !versionSatisfied("1.2.3", "v1.2.4"))
		test.Assert(t, !versionSatisfied("1.2.3", "1.2.4"))
	})
}
