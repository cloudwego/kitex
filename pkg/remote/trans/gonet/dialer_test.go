package gonet

import (
	"net"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestConnAlive(t *testing.T) {
	ch := make(chan struct{})
	address := "127.0.0.1:1280"
	go func(close chan struct{}) {
		l, err := net.Listen("tcp", address)
		if err != nil {
			panic(err)
		}
		select {
		case <-ch:
			l.Close()
			break
		}
	}(ch)

	d := newDialerWithInterval(time.Millisecond * 50).(*dialer)

	conn, err := d.DialTimeout("tcp", address, time.Minute)
	test.Assert(t, err == nil, err)
	_, ok := d.cm.Load(conn)
	test.Assert(t, ok)
	b := d.probe(conn.(*connWithState))
	test.Assert(t, b == false)

	// server close
	close(ch)
	time.Sleep(time.Millisecond * 100)
	b = d.probe(conn.(*connWithState))
	test.Assert(t, b == true)
	// should be removed from the dialer.cm
	_, ok = d.cm.Load(conn)
	test.Assert(t, !ok)
}
