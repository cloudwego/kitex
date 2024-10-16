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

package jsonrpc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cloudwego/netpoll"
)

/* JSON RPC Protocol

=== client create a new stream ===
- send {type=META, sid=1, service="a.b.c", method="xxx"}
- send {type=DATA, sid=1, service="a.b.c", method="xxx", payload="..."}
- recv {type=DATA, sid=1, service="a.b.c", method="xxx", payload="..."}

=== server accept a new stream ===
- recv {type=META, sid=1, service="a.b.c", method="xxx"}
- recv {type=DATA, sid=1, service="a.b.c", method="xxx", payload="..."}
- send {type=DATA, sid=1, service="a.b.c", method="xxx", payload="..."}

=== client try to close stream ===
- client send {type=EOF , sid=1, service="a.b.c", method="xxx"}
- server recv {type=EOF , sid=1, service="a.b.c", method="xxx"}
- server send {type=EOF , sid=1, service="a.b.c", method="xxx"}: server close stream
- client recv {type=EOF , sid=1, service="a.b.c", method="xxx"}: client close stream

=== server try to close stream ===
- server send {type=EOF , sid=1, service="a.b.c", method="xxx"}
- client recv {type=EOF , sid=1, service="a.b.c", method="xxx"}
- client send {type=EOF , sid=1, service="a.b.c", method="xxx"}: client close stream
- server recv {type=EOF , sid=1, service="a.b.c", method="xxx"}: server close stream
*/

const (
	frameMagic uint32 = 0x123321
	// meta: new stream
	frameTypeMeta = 0
	// data: stream streamSend/streamRecv data
	frameTypeData = 1
	// eof: stream closed by peer
	frameTypeEOF = 2
)

// Frame define a JSON RPC protocol frame
// - 4                       bytes: frameMagic
// - 4                       bytes: data size
// - 4                       bytes: frame kind
// - 4                       bytes: stream id
// - 4                       bytes: service name size
// - service_name_size       bytes: service name
// - 4                       bytes: method name size
// - method_name_size        bytes: method name
// - ...                     bytes: json payload
type Frame struct {
	typ     int
	sid     int
	service string
	method  string
	payload []byte
}

func newFrame(typ int, sid int, service, method string, payload []byte) Frame {
	return Frame{
		typ:     typ,
		sid:     sid,
		service: service,
		method:  method,
		payload: payload,
	}
}

func EncodeFrame(writer io.Writer, frame Frame) (err error) {
	// not include data size field length
	dataSize := 4*4 + len(frame.service) + len(frame.method) + len(frame.payload)
	data := make([]byte, 4+4+dataSize)
	offset := 0

	// header
	binary.BigEndian.PutUint32(data[offset:offset+4], frameMagic)
	offset += 4
	binary.BigEndian.PutUint32(data[offset:offset+4], uint32(dataSize))
	offset += 4

	// data
	binary.BigEndian.PutUint32(data[offset:offset+4], uint32(frame.typ))
	offset += 4
	binary.BigEndian.PutUint32(data[offset:offset+4], uint32(frame.sid))
	offset += 4
	binary.BigEndian.PutUint32(data[offset:offset+4], uint32(len(frame.service)))
	offset += 4
	copy(data[offset:offset+len(frame.service)], frame.service)
	offset += len(frame.service)
	binary.BigEndian.PutUint32(data[offset:offset+4], uint32(len(frame.method)))
	offset += 4
	copy(data[offset:offset+len(frame.method)], frame.method)
	offset += len(frame.method)
	copy(data[offset:offset+len(frame.payload)], frame.payload)
	offset += len(frame.payload)
	_ = offset

	idx := 0
	for idx < len(data) {
		n, err := writer.Write(data[idx:])
		if err != nil {
			return err
		}
		idx += n
	}
	return nil
}

func EncodePayload(msg any) ([]byte, error) {
	return json.Marshal(msg)
}

func DecodeFrame(reader io.Reader) (frame Frame, err error) {
	header := make([]byte, 8)
	_, err = io.ReadFull(reader, header)
	if err != nil {
		return
	}
	magic := binary.BigEndian.Uint32(header[:4])
	size := binary.BigEndian.Uint32(header[4:8])
	if magic != frameMagic {
		err = fmt.Errorf("invalid frame magic number: %d", magic)
		return
	}

	data := make([]byte, size)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return
	}
	offset := 0
	frame.typ = int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4
	frame.sid = int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4
	serviceSize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4
	frame.service = string(data[offset : offset+serviceSize])
	offset += serviceSize
	methodSize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4
	frame.method = string(data[offset : offset+methodSize])
	offset += methodSize
	frame.payload = data[offset:]
	return
}

func DecodePayload(payload []byte, msg any) (err error) {
	return json.Unmarshal(payload, msg)
}

func checkFrame(conn netpoll.Connection) error {
	header, err := conn.Reader().Peek(8)
	if err != nil {
		return err
	}
	magic := binary.BigEndian.Uint32(header[:4])
	if magic != frameMagic {
		return fmt.Errorf("invalid frame magic number: %d", magic)
	}
	return nil
}
