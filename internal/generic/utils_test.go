/*
 * Copyright 2026 CloudWeGo Authors
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

package generic

import (
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestBinaryGenericSupportsMultiService(t *testing.T) {
	t.Run("nil svcInfo", func(t *testing.T) {
		res := BinaryGenericSupportsMultiService(nil)
		test.Assert(t, res == false)
	})

	t.Run("nil extra", func(t *testing.T) {
		svcInfo := &serviceinfo.ServiceInfo{
			ServiceName: "TestService",
			Extra:       nil,
		}
		res := BinaryGenericSupportsMultiService(svcInfo)
		test.Assert(t, res == false)
	})

	t.Run("empty extra", func(t *testing.T) {
		svcInfo := &serviceinfo.ServiceInfo{
			ServiceName: "TestService",
			Extra:       make(map[string]interface{}),
		}
		res := BinaryGenericSupportsMultiService(svcInfo)
		test.Assert(t, res == false)
	})

	t.Run("IsBinaryGeneric false", func(t *testing.T) {
		svcInfo := &serviceinfo.ServiceInfo{
			ServiceName: "TestService",
			Extra: map[string]interface{}{
				IsBinaryGeneric: false,
			},
		}
		res := BinaryGenericSupportsMultiService(svcInfo)
		test.Assert(t, res == false)
	})

	t.Run("IsBinaryGeneric true", func(t *testing.T) {
		svcInfo := &serviceinfo.ServiceInfo{
			ServiceName: "TestService",
			Extra: map[string]interface{}{
				IsBinaryGeneric: true,
			},
		}
		res := BinaryGenericSupportsMultiService(svcInfo)
		test.Assert(t, res == true)
	})

	t.Run("IsBinaryGeneric wrong type", func(t *testing.T) {
		svcInfo := &serviceinfo.ServiceInfo{
			ServiceName: "TestService",
			Extra: map[string]interface{}{
				IsBinaryGeneric: "not_a_bool",
			},
		}
		res := BinaryGenericSupportsMultiService(svcInfo)
		test.Assert(t, res == false)
	})
}
