//go:build linux
// +build linux

package shmipc

import (
	"context"
	"net"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
)

func TestFallbackConnpool(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	shmUDS, _ := net.ResolveUnixAddr("unix", "fakeshmipc.sock")
	fbpool := NewFallbackShmIPCPool("test", shmUDS, uds, nil)

	conn, err := fbpool.Get(context.Background(), "", "", remote.ConnOption{
		Dialer: netpoll.NewDialer(),
	})
	test.Assert(t, err == nil, err)

	test.Assert(t, conn.RemoteAddr().String() == uds.String(), conn.RemoteAddr())
}

func TestNonFallbackConnpool(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	fbpool := NewFallbackShmIPCPool("test", shmUDS, uds, nil)

	conn, err := fbpool.Get(context.Background(), "", "", remote.ConnOption{
		Dialer: netpoll.NewDialer(),
	})
	test.Assert(t, err == nil, err)

	test.Assert(t, conn.RemoteAddr().String() == shmUDS.String(), conn.RemoteAddr())
}
