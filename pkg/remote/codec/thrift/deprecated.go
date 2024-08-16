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

package thrift

import (
	athrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
	"github.com/cloudwego/kitex/pkg/remote"
)

// MessageReader read from athrift.TProtocol
// Deprecated: use github.com/apache/thrift/lib/go/thrift.TStruct
type MessageReader interface {
	Read(iprot athrift.TProtocol) error
}

// MessageWriter write to athrift.TProtocol
// Deprecated: use github.com/apache/thrift/lib/go/thrift.TStruct
type MessageWriter interface {
	Write(oprot athrift.TProtocol) error
}

// UnmarshalThriftException decode thrift exception from tProt
// TODO: this func should be removed in the future. it's exposed accidentally.
// Deprecated: Use `SkipDecoder` + `ApplicationException` of `cloudwego/gopkg/protocol/thrift` instead.
func UnmarshalThriftException(tProt athrift.TProtocol) error {
	return unmarshalThriftException(tProt.Transport())
}

// BinaryProtocol ...
// Deprecated: use github.com/apache/thrift/lib/go/thrift.NewTBinaryProtocol
type BinaryProtocol = athrift.BinaryProtocol

// NewBinaryProtocol ...
// Deprecated: use github.com/apache/thrift/lib/go/thrift.NewTBinaryProtocol
func NewBinaryProtocol(t remote.ByteBuffer) *athrift.BinaryProtocol {
	return athrift.NewBinaryProtocol(t)
}
