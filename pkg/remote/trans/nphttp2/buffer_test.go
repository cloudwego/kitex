package nphttp2

import (
	"github.com/cloudwego/kitex/internal/test"
	"testing"
)

func TestNewBuffer(t *testing.T) {

	// test NewBuffer()
	grpcConn := MockNpConn(mockAddr0)
	buffer := newBuffer(grpcConn)

	// mock conn only returen 00000 bytes
	testBytes := []byte{0, 0, 0, 0, 0}
	err := buffer.WriteData(testBytes)
	test.Assert(t, err == nil, err)

	ret, err := buffer.Next(len(testBytes))
	test.Assert(t, err == nil, err)
	test.Assert(t, string(ret) == string(testBytes))

	err = buffer.WriteHeader(testBytes)
	test.Assert(t, err == nil, err)

	err = buffer.Flush()
	test.Assert(t, err == nil, err)

	err = buffer.Release(nil)
	test.Assert(t, err == nil, err)

}
