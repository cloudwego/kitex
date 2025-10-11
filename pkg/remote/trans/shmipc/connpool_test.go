//go:build linux
// +build linux

package shmipc

import (
	"context"
	"testing"

	"github.com/cloudwego/shmipc-go"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
)

func TestShmIPCPool(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()
	cp := newShmIPCConnPoolForTest()

	conn, err := cp.Get(context.Background(), "", "", remote.ConnOption{
		Dialer: netpoll.NewDialer(),
	})

	test.Assert(t, err == nil, err)
	test.Assert(t, conn.RemoteAddr().String() == shmUDSAddrStr, conn.RemoteAddr().String())
	stream, ok := conn.(*shmipc.Stream)
	test.Assert(t, ok)
	test.Assert(t, stream.IsOpen())

	err = cp.Put(stream)
	test.Assert(t, err == nil, err)
	test.Assert(t, stream.IsOpen())

	err = cp.Discard(stream)
	test.Assert(t, err == nil, err)
	test.Assert(t, !stream.IsOpen())

	cp.Close()
}

func newShmIPCConnPoolForTest() remote.ConnPool {
	return NewShmIPCPool(
		&Config{
			UnixPathBuilder: func(network, address string) string {
				return shmUDS.String()
			},
			SMConfigBuilder: func(network, address string) *shmipc.SessionManagerConfig {
				smc := testShmipcConfig("test", shmUDS)
				return smc
			},
		})
}
