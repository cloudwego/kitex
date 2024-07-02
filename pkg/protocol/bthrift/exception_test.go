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

package bthrift

import (
	"bytes"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	thrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
)

func TestApplicationException(t *testing.T) {
	ex1 := NewApplicationException(1, "t1")
	b := make([]byte, ex1.BLength())
	n := ex1.FastWrite(b)
	test.Assert(t, n == len(b))

	ex2 := NewApplicationException(0, "")
	n, err := ex2.FastRead(b)
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(b), n)
	test.Assert(t, ex2.TypeID() == 1)
	test.Assert(t, ex2.Msg() == "t1")

	// =================
	// the code below, it's for compatibility test only.
	// it can be removed in the future along with Read/Write method

	trans := thrift.NewTMemoryBufferLen(100)
	proto := thrift.NewTBinaryProtocol(trans, true, true)
	ex9 := thrift.NewTApplicationException(1, "t1")
	err = ex9.Write(proto)
	test.Assert(t, err == nil, err)
	test.Assert(t, bytes.Equal(b, trans.Bytes()))

	trans = thrift.NewTMemoryBufferLen(100)
	proto = thrift.NewTBinaryProtocol(trans, true, true)
	ex3 := NewApplicationException(1, "t1")
	err = ex3.Write(proto)
	test.Assert(t, err == nil, err)
	test.Assert(t, bytes.Equal(b, trans.Bytes()))

	ex4 := NewApplicationException(0, "")
	err = ex4.Read(proto)
	test.Assert(t, err == nil, err)
	test.Assert(t, ex4.TypeID() == 1)
	test.Assert(t, ex4.Msg() == "t1")
}
