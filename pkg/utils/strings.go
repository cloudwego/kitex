/*
 * Copyright 2023 CloudWeGo Authors
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

package utils

import (
	"strings"
	"sync"
	"unsafe"
)

func StringDeepCopy(s string) string {
	buf := []byte(s)
	ns := (*string)(unsafe.Pointer(&buf))
	return *ns
}

// StringBuilder is a concurrently safe wrapper for strings.Builder.
type StringBuilder struct {
	sync.Mutex
	sb strings.Builder
}

func (b *StringBuilder) String() (s string) {
	b.Lock()
	s = b.sb.String()
	b.Unlock()
	return
}

func (b *StringBuilder) Len() (l int) {
	b.Lock()
	l = b.sb.Len()
	b.Unlock()
	return
}

func (b *StringBuilder) Cap() (l int) {
	b.Lock()
	l = b.sb.Cap()
	b.Unlock()
	return
}

func (b *StringBuilder) Reset() {
	b.Lock()
	b.sb.Reset()
	b.Unlock()
}

func (b *StringBuilder) Grow(n int) {
	b.Lock()
	b.sb.Grow(n)
	b.Unlock()
}

func (b *StringBuilder) Write(p []byte) (n int, err error) {
	b.Lock()
	n, err = b.sb.Write(p)
	b.Unlock()
	return
}

func (b *StringBuilder) WriteByte(c byte) (err error) {
	b.Lock()
	err = b.sb.WriteByte(c)
	b.Unlock()
	return
}

func (b *StringBuilder) WriteRune(r rune) (n int, err error) {
	b.Lock()
	n, err = b.sb.WriteRune(r)
	b.Unlock()
	return
}

func (b *StringBuilder) WriteString(s string) (n int, err error) {
	b.Lock()
	n, err = b.sb.WriteString(s)
	b.Unlock()
	return
}
