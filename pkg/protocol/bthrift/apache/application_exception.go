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

// originally from github.com/apache/thrift@v0.13.0/lib/go/thrift/application_exception.go

const (
	UNKNOWN_APPLICATION_EXCEPTION  = 0
	UNKNOWN_METHOD                 = 1
	INVALID_MESSAGE_TYPE_EXCEPTION = 2
	WRONG_METHOD_NAME              = 3
	BAD_SEQUENCE_ID                = 4
	MISSING_RESULT                 = 5
	INTERNAL_ERROR                 = 6
	PROTOCOL_ERROR                 = 7
	INVALID_TRANSFORM              = 8
	INVALID_PROTOCOL               = 9
	UNSUPPORTED_CLIENT_TYPE        = 10
)

type TApplicationException = thrift.TApplicationException

var NewTApplicationException = thrift.NewTApplicationException
