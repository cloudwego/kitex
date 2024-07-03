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
	"errors"
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

func TestPrependError(t *testing.T) {
	var ok bool
	ex0 := NewTransportException(1, "world")
	err0 := PrependError("hello ", ex0)
	ex0, ok = err0.(*TransportException)
	test.Assert(t, ok)
	test.Assert(t, ex0.TypeID() == 1)
	test.Assert(t, ex0.Error() == "hello world")

	ex1 := NewProtocolException(2, "world")
	err1 := PrependError("hello ", ex1)
	ex1, ok = err1.(*ProtocolException)
	test.Assert(t, ok)
	test.Assert(t, ex1.TypeID() == 2)
	test.Assert(t, ex1.Error() == "hello world")

	ex2 := NewApplicationException(3, "world")
	err2 := PrependError("hello ", ex2)
	ex2, ok = err2.(*ApplicationException)
	test.Assert(t, ok)
	test.Assert(t, ex2.TypeID() == 3)
	test.Assert(t, ex2.Error() == "hello world")

	err3 := PrependError("hello ", errors.New("world"))
	_, ok = err3.(tException)
	test.Assert(t, !ok)
	test.Assert(t, err3.Error() == "hello world")

	// the code below, it's for compatibility test only.
	// it can be removed in the future along with Read/Write method
	ex9 := thrift.NewTApplicationException(9, "world")
	err9 := PrependError("hello ", ex9)
	ex, ok := err9.(tException)
	test.Assert(t, ok)
	test.Assert(t, ex.TypeId() == 9)
	test.Assert(t, ex.Error() == "hello world")
}
