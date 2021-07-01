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

package rpcinfo

// EndpointBasicInfo should be immutable after created.
type EndpointBasicInfo struct {
	ServiceName string
	Method      string
	Tags        map[string]string
}

// Tag names in EndpointInfo. Notice: These keys just be used for framework.
const (
	ConnResetTag     = "crrst"
	RetryTag         = "retry"
	RetryLastCostTag = "last_cost"
	RetryPrevInstTag = "prev_inst"
	ShmIPCTag        = "shmipc"
	RemoteClosedTag  = "remote_closed"
)

// client HTTP
const (
	// connection full url
	HTTPURL = "http_url"
	// specify host header
	HTTPHost = "http_host"
	// http header for remote message tag
	HTTPHeader = "http_header"
)
