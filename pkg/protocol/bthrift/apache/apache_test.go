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

package apache

import (
	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift/internal/test"
	"testing"
)

type mockTStruct struct {
	content string
}

func (m *mockTStruct) Read(iprot thrift.TProtocol) (err error) {
	str, err := iprot.ReadString()
	if err != nil {
		return err
	}
	m.content = str
	return nil
}

func (m *mockTStruct) Write(oprot thrift.TProtocol) (err error) {
	return oprot.WriteString(m.content)
}

func TestCheckTStruct(t *testing.T) {
	test.Assert(t, checkTStruct(&mockTStruct{}) == nil)
	test.Assert(t, checkTStruct(&struct{}{}) != nil)
}

func TestCallThriftReadAndWrite(t *testing.T) {

	var bytes []byte

	s1 := &mockTStruct{content: "test"}
	bw := bufiox.NewBytesWriter(&bytes)
	err := callThriftWrite(bw, s1)
	test.Assert(t, err == nil, err)
	test.Assert(t, bw.Flush() == nil)

	s2 := &mockTStruct{}
	br := bufiox.NewBytesReader(bytes)
	err = callThriftRead(br, s2)
	test.Assert(t, err == nil, err)

	test.Assert(t, s1.content == s2.content)

}
