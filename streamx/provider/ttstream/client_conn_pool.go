package ttstream

import (
	"github.com/cloudwego/netpoll"
	"sync"
	"time"
)

const maxStreamsPerConn = 1

// FILO
type connStack struct {
	mu    sync.Mutex
	stack []netpoll.Connection
}

func (s *connStack) Pop() (conn netpoll.Connection) {
	s.mu.Lock()
	if len(s.stack) == 0 {
		s.mu.Unlock()
		return nil
	}
	conn = s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	s.mu.Unlock()
	return conn
}

func (s *connStack) Push(conn netpoll.Connection) {
	s.mu.Lock()
	s.stack = append(s.stack, conn)
	s.mu.Unlock()
}

type connPool struct {
	pool sync.Map // {"addr":*connStack}
}

func (c *connPool) Get(network string, addr string) (conn netpoll.Connection, err error) {
	var cstack *connStack
	val, ok := c.pool.Load(addr)
	if !ok {
		val, _ = c.pool.LoadOrStore(addr, new(connStack))
	}
	cstack = val.(*connStack)
	conn = cstack.Pop()
	if conn == nil {
		conn, err = netpoll.DialConnection(network, addr, time.Second)
		if err != nil {
			return nil, err
		}
	}
	return conn, nil
}

func (c *connPool) Put(conn netpoll.Connection) {
	var cstack *connStack
	val, ok := c.pool.Load(conn.RemoteAddr())
	if !ok {
		return
	}
	cstack = val.(*connStack)
	cstack.Push(conn)
}
