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

package client

import (
	"github.com/apache/thrift/lib/go/thrift"
)

// MockTStruct implements the thrift.TStruct interface.
type MockTStruct struct {
	WriteFunc func(p thrift.TProtocol) (e error)
	ReadFunc  func(p thrift.TProtocol) (e error)
}

// Write implements the thrift.TStruct interface.
func (m MockTStruct) Write(p thrift.TProtocol) (e error) {
	if m.WriteFunc != nil {
		return m.WriteFunc(p)
	}
	return
}

// Read implements the thrift.TStruct interface.
func (m MockTStruct) Read(p thrift.TProtocol) (e error) {
	if m.ReadFunc != nil {
		return m.ReadFunc(p)
	}
	return
}
