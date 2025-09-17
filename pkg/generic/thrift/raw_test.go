/*
 * Copyright 2025 CloudWeGo Authors
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
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
)

func TestRawReader_Read(t *testing.T) {
	r := NewRawReader()
	b := make([]byte, 1024)
	var off int
	off += thrift.Binary.WriteFieldBegin(b[off:], thrift.STRING, 1)
	off += thrift.Binary.WriteString(b[off:], "hello world")
	off += thrift.Binary.WriteFieldBegin(b[off:], thrift.I32, 2)
	off += thrift.Binary.WriteI32(b[off:], int32(123456))
	off += thrift.Binary.WriteByte(b[off:], thrift.STOP)

	in := bufiox.NewBytesReader(b[:off])
	data, err := r.Read(context.Background(), "method", true, 0, in)
	if err != nil {
		t.Fatal(err)
	}
	nb := []byte(string(b))
	b[0], b[1], b[2] = 0, 0, 0 // reset some bytes
	if !reflect.DeepEqual(data, nb[:off]) {
		t.Fatalf("expect %v, got %v", nb[:off], data)
	}
}
