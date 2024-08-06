package jsonrpc

import (
	"context"
	"github.com/cloudwego/kitex/pkg/remote"
	"net"
)

var _ remote.ConnPool = (*connPool)(nil)

type connPool struct{}

func NewConnPool() remote.ConnPool {
	return &connPool{}
}

func (c *connPool) Get(ctx context.Context, network, address string, opt remote.ConnOption) (net.Conn, error) {
	conn, err := net.Dial(network, address)
	return conn, err
}

func (c *connPool) Put(conn net.Conn) error {
	return conn.Close()
}

func (c *connPool) Discard(conn net.Conn) error {
	return conn.Close()
}

func (c *connPool) Close() error {
	return nil
}
