package nphttp2

import (
	"github.com/cloudwego/kitex/internal/test"
	"testing"
	"time"
)

func TestConnPool(t *testing.T) {

	// mock init
	connPool := MockConnPool()
	opt := MockConnOption()
	ctx := MockCtxWithRPCInfo()

	// test Get()
	conn, err := connPool.Get(ctx, "tcp", mockAddr0, opt)
	test.Assert(t, err == nil, err)

	// test connection reuse
	// keep create new connection until filling the pool size,
	// then Get() will reuse established connection by round-robin
	for i := 0; uint32(i) < connPool.size*2; i++ {
		_, err := connPool.Get(ctx, "tcp", mockAddr1, opt)
		test.Assert(t, err == nil, err)
	}

	// test TimeoutGet()
	opt.ConnectTimeout = time.Microsecond
	_, err = connPool.Get(ctx, "tcp", mockAddr0, opt)
	test.Assert(t, err != nil, err)

	//test Clean()
	connPool.Clean("tcp", mockAddr0)

	// test put
	err = connPool.Put(conn)
	test.Assert(t, err == nil, err)

	//test Discard()
	err = connPool.Discard(conn)
	test.Assert(t, err == nil, err)

	//test Close()
	err = connPool.Close()
	test.Assert(t, err == nil, err)

}
