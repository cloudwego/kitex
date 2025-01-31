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

package gonet

import (
	"testing"

	"github.com/cloudwego/kitex/internal/test"

	"github.com/cloudwego/kitex/internal/mocks"
)

func TestSvrConn(t *testing.T) {
	closeNum := 0
	c := mocks.Conn{
		ReadFunc: func(b []byte) (n int, err error) {
			copy(b, "1")
			return 1, nil
		},
		CloseFunc: func() (e error) {
			closeNum++
			return nil
		},
	}
	sc := newSvrConn(c)
	test.Assert(t, sc.Reader() == sc.r)
	b := make([]byte, 1)
	n, err := sc.Read(b)
	test.Assert(t, err == nil)
	test.Assert(t, n == len(b))

	// Do and Done
	test.Assert(t, sc.Do())
	test.Assert(t, sc.Done())
	test.Assert(t, !sc.Do())

	// close
	sc.Close()
	test.Assert(t, sc.closed.Load())
	sc.Close()
	test.Assert(t, closeNum == 1)
}
