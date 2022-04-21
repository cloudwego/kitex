package nphttp2

import (
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"testing"
	"time"
)

func TestClientConn(t *testing.T) {

	// init
	connPool := MockConnPool()
	ctx := MockCtxWithRPCInfo()
	conn, err := connPool.Get(ctx, "tcp", mockAddr0, MockConnOption())
	test.Assert(t, err == nil, err)
	clientConn := conn.(*clientConn)

	// test LocalAddr()
	la := clientConn.LocalAddr()
	test.Assert(t, la == nil)

	// test RemoteAddr()
	ra := clientConn.RemoteAddr()
	test.Assert(t, ra.String() == mockAddr0)

	// test Read()
	grpc.MockStreamRecv(clientConn.s)
	testStr := "1234567890"
	testByte := []byte(testStr)
	n, err := clientConn.Read(testByte)
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(testStr))
	//todo check testStr data

	// test Write()
	testByte = []byte(testStr)
	n, err = clientConn.Write(testByte)
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(testByte))

	// test Write() short write error
	n, err = clientConn.Write([]byte(""))
	test.Assert(t, err != nil, err)

	// test Header()
	grpc.DontWaitHeader(clientConn.s)
	_, err = clientConn.Header()

	// test Trailer()
	_ = clientConn.Trailer()

	//todo check return value

	// test SetDeadline()
	err = clientConn.SetDeadline(time.Now())
	test.Assert(t, err == nil, err)

	// test SetWriteDeadline()
	err = clientConn.SetWriteDeadline(time.Now())
	test.Assert(t, err == nil, err)

	// test SetReadDeadline()
	err = clientConn.SetReadDeadline(time.Now())
	test.Assert(t, err == nil, err)

	// test Close()
	err = clientConn.Close()
	test.Assert(t, err == nil, err)
}
