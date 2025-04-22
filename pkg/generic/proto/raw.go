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

package proto

import (
	"context"
)

type RawReaderWriter struct {
	*RawReader
	*RawWriter
}

func NewRawReaderWriter() *RawReaderWriter {
	return &RawReaderWriter{RawReader: NewRawReader(), RawWriter: NewRawWriter()}
}

// NewRawWriter build RawWriter
func NewRawWriter() *RawWriter {
	return &RawWriter{}
}

// RawWriter implement of MessageWriter
type RawWriter struct{}

var _ MessageWriter = (*RawWriter)(nil)

// Write returns the copy of data
func (m *RawWriter) Write(ctx context.Context, msg interface{}, method string, isClient bool) (interface{}, error) {
	return msg, nil
}

// NewRawReader build RawReader
func NewRawReader() *RawReader {
	return &RawReader{}
}

// RawReader implement of MessageReaderWithMethod
type RawReader struct{}

var _ MessageReader = (*RawReader)(nil)

// Read returns the copy of data
func (m *RawReader) Read(ctx context.Context, method string, isClient bool, actualMsgBuf []byte) (interface{}, error) {
	copied := make([]byte, len(actualMsgBuf))
	copy(copied, actualMsgBuf)
	return copied, nil
}
