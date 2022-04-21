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

package metadata

import (
	"fmt"
	"github.com/cloudwego/kitex/internal/test"
	"golang.org/x/net/context"
	"testing"
)

var (
	key1    = "key1"
	key2    = "key2"
	key3    = "key3"
	key4    = "key4"
	key5    = "key5"
	value1  = "value1"
	value2  = "value2"
	value3  = "value3"
	value11 = "value11"
	value2m = "value2m"
	value4  = "value4"
	value5  = "value5"
	m       = map[string]string{}
	testErr = fmt.Errorf("metadata unit test error")
)

func init() {
	m[key1] = value1
	m[key2] = value2
	m[key3] = value3
}

func TestDecodeKeyValue(t *testing.T) {
	k, v, err := DecodeKeyValue(key1, value1)
	test.Assert(t, k == key1, testErr)
	test.Assert(t, v == value1, testErr)
	test.Assert(t, err == nil, err)
}

func TestPairs(t *testing.T) {

	//new pairs
	md := Pairs(key1, value1)

	v1 := md.Get(key1)[0]
	test.Assert(t, v1 == value1, testErr)

	//invalid parameter length
	defer func() {
		err1 := recover()
		test.Assert(t, err1 != nil, testErr)
	}()
	Pairs(key1)
}

func TestMetadata(t *testing.T) {

	md := New(m)

	// test Len()
	test.Assert(t, md.Len() == len(m), testErr)
	// test Get()
	test.Assert(t, md.Get(key1)[0] == value1, testErr)
	// test Append()
	md.Append(key1)
	test.Assert(t, len(md.Get(key1)) == 1, testErr)
	md.Append(key1, value11)
	test.Assert(t, md.Get(key1)[1] == value11, testErr)
	// test Set()
	md.Set(key2)
	test.Assert(t, md.Get(key2)[0] == value2, testErr)
	md.Set(key2, value2m)
	test.Assert(t, md.Get(key2)[0] == value2m, testErr)
	// test Copy()
	copiedMd := md.Copy()
	test.Assert(t, copiedMd.Get(key3)[0] == value3, testErr)

}

func TestContext(t *testing.T) {
	md := Pairs(key1, value1)

	// test incomingCtx
	incomingCtx := NewIncomingContext(context.Background(), md)
	_, incomingOk := FromIncomingContext(incomingCtx)
	test.Assert(t, incomingOk, testErr)

	// test outgoingCtx
	outgoingCtx := NewOutgoingContext(context.Background(), md)
	_, outgoingOk := FromOutgoingContext(outgoingCtx)
	test.Assert(t, outgoingOk, testErr)

	// test outgoingCtxRaw
	AppendToOutgoingContext(outgoingCtx, key4, value4, key5, value5)
	_, _, outgoingRawOk := FromOutgoingContextRaw(outgoingCtx)
	test.Assert(t, outgoingRawOk, testErr)

}
