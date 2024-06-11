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
	"encoding/binary"
	"errors"
	"fmt"

	thrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

const (
	EnableSkipDecoder CodecType = 0b10000
)

// skipDecoder is used to parse the input byte-by-byte and skip the thrift payload
// for making use of Frugal and FastCodec in standard Thrift Binary Protocol scenario.
type skipDecoder struct {
	remote.ByteBuffer

	r   int
	buf []byte
}

func (p *skipDecoder) init() (err error) {
	p.r = 0
	p.buf, err = p.Peek(p.ReadableLen())
	return
}

func (p *skipDecoder) loadmore(n int) error {
	// trigger underlying conn to read more
	_, err := p.Peek(n)
	if err == nil {
		// read as much as possible, luckly, we will have a full buffer
		// then we no need to call p.Peek many times
		p.buf, err = p.Peek(p.ReadableLen())
	}
	return err
}

func (p *skipDecoder) next(n int) ([]byte, error) {
	if len(p.buf)-p.r < n {
		if err := p.loadmore(p.r + n); err != nil {
			return nil, err
		}
		// after calling p.loadmore, p.buf MUST be at least (p.r + n) len
	}
	ret := p.buf[p.r : p.r+n]
	p.r += n
	return ret, nil
}

func (p *skipDecoder) NextStruct() (buf []byte, err error) {
	// should be ok with init, just one less p.Peek call
	if err := p.init(); err != nil {
		return nil, err
	}
	if err := p.skip(thrift.STRUCT, thrift.DEFAULT_RECURSION_DEPTH); err != nil {
		return nil, err
	}
	return p.Next(p.r)
}

// skip skips bytes for a specific type.
// After calling skip, p.r contains the len of the type.
// Since BinaryProtocol calls Next of remote.ByteBuffer many times when reading a struct,
// we don't use it for performance concerns
func (p *skipDecoder) skip(typeID thrift.TType, maxDepth int) error {
	if maxDepth <= 0 {
		return thrift.NewTProtocolExceptionWithType(thrift.DEPTH_LIMIT, errors.New("depth limit exceeded"))
	}
	switch typeID {
	case thrift.BOOL, thrift.BYTE:
		if _, err := p.next(1); err != nil {
			return err
		}
	case thrift.I16:
		if _, err := p.next(2); err != nil {
			return err
		}
	case thrift.I32:
		if _, err := p.next(4); err != nil {
			return err
		}
	case thrift.I64, thrift.DOUBLE:
		if _, err := p.next(8); err != nil {
			return err
		}
	case thrift.STRING:
		b, err := p.next(4)
		if err != nil {
			return err
		}
		sz := int(binary.BigEndian.Uint32(b))
		if sz < 0 {
			return perrors.InvalidDataLength
		}
		if _, err := p.next(sz); err != nil {
			return err
		}
	case thrift.STRUCT:
		for {
			b, err := p.next(1) // TType
			if err != nil {
				return err
			}
			tp := thrift.TType(b[0])
			if tp == thrift.STOP {
				break
			}
			if _, err := p.next(2); err != nil { // Field ID
				return err
			}
			if err := p.skip(tp, maxDepth-1); err != nil {
				return err
			}
		}
	case thrift.MAP:
		b, err := p.next(6) // 1 byte key TType, 1 byte value TType, 4 bytes Len
		if err != nil {
			return err
		}
		kt, vt, sz := thrift.TType(b[0]), thrift.TType(b[1]), int32(binary.BigEndian.Uint32(b[2:]))
		if sz < 0 {
			return perrors.InvalidDataLength
		}
		for i := int32(0); i < sz; i++ {
			if err := p.skip(kt, maxDepth-1); err != nil {
				return err
			}
			if err := p.skip(vt, maxDepth-1); err != nil {
				return err
			}
		}
	case thrift.SET, thrift.LIST:
		b, err := p.next(5) // 1 byte value type, 4 bytes Len
		if err != nil {
			return err
		}
		vt, sz := thrift.TType(b[0]), int32(binary.BigEndian.Uint32(b[1:]))
		if sz < 0 {
			return perrors.InvalidDataLength
		}
		for i := int32(0); i < sz; i++ {
			if err := p.skip(vt, maxDepth-1); err != nil {
				return err
			}
		}
	default:
		return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("unknown data type %d", typeID))
	}
	return nil
}
