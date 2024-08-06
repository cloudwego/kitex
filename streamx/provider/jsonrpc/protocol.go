package jsonrpc

import (
	"encoding/binary"
	"encoding/json"
	"io"
)

const (
	// meta: new stream
	frameTypeMeta = 0
	// data: stream streamSend/streamRecv data
	frameTypeData = 1
	// eof: stream closed by peer
	frameTypeEOF = 2
)

// Frame define a JSON RPC protocol frame
// - 4                       bytes: data size
// - 4                       bytes: frame kind
// - 4                       bytes: stream id
// - 4                       bytes: method name size
// - method_name_size        bytes: method name
// - ...                     bytes: json payload
type Frame struct {
	typ     int
	sid     int
	method  string
	payload []byte
}

func newFrame(typ int, sid int, method string, payload []byte) Frame {
	return Frame{
		typ:     typ,
		sid:     sid,
		method:  method,
		payload: payload,
	}
}

func EncodeFrame(writer io.Writer, frame Frame) (err error) {
	dataSize := 4*3 + len(frame.method) + len(frame.payload) // not include data size field length
	data := make([]byte, 4+dataSize)
	binary.BigEndian.PutUint32(data[:4], uint32(dataSize))
	binary.BigEndian.PutUint32(data[4:8], uint32(frame.typ))
	binary.BigEndian.PutUint32(data[8:12], uint32(frame.sid))
	binary.BigEndian.PutUint32(data[12:16], uint32(len(frame.method)))
	copy(data[16:16+len(frame.method)], frame.method)
	copy(data[16+len(frame.method):], frame.payload)

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
	header := make([]byte, 4)
	_, err = io.ReadFull(reader, header)
	if err != nil {
		return
	}
	size := binary.BigEndian.Uint32(header)

	data := make([]byte, size)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return
	}
	frame.typ = int(binary.BigEndian.Uint32(data[0:4]))
	frame.sid = int(binary.BigEndian.Uint32(data[4:8]))
	methodSize := int(binary.BigEndian.Uint32(data[8:12]))
	frame.method = string(data[12 : 12+methodSize])
	frame.payload = data[12+methodSize:]
	return
}

func DecodePayload(payload []byte, msg any) (err error) {
	return json.Unmarshal(payload, msg)
}
