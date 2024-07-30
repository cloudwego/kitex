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

package apache

import (
	"bytes"
	"context"
)

// originally from github.com/apache/thrift@v0.13.0/lib/go/thrift/memory_buffer.go

// Memory buffer-based implementation of the TTransport interface.
type TMemoryBuffer struct {
	*bytes.Buffer
	size int
}

func NewTMemoryBufferLen(size int) *TMemoryBuffer {
	buf := make([]byte, 0, size)
	return &TMemoryBuffer{Buffer: bytes.NewBuffer(buf), size: size}
}

func (p *TMemoryBuffer) IsOpen() bool {
	return true
}

func (p *TMemoryBuffer) Open() error {
	return nil
}

func (p *TMemoryBuffer) Close() error {
	p.Buffer.Reset()
	return nil
}

// Flushing a memory buffer is a no-op
func (p *TMemoryBuffer) Flush(ctx context.Context) error {
	return nil
}

func (p *TMemoryBuffer) RemainingBytes() (num_bytes uint64) {
	return uint64(p.Buffer.Len())
}
