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
	"io"

	"github.com/cloudwego/gopkg/protocol/thrift"

	athrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
)

// ApacheCodecAdapter converts a fastcodec struct to apache codec
type ApacheCodecAdapter struct {
	p thrift.FastCodec
}

// Write implements athrift.TStruct
func (p ApacheCodecAdapter) Write(tp athrift.TProtocol) error {
	b := make([]byte, p.p.BLength())
	b = b[:p.p.FastWriteNocopy(b, nil)]
	_, err := tp.Transport().Write(b)
	return err
}

// Read implements athrift.TStruct
func (p ApacheCodecAdapter) Read(tp athrift.TProtocol) error {
	var err error
	var b []byte
	trans := tp.Transport()
	n := trans.RemainingBytes()
	if int64(n) < 0 {
		return errors.New("unknown buffer len")
	}
	b = make([]byte, n)
	_, err = io.ReadFull(trans, b)
	if err == nil {
		_, err = p.p.FastRead(b)
	}
	return err
}

// ToApacheCodec converts a thrift.FastCodec to athrift.TStruct
func ToApacheCodec(p thrift.FastCodec) athrift.TStruct {
	return ApacheCodecAdapter{p: p}
}

// UnpackApacheCodec unpacks ToApacheCodec
func UnpackApacheCodec(v interface{}) interface{} {
	return v.(ApacheCodecAdapter).p
}
