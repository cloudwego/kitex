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
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll/bytebuf"
)

// The original implementation is moved to the sub package bytebuf.
// This file now only contains existing exported symbols to keep the compatibility.

// NewReaderByteBuffer creates a new remote.ByteBuffer using the given netpoll.ZeroCopyReader.
func NewReaderByteBuffer(r netpoll.Reader) remote.ByteBuffer {
	return bytebuf.NewReaderByteBuffer(r)
}

// NewWriterByteBuffer creates a new remote.ByteBuffer using the given netpoll.ZeroCopyWriter.
func NewWriterByteBuffer(w netpoll.Writer) remote.ByteBuffer {
	return bytebuf.NewWriterByteBuffer(w)
}

// NewReaderWriterByteBuffer creates a new remote.ByteBuffer using the given netpoll.ZeroCopyReadWriter.
func NewReaderWriterByteBuffer(rw netpoll.ReadWriter) remote.ByteBuffer {
	return bytebuf.NewReaderWriterByteBuffer(rw)
}
