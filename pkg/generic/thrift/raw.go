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

package thrift

import (
	"context"
	"fmt"

	"github.com/bytedance/gopkg/lang/dirtmake"
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/thrift/base"

	"github.com/cloudwego/kitex/pkg/utils"
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
func (m *RawWriter) Write(ctx context.Context, out bufiox.Writer, msg interface{}, method string, isClient bool, requestBase *base.Base) error {
	if msg == nil {
		bw := thrift.NewBufferWriter(out)
		defer bw.Recycle()
		if err := bw.WriteFieldStop(); err != nil {
			return err
		}
		return nil
	}
	buf, ok := msg.([]byte)
	if !ok {
		return fmt.Errorf("thrift binary generic msg is not []byte, method=%v", method)
	}
	_, err := out.WriteBinary(buf)
	return err
}

// NewRawReader build RawReader
func NewRawReader() *RawReader {
	return &RawReader{}
}

// RawReader implement of MessageReaderWithMethod
type RawReader struct{}

var _ MessageReader = (*RawReader)(nil)

// Read returns the copy of data
func (m *RawReader) Read(ctx context.Context, method string, isClient bool, dataLen int, in bufiox.Reader) (interface{}, error) {
	if dataLen > 0 {
		buf := dirtmake.Bytes(dataLen, dataLen)
		_, err := in.ReadBinary(buf)
		return buf, err
	}
	d := thrift.NewSkipDecoder(in)
	defer d.Release()
	rbuf, err := d.Next(thrift.STRUCT)
	if err != nil {
		return nil, err
	}
	return utils.StringToSliceByte(string(rbuf)), nil
}
