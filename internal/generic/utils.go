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

import "github.com/cloudwego/kitex/pkg/serviceinfo"

// BinaryGenericSupportsMultiService checks whether the Binary Generic type corresponding to svcInfo supports
// multiple IDL Service.
// For now, only BinaryThriftGenericV2 and BinaryPbGeneric support.
func BinaryGenericSupportsMultiService(svcInfo *serviceinfo.ServiceInfo) bool {
	if svcInfo == nil || svcInfo.Extra == nil {
		return false
	}
	res, _ := svcInfo.Extra[IsBinaryGeneric].(bool)
	return res
}
