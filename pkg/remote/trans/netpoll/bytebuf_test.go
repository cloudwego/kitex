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

package netpoll

import (
	"bytes"
	"testing"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/utils"

	"github.com/cloudwego/kitex/internal/test"
)

// TestByteBuffer test bytebuf write and read success
func TestByteBuffer(t *testing.T) {
	// 1. prepare mock data
	msg := "hello world"

	buf := &bytes.Buffer{}
	// 2. test
	rw := netpoll.NewReadWriter(buf)

	bw := NewBufioxWriter(rw)
	br := NewBufioxReader(rw)

	firstw, err := bw.Malloc(len(msg) * 2)
	test.Assert(t, err == nil, err)
	copy(firstw, msg)
	copy(firstw[len(msg):], msg)
	_, err = bw.WriteBinary(utils.StringToSliceByte(msg + msg + msg))
	test.Assert(t, err == nil, err)

	test.Assert(t, bw.WrittenLen() == len(msg)*5)
	err = bw.Flush()
	test.Assert(t, err == nil, err)
	test.Assert(t, bw.WrittenLen() == 0)

	firstr, err := br.Next(len(msg) * 2)
	test.Assert(t, err == nil, err)
	test.Assert(t, string(firstr) == msg+msg)
	p := make([]byte, len(msg)*3)
	_, err = br.ReadBinary(p)
	test.Assert(t, err == nil, err)
	test.Assert(t, string(p) == msg+msg+msg)

	test.Assert(t, br.ReadLen() == len(msg)*5)

	_ = br.Release(nil)
	test.Assert(t, br.ReadLen() == 0)
}
