/*
 * Copyright 2022 ByteDance Inc.
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

package frugal

import (
    `github.com/cloudwego/frugal/internal/binary/decoder`
    `github.com/cloudwego/frugal/internal/binary/encoder`
    `github.com/cloudwego/frugal/iov`
)

// EncodedSize measures the encoded size of val.
func EncodedSize(val interface{}) int {
    return encoder.EncodedSize(val)
}

// EncodeObject serializes val into buf with Thrift Binary Protocol, with optional Zero-Copy iov.BufferWriter.
// buf must be large enough to contain the entire serialization result.
func EncodeObject(buf []byte, mem iov.BufferWriter, val interface{}) (int, error) {
    return encoder.EncodeObject(buf, mem, val)
}

// DecodeObject deserializes buf into val with Thrift Binary Protocol.
func DecodeObject(buf []byte, val interface{}) (int, error) {
    return decoder.DecodeObject(buf, val)
}
