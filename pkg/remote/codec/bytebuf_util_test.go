package codec

import (
	"encoding/binary"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"testing"
)

func TestPeekUint32(t *testing.T) {
	bytes := []byte{0x18, 0x19, 0x44, 0x54}
	buffer := remote.NewReaderBuffer(bytes)
	read, err := PeekUint32(buffer)
	b2 := binary.BigEndian.Uint32(bytes)
	test.Assert(t, read == b2, err)

	bytes2 := []byte{0x11, 0x12, 0x13, 0x14, 0x15}
	buffer2 := remote.NewReaderBuffer(bytes2)
	read2, err2 := PeekUint32(buffer2)
	b22 := binary.BigEndian.Uint32(bytes2)
	test.Assert(t, read2 == b22, err2)

}

// test readUint16 with  byte array
func TestReadUint16(t *testing.T) {
	bytes := []byte{0x44, 0x54}
	buffer := remote.NewReaderBuffer(bytes)
	read, err := ReadUint16(buffer)
	bytes2 := binary.BigEndian.Uint16(bytes)
	test.Assert(t, read == bytes2, err)
}

// test writeUint32 with  byte array
func TestWriteUint32(t *testing.T) {
	bytes := []byte{0x12, 0x34, 0x56, 0x78}
	bytes2Uint32 := binary.BigEndian.Uint32(bytes)
	buffer := remote.NewWriterBuffer(4)
	err := WriteUint32(bytes2Uint32, buffer)
	next, _ := buffer.Bytes()
	bytes2 := binary.BigEndian.Uint32(next)
	test.Assert(t, bytes2Uint32 == bytes2, err)
}

// test  ReadString with  string
func TestReadString(t *testing.T) {
	msg := ""
	bytes := []byte(msg)
	buffer := remote.NewReaderBuffer(bytes)
	readString, i, err := ReadString(buffer)
	test.Assert(t, readString == "", err)
	test.Assert(t, i == 0, err)
}

func TestBytes2Uint32(t *testing.T) {
	bytes := []byte{0x01, 0x01, 0x01, 0x01}
	bytes2 := []byte{0x12, 0x34}
	u := binary.BigEndian.Uint32(bytes)
	b, err := Bytes2Uint32(bytes)
	test.Assert(t, u == b, err)
	b2, err2 := Bytes2Uint32(bytes2)
	test.Assert(t, b2 == 0, err2)
}

func TestBytes2Uint16(t *testing.T) {
	bytes := []byte{0x01, 0x01, 0x01, 0x01}
	u := binary.BigEndian.Uint16(bytes)
	b, err := Bytes2Uint16(bytes, 0)
	test.Assert(t, u == b, err)
	b2, err2 := Bytes2Uint16(bytes, 3)
	test.Assert(t, b2 == 0, err2)
}

func TestReadString2BLen(t *testing.T) {
	bytes := []byte{0x01, 0x01, 0x01, 0x01}
	bLen, i, err := ReadString2BLen(bytes, 2)
	test.Assert(t, bLen == "", err)
	test.Assert(t, i == 0, err)
	bLen2, i2, err2 := ReadString2BLen(bytes, 0)
	test.Assert(t, bLen2 == "", err2)
	test.Assert(t, i2 == 0, err2)
	bLen3, i3, err3 := ReadString2BLen(bytes, 3)
	test.Assert(t, bLen3 == "", err3)
	test.Assert(t, i3 == 0, err3)

}

func TestWriteString(t *testing.T) {
	bytes := []byte{0x12, 0x34, 0x56, 0x78}
	buffer := remote.NewReaderBuffer(bytes)
	str := "test123"
	writeString, err := WriteString(str, buffer)
	test.Assert(t, writeString == 0, err)

}

func TestWriteString2BLen(t *testing.T) {
	bytes := []byte{0x12, 0x34, 0x56, 0x78}
	buffer := remote.NewReaderBuffer(bytes)
	str := "test123"
	writeString, err := WriteString2BLen(str, buffer)
	test.Assert(t, writeString == 0, err)
}
