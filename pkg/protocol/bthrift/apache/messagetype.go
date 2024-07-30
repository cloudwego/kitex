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

package apache

import "github.com/apache/thrift/lib/go/thrift"

// originally from github.com/apache/thrift@v0.13.0/lib/go/thrift/messagetype.go

// Message type constants in the Thrift protocol.
type TMessageType = thrift.TMessageType

const (
	INVALID_TMESSAGE_TYPE TMessageType = 0
	CALL                  TMessageType = 1
	REPLY                 TMessageType = 2
	EXCEPTION             TMessageType = 3
	ONEWAY                TMessageType = 4
)
