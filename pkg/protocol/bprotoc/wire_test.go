/*
 * Copyright 2022 CloudWeGo Authors
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

package bprotoc

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestAppendTag(t *testing.T) {
	buf := make([]byte, 64)
	expect_n := 2
	expect_s := "fa0f"

	n := AppendTag(buf, protowire.Number(255), protowire.BytesType)
	got_s := fmt.Sprintf("%x", buf[:n])
	if n != expect_n || got_s != expect_s {
		panic(fmt.Errorf("AppendTag[%d][%x] != [%d][%s]\n", n, buf[:n], expect_n, expect_s))
	}
}

func TestAppendVarint(t *testing.T) {
	buf := make([]byte, 64)
	expect_n := 5
	expect_s := "ffffffff0f"

	n := AppendVarint(buf, 0xFFFFFFFF)
	got_s := fmt.Sprintf("%x", buf[:n])
	if n != expect_n || got_s != expect_s {
		panic(fmt.Errorf("AppendVarint[%d][%x] != [%d][%s]\n", n, buf[:n], expect_n, expect_s))
	}
}

func TestAppendFixed32(t *testing.T) {
	buf := make([]byte, 64)
	expect_n := 4
	expect_s := "ffffffff"

	n := AppendFixed32(buf, 0xFFFFFFFF)
	got_s := fmt.Sprintf("%x", buf[:n])
	if n != expect_n || got_s != expect_s {
		panic(fmt.Errorf("AppendFixed32[%d][%x] != [%d][%s]\n", n, buf[:n], expect_n, expect_s))
	}
}

func TestAppendFixed64(t *testing.T) {
	buf := make([]byte, 64)
	expect_n := 8
	expect_s := "ffffffff00000000"

	n := AppendFixed64(buf, 0xFFFFFFFF)
	got_s := fmt.Sprintf("%x", buf[:n])
	if n != expect_n || got_s != expect_s {
		panic(fmt.Errorf("AppendFixed64[%d][%x] != [%d][%s]\n", n, buf[:n], expect_n, expect_s))
	}
}

func TestAppendBytes(t *testing.T) {
	buf := make([]byte, 64)
	expect_n := 12
	expect_s := "0b68656c6c6f20776f726c64"

	n := AppendBytes(buf, []byte("hello world"))
	got_s := fmt.Sprintf("%x", buf[:n])
	if n != expect_n || got_s != expect_s {
		panic(fmt.Errorf("AppendBytes[%d][%x] != [%d][%s]\n", n, buf[:n], expect_n, expect_s))
	}
}

func TestAppendString(t *testing.T) {
	buf := make([]byte, 64)
	expect_n := 12
	expect_s := "0b68656c6c6f20776f726c64"

	n := AppendString(buf, "hello world")
	got_s := fmt.Sprintf("%x", buf[:n])
	if n != expect_n || got_s != expect_s {
		panic(fmt.Errorf("AppendString[%d][%x] != [%d][%s]\n", n, buf[:n], expect_n, expect_s))
	}
}

func TestEnforceUTF8(t *testing.T) {
	if !EnforceUTF8() {
		panic(fmt.Errorf("EnforceUTF8 must be true"))
	}
}
