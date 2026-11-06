package connpool

import (
	"context"
	"fmt"
	"net"

	"github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
)

// IdleConfig returns a copy of the pool's configured idle limits.
func (lp *LongPool) IdleConfig() connpool.IdleConfig { return lp.idleConfig }

// Clone creates an empty, independently owned pool with the same configuration
// and reporter. Closing either pool does not close the other's connections.
func (lp *LongPool) Clone() *LongPool {
	limit := utils.NewMaxCounter(lp.idleConfig.MaxIdleGlobal)
	clone := &LongPool{
		reporter: lp.reporter, idleConfig: lp.idleConfig, globalIdle: limit,
		newPeer: func(addr net.Addr) *peer {
			p := lp.newPeer(addr)
			p.globalIdle = limit
			return p
		},
	}
	clone.sharedTicker = getSharedTicker(clone, clone.idleConfig.MaxIdleTimeout)
	return clone
}

// GetWithConfig borrows from an independently keyed peer using the existing
// connection pool implementation. The key must uniquely identify an address
// and immutable configuration; it is never passed to the dialer or reporter.
// Per-peer limits use cfg; the pool's global idle limit still applies.
// Callers retire a key with Clean and keep retired leases out of Put.
func (lp *LongPool) GetWithConfig(ctx context.Context, network, address, key string, opt remote.ConnOption, cfg connpool.IdleConfig) (net.Conn, error) {
	if key == "" || key == address || cfg.MinIdlePerAddress < 0 ||
		cfg.MaxIdlePerAddress < cfg.MinIdlePerAddress || cfg.MaxIdleTimeout <= 0 {
		return nil, fmt.Errorf("invalid scoped connection pool configuration")
	}
	mapKey := netAddr{network, key}
	value, ok := lp.peerMap.Load(mapKey)
	if !ok {
		p := lp.newPeer(netAddr{network, address})
		p.pool = newPool(cfg.MinIdlePerAddress, cfg.MaxIdlePerAddress, cfg.MaxIdleTimeout)
		value, _ = lp.peerMap.LoadOrStore(mapKey, p)
	}
	p := value.(*peer)
	if p.addr.Network() != network || p.addr.String() != address ||
		p.pool.minIdle != cfg.MinIdlePerAddress || p.pool.maxIdle != cfg.MaxIdlePerAddress ||
		p.pool.maxIdleTimeout != cfg.MaxIdleTimeout {
		return nil, fmt.Errorf("connection pool scope reused with different configuration")
	}
	return p.Get(opt.Dialer, opt.ConnectTimeout, lp.reporter, key)
}
