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

package thrift

import (
	"errors"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

const (
	EnableSkipDecoder CodecType = 0b10000
)

// skipBuffer wraps remote.ByteBuffer and reimplement the Next method.
// Next method would not advance the reader in the underlying buffer until Buffer method is called.
// It is used to fit in BinaryProtocol and reduce memory allocation.
type skipBuffer struct {
	remote.ByteBuffer
	readNum int
}

func (b *skipBuffer) Next(n int) ([]byte, error) {
	prev := b.readNum
	next := prev + n
	buf, err := b.ByteBuffer.Peek(next)
	if err != nil {
		return nil, err
	}
	b.readNum = next
	return buf[prev:next], nil
}

func (b *skipBuffer) Buffer() ([]byte, error) {
	return b.ByteBuffer.Next(b.readNum)
}

func newSkipBuffer(bb remote.ByteBuffer) *skipBuffer {
	return &skipBuffer{
		ByteBuffer: bb,
	}
}

// skipDecoder is used to parse the input byte-by-byte and skip the thrift payload
// for making use of Frugal and FastCodec in standard Thrift Binary Protocol scenario.
type skipDecoder struct {
	tprot *BinaryProtocol
	sb    *skipBuffer
}

func newSkipDecoder(trans remote.ByteBuffer) *skipDecoder {
	sb := newSkipBuffer(trans)
	return &skipDecoder{
		tprot: NewBinaryProtocol(sb),
		sb:    sb,
	}
}

func (sd *skipDecoder) SkipStruct() error {
	return sd.skip(thrift.STRUCT, thrift.DEFAULT_RECURSION_DEPTH)
}

func (sd *skipDecoder) skipString() error {
	size, err := sd.tprot.ReadI32()
	if err != nil {
		return err
	}
	if size < 0 {
		return perrors.InvalidDataLength
	}
	_, err = sd.tprot.next(int(size))
	return err
}

func (sd *skipDecoder) skipMap(maxDepth int) error {
	keyTypeId, valTypeId, size, err := sd.tprot.ReadMapBegin()
	if err != nil {
		return err
	}
	for i := 0; i < size; i++ {
		if err = sd.skip(keyTypeId, maxDepth); err != nil {
			return err
		}
		if err = sd.skip(valTypeId, maxDepth); err != nil {
			return err
		}
	}
	return nil
}

func (sd *skipDecoder) skipList(maxDepth int) error {
	elemTypeId, size, err := sd.tprot.ReadListBegin()
	if err != nil {
		return err
	}
	for i := 0; i < size; i++ {
		if err = sd.skip(elemTypeId, maxDepth); err != nil {
			return err
		}
	}
	return nil
}

func (sd *skipDecoder) skipSet(maxDepth int) error {
	return sd.skipList(maxDepth)
}

func (sd *skipDecoder) skip(typeId thrift.TType, maxDepth int) (err error) {
	if maxDepth <= 0 {
		return thrift.NewTProtocolExceptionWithType(thrift.DEPTH_LIMIT, errors.New("depth limit exceeded"))
	}

	switch typeId {
	case thrift.BOOL, thrift.BYTE:
		if _, err = sd.tprot.next(1); err != nil {
			return
		}
	case thrift.I16:
		if _, err = sd.tprot.next(2); err != nil {
			return
		}
	case thrift.I32:
		if _, err = sd.tprot.next(4); err != nil {
			return
		}
	case thrift.I64, thrift.DOUBLE:
		if _, err = sd.tprot.next(8); err != nil {
			return
		}
	case thrift.STRING:
		if err = sd.skipString(); err != nil {
			return
		}
	case thrift.STRUCT:
		if err = sd.skipStruct(maxDepth - 1); err != nil {
			return
		}
	case thrift.MAP:
		if err = sd.skipMap(maxDepth - 1); err != nil {
			return
		}
	case thrift.SET:
		if err = sd.skipSet(maxDepth - 1); err != nil {
			return
		}
	case thrift.LIST:
		if err = sd.skipList(maxDepth - 1); err != nil {
			return
		}
	default:
		return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("unknown data type %d", typeId))
	}
	return nil
}

func (sd *skipDecoder) skipStruct(maxDepth int) (err error) {
	var fieldTypeId thrift.TType

	for {
		_, fieldTypeId, _, err = sd.tprot.ReadFieldBegin()
		if err != nil {
			return err
		}
		if fieldTypeId == thrift.STOP {
			return err
		}
		if err = sd.skip(fieldTypeId, maxDepth); err != nil {
			return err
		}
	}
}

// Buffer returns the skipped buffer.
// Using this buffer to feed to Frugal or FastCodec
func (sd *skipDecoder) Buffer() ([]byte, error) {
	return sd.sb.Buffer()
}

// Recycle recycles the internal BinaryProtocol and would not affect the buffer
// returned by Buffer()
func (sd *skipDecoder) Recycle() {
	sd.tprot.Recycle()
}
