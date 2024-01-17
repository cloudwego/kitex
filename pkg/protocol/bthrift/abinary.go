package bthrift

import (
	"sync"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/cloudwego/kitex/pkg/allocator"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/utils"
)

var acPool sync.Pool

var _ BTProtocol = aBinaryProtocol{}

func NewABinaryProtocol(ac allocator.Allocator) BTProtocol {
	return aBinaryProtocol{binaryProtocol: Binary, ac: ac}
}

type aBinaryProtocol struct {
	binaryProtocol
	ac allocator.Allocator
}

func (p aBinaryProtocol) Allocator() allocator.Allocator {
	return p.ac
}

func (p aBinaryProtocol) ReadString(buf []byte) (value string, length int, err error) {
	size, l, e := p.ReadI32(buf)
	length += l
	if e != nil {
		err = e
		return
	}
	if size < 0 || int(size) > len(buf) {
		return value, length, perrors.NewProtocolErrorWithType(thrift.INVALID_DATA, "[ReadString] the string size greater than buf length")
	}
	value = p.ac.String(utils.SliceByteToString(buf[length : length+int(size)]))
	//value = string(buf[length : length+int(size)])
	length += int(size)
	return
}

func (p aBinaryProtocol) ReadBinary(buf []byte) (value []byte, length int, err error) {
	size, l, e := p.ReadI32(buf)
	length += l
	if e != nil {
		err = e
		return
	}
	if size < 0 || int(size) > len(buf) {
		return value, length, perrors.NewProtocolErrorWithType(thrift.INVALID_DATA, "[ReadBinary] the binary size greater than buf length")
	}
	value = p.ac.Bytes(buf[length : length+int(size)])
	length += int(size)
	return
}
