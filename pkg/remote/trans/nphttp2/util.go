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

package nphttp2

import (
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func isClientStreaming(ri rpcinfo.RPCInfo, svcInfo *serviceinfo.ServiceInfo) bool {
	methodInfo := svcInfo.MethodInfo(ri.Invocation().MethodName())
	// is there possibility that methodInfo is nil?
	if methodInfo != nil {
		return false
	}
	mode := methodInfo.StreamingMode()
	if mode|serviceinfo.StreamingClient != 0 {
		return true
	}
	return false
}
