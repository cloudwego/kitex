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

package bthrift

import (
	"fmt"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	
	"github.com/cloudwego/kitex/internal/test"
)

func TestWriteMessageEnd(t *testing.T) {
	buf := make([]byte, 0)
	test.Assert(t, 0 == Binary.WriteMessageEnd(buf))
}

func TestWriteStructBegin(t *testing.T) {
	buf := make([]byte, 0)
	test.Assert(t, 0 == Binary.WriteStructBegin(buf, ""))
}

func TestWriteStructEnd(t *testing.T) {
	buf := make([]byte, 0)
	test.Assert(t, 0 == Binary.WriteStructEnd(buf))
}

func TestWriteFieldBegin(t *testing.T) {
	buf := make([]byte, 64)
	exceptWs := "080020"
	exceptSize := 3
	wn := Binary.WriteFieldBegin(buf, "", thrift.I32, 32)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != exceptSize || ws != exceptWs {
		panic(fmt.Errorf("WriteFieldBegin[%d][%s] != except[%d][%s]", wn, ws, exceptSize, exceptWs))
	}
}

func TestWriteFieldEnd(t *testing.T) {
	buf := make([]byte, 0)
	test.Assert(t, 0 == Binary.WriteFieldEnd(buf))
}

func TestWriteFieldStop(t *testing.T) {
	buf := make([]byte, 64)
	test.Assert(t, 1 == Binary.WriteFieldStop(buf))
}

func TestWriteMapBegin(t *testing.T) {
	buf := make([]byte, 64)
	exceptWs := "082000000001"
	exceptSize := 6
	wn := Binary.WriteMapBegin(buf, thrift.I32, 32, 1)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != exceptSize || ws != exceptWs {
		panic(fmt.Errorf("WriteMapBegin[%d][%s] != except[%d][%s]", wn, ws, exceptSize, exceptWs))
	}
}

func TestWriteMapEnd(t *testing.T) {
	buf := make([]byte, 64)
	test.Assert(t, 0 == Binary.WriteMapEnd(buf))
}

func TestWriteListBegin(t *testing.T) {
	buf := make([]byte, 128)
	exceptWs := "0800000020"
	exceptSize := 5
	wn := Binary.WriteListBegin(buf, thrift.I32, 32)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != exceptSize || ws != exceptWs {
		panic(fmt.Errorf("WriteListBegin[%d][%s] != except[%d][%s]", wn, ws, exceptSize, exceptWs))
	}
}

func TestWriteListEnd(t *testing.T) {
	buf := make([]byte, 64)
	test.Assert(t, 0 == Binary.WriteListEnd(buf))
}

func TestWriteSetBegin(t *testing.T) {
	buf := make([]byte, 128)
	exceptWs := "0800000020"
	exceptSize := 5
	wn := Binary.WriteSetBegin(buf, thrift.I32, 32)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != exceptSize || ws != exceptWs {
		panic(fmt.Errorf("WriteSetBegin[%d][%s] != except[%d][%s]", wn, ws, exceptSize, exceptWs))
	}
}
