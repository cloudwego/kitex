package shmipc

import (
	"context"
	"net"
	"time"

	"github.com/cloudwego/shmipc-go"

	connpool2 "github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/connpool"
)

type fallbackShmipcPool struct {
	shmipcConnPool   remote.ConnPool
	fallbackConnPool remote.ConnPool
	fallbackAddr     net.Addr
}

// NewFallbackShmIPCPool create a connection pool which use shmipc stream, and also provides fallback function to
// use uds connection rather than shmipc when shmipc is not available.
func NewFallbackShmIPCPool(psm string, shmipcAddr, fallbackAddr net.Addr, fCP remote.ConnPool) remote.ConnPool {
	sCP := NewShmIPCPool(&Config{
		UnixPathBuilder: func(network, address string) string {
			return shmipcAddr.String()
		},
		SMConfigBuilder: func(network, address string) *shmipc.SessionManagerConfig {
			return DefaultShmipcConfig(psm, shmipcAddr)
		},
	})
	if fCP == nil {
		idleCfg := connpool2.IdleConfig{
			MaxIdlePerAddress: 5000,
			MaxIdleGlobal:     10000,
			MaxIdleTimeout:    5 * time.Second,
		}
		fCP = connpool.NewLongPool(psm, idleCfg)
	}
	return &fallbackShmipcPool{
		shmipcConnPool:   sCP,
		fallbackConnPool: fCP,
		fallbackAddr:     fallbackAddr,
	}
}

func (p *fallbackShmipcPool) Get(ctx context.Context, network, address string, opt remote.ConnOption) (conn net.Conn, err error) {
	conn, err = p.shmipcConnPool.Get(ctx, network, address, opt)
	if err == nil {
		return conn, nil
	}
	return p.fallbackConnPool.Get(ctx, p.fallbackAddr.Network(), p.fallbackAddr.String(), opt)
}

func (p *fallbackShmipcPool) Put(conn net.Conn) error {
	if _, ok := conn.(*shmipc.Stream); ok {
		return p.shmipcConnPool.Put(conn)
	}
	return p.fallbackConnPool.Put(conn)
}

func (p *fallbackShmipcPool) Discard(conn net.Conn) error {
	if _, ok := conn.(*shmipc.Stream); ok {
		return p.shmipcConnPool.Discard(conn)
	}
	return p.fallbackConnPool.Discard(conn)
}

func (p *fallbackShmipcPool) Close() error {
	_ = p.fallbackConnPool.Close()
	return p.shmipcConnPool.Close()
}
