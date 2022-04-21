package nphttp2

import (
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"testing"
	"time"
)

func TestServerConn(t *testing.T) {

	// init
	tr, err := MockServerTransport(mockAddr0)
	test.Assert(t, err == nil, err)
	s := grpc.MockNewServerSideStream()
	serverConn := newServerConn(tr, s)

	// test LocalAddr()
	la := serverConn.LocalAddr()
	test.Assert(t, la == nil)

	// test RemoteAddr()
	ra := serverConn.RemoteAddr()
	test.Assert(t, ra.String() == mockAddr0)

	// test Read()
	testStr := "1234567890"
	testByte := []byte(testStr)
	grpc.MockStreamRecv(s)
	n, err := serverConn.Read(testByte)
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(testStr))
	//todo check string

	// test Write()
	testByte = []byte(testStr)
	n, err = serverConn.Write(testByte)
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(testStr))

	// test Write() short write error
	n, err = serverConn.Write([]byte(""))
	test.Assert(t, err != nil, err)

	// test SetReadDeadline()
	err = serverConn.SetReadDeadline(time.Now())
	test.Assert(t, err == nil, err)

	// test SetDeadline()
	err = serverConn.SetDeadline(time.Now())
	test.Assert(t, err == nil, err)

	// test SetWriteDeadline()
	err = serverConn.SetWriteDeadline(time.Now())
	test.Assert(t, err == nil, err)

	// test Close()
	err = serverConn.Close()
	test.Assert(t, err == nil, err)
}
