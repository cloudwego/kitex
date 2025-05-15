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

package generic

import (
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/proto"
)

func TestNewBinaryPbCodec(t *testing.T) {
	codec := newBinaryPbCodec()
	test.Assert(t, codec != nil)
}

func TestBinaryPbCodecGetMessageReaderWriter(t *testing.T) {
	codec := newBinaryPbCodec()

	rw := codec.getMessageReaderWriter()
	_, ok := rw.(*proto.RawReaderWriter)
	test.Assert(t, ok, "should return *proto.RawReaderWriter")
}

func TestBinaryPbCodecName(t *testing.T) {
	codec := newBinaryPbCodec()
	test.Assert(t, codec.Name() == "BinaryPb")
}

func TestBinaryPbCodecReaderWriter(t *testing.T) {
	codec := newBinaryPbCodec()
	rw := codec.getMessageReaderWriter().(*proto.RawReaderWriter)

	test.Assert(t, rw.RawReader != nil)
	test.Assert(t, rw.RawWriter != nil)
}
