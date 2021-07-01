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

package transmeta

// key of http egress meta
const (
	HTTPDestService     = "destination-service"
	HTTPDestCluster     = "destination-cluster"
	HTTPDestIDC         = "destination-idc"
	HTTPDestEnv         = "destination-env"
	HTTPDestAddr        = "destination-addr"
	HTTPDestConnTimeout = "destination-connect-timeout"
	HTTPDestReqTimeout  = "destination-request-timeout"
	HTTPSourceService   = "source-service"
	HTTPSourceCluster   = "source-cluster"
	HTTPSourceIDC       = "source-idc"
	HTTPSourceEnv       = "source-env"
	HTTPRequestHash     = "http-request-hash"
	HTTPLogID           = "x-tt-logid"
	HTTPStress          = "x-tt-stress"
	HTTPFilterType      = "bytemesh-filter-type"
	// supply metadata
	HTTPDestMethod      = "destination-method"
	HTTPSourceMethod    = "source-method"
	HTTPSpanTag         = "span-tag"
	HTTPRawRingHashKey  = "raw-ring-hash-key"
	HTTPLoadBalanceType = "lb-type"
)

// value of http egress meta
const (
	RPCFilter  = "rpc"
	GRPCFilter = "grpc"
)
