package connpool

import (
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	config "github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/remote"
)

type scopeConn struct {
	addr   net.Addr
	closed atomic.Int32
}

func (*scopeConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (*scopeConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c *scopeConn) Close() error                   { c.closed.Add(1); return nil }
func (c *scopeConn) IsActive() bool                 { return c.closed.Load() == 0 }
func (c *scopeConn) LocalAddr() net.Addr            { return c.addr }
func (c *scopeConn) RemoteAddr() net.Addr           { return c.addr }
func (*scopeConn) SetDeadline(time.Time) error      { return nil }
func (*scopeConn) SetReadDeadline(time.Time) error  { return nil }
func (*scopeConn) SetWriteDeadline(time.Time) error { return nil }

type scopeDialer struct{ dials atomic.Int32 }

func (d *scopeDialer) DialTimeout(network, addr string, _ time.Duration) (net.Conn, error) {
	d.dials.Add(1)
	return &scopeConn{addr: netAddr{network, addr}}, nil
}

func TestScopedPeersKeepRealTargetsAndSharedGlobalLimit(t *testing.T) {
	cfg := config.IdleConfig{MaxIdlePerAddress: 2, MaxIdleGlobal: 2, MaxIdleTimeout: time.Hour}
	p := NewLongPool("scope", cfg)
	defer p.Close()
	d := new(scopeDialer)
	opt := remote.ConnOption{Dialer: d}
	get := func(key string, selected config.IdleConfig) *longConn {
		t.Helper()
		c, err := p.GetWithConfig(context.Background(), "tcp", "127.0.0.1:80", key, opt, selected)
		if err != nil {
			t.Fatal(err)
		}
		if c.RemoteAddr().String() != "127.0.0.1:80" {
			t.Fatal("scope key reached the physical dialer")
		}
		return c.(*longConn)
	}
	one := cfg
	one.MaxIdlePerAddress = 1
	a, b := get("scope-a", one), get("scope-a", one)
	_ = p.Put(a)
	_ = p.Put(b)
	if b.Conn.(*scopeConn).closed.Load() != 1 || p.globalIdle.Now() != 1 {
		t.Fatal("scoped peer idle bound was lost")
	}
	c, other := get("scope-b", cfg), get("scope-b", cfg)
	_ = p.Put(c)
	_ = p.Put(other)
	if other.Conn.(*scopeConn).closed.Load() != 1 || p.globalIdle.Now() != 2 {
		t.Fatal("scope escaped the original global idle bound")
	}
	if _, err := p.GetWithConfig(context.Background(), "tcp", "127.0.0.1:80", "scope-a", opt, cfg); err == nil {
		t.Fatal("mutable scope configuration was accepted")
	}
	legacy, err := p.Get(context.Background(), "tcp", "127.0.0.1:80", opt)
	if err != nil || legacy == a || legacy == c {
		t.Fatal("ordinary Get borrowed a scoped connection")
	}
	_ = p.Discard(legacy)
}

func TestClonedPoolHasIndependentLifetimeAndAccounting(t *testing.T) {
	p := NewLongPool("clone", config.IdleConfig{MaxIdlePerAddress: 2, MaxIdleGlobal: 2, MaxIdleTimeout: time.Hour})
	clone := p.Clone()
	defer clone.Close()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	opt := remote.ConnOption{Dialer: new(scopeDialer)}
	c, err := clone.Get(context.Background(), "tcp", "127.0.0.1:80", opt)
	if err != nil {
		t.Fatal(err)
	}
	_ = clone.Put(c)
	if clone.globalIdle.Now() != 1 || p.globalIdle.Now() != 0 || clone.IdleConfig() != p.IdleConfig() {
		t.Fatal("clone retained the original pool's idle ownership")
	}
	reused, err := clone.Get(context.Background(), "tcp", "127.0.0.1:80", opt)
	if err != nil || reused != c {
		t.Fatal("closing the template affected clone reuse")
	}
	_ = clone.Discard(reused)
}
