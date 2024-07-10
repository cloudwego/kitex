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
	"io"

	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	thrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
)

// ApacheCodecAdapter converts a fastcodec struct to apache codec
type ApacheCodecAdapter struct {
	p bthrift.ThriftFastCodec
}

// Write implements thrift.TStruct
func (p ApacheCodecAdapter) Write(tp thrift.TProtocol) error {
	b := make([]byte, p.p.BLength())
	b = b[:p.p.FastWriteNocopy(b, nil)]
	trans := tp.Transport()
	if t, ok := trans.(remoteByteBuffer); ok {
		// remote.ByteBuffer not always implement io.Writer ...
		// can only use WriteBinary
		_, err := t.WriteBinary(b)
		return err
	}
	_, err := tp.Transport().Write(b)
	return err
}

type remoteByteBuffer interface {
	ReadableLen() (n int)
	Next(n int) (p []byte, err error)
	WriteBinary(b []byte) (n int, err error)
}

// Read implements thrift.TStruct
func (p ApacheCodecAdapter) Read(tp thrift.TProtocol) error {
	var err error
	var b []byte
	trans := tp.Transport()
	if t, ok := trans.(remoteByteBuffer); ok {
		// remote.ByteBuffer not always implement io.Reader ...
		// can only use Next()
		b, err = t.Next(t.ReadableLen())
	} else {
		n := trans.RemainingBytes()
		b = make([]byte, n)
		_, err = io.ReadFull(trans, b)
	}
	if err == nil {
		_, err = p.p.FastRead(b)
	}
	return err
}

// ToApacheCodec converts a bthrift.ThriftFastCodec to thrift.TStruct
func ToApacheCodec(p bthrift.ThriftFastCodec) thrift.TStruct {
	return ApacheCodecAdapter{p: p}
}

// UnpackApacheCodec unpacks ToApacheCodec
func UnpackApacheCodec(v interface{}) interface{} {
	return v.(ApacheCodecAdapter).p
}
