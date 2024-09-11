package ttstream

import (
	"runtime"
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/netpoll"
)

// TODO: it's to complex for users implement idle check
// so let's implement it in netpoll

func newTransStack() *transStack {
	return &transStack{}
}

// FILO
type transStack struct {
	mu       sync.Mutex
	stack    []*transport // TODO: now it's a mem leak stack implementation
	modified time.Time
}

func (s *transStack) Pop() (trans *transport) {
	s.mu.Lock()
	if len(s.stack) == 0 {
		s.mu.Unlock()
		return nil
	}
	trans = s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	s.mu.Unlock()
	return trans
}

func (s *transStack) Push(trans *transport) {
	s.mu.Lock()
	s.stack = append(s.stack, trans)
	s.modified = time.Now()
	s.mu.Unlock()
}

func (s *transStack) Clear() {
	s.mu.Lock()
	s.stack = []*transport{}
	s.modified = time.Now()
	s.mu.Unlock()
}

func newTransPool(sinfo *serviceinfo.ServiceInfo) *transPool {
	tp := &transPool{sinfo: sinfo}
	go func() {
		now := time.Now()
		deleteKeys := make([]string, 0)
		tp.pool.Range(func(addr, value any) bool {
			tstack := value.(*transStack)
			duration := now.Sub(tstack.modified)
			if duration >= time.Minute*10 {
				deleteKeys = append(deleteKeys, addr.(string))
			}
			return true
		})
	}()
	return tp
}

type transPool struct {
	pool  sync.Map // {"addr":*transStack}
	sinfo *serviceinfo.ServiceInfo
}

func (c *transPool) Get(network string, addr string) (trans *transport, err error) {
	var cstack *transStack
	val, ok := c.pool.Load(addr)
	if !ok {
		// TODO: here may have a race problem
		cstack = newTransStack()
		_, _ = c.pool.LoadOrStore(addr, cstack)
	} else {
		cstack = val.(*transStack)
	}
	trans = cstack.Pop()
	if trans != nil {
		return trans, nil
	}
	conn, err := netpoll.DialConnection(network, addr, time.Second)
	if err != nil {
		return nil, err
	}
	trans = newTransport(clientTransport, c.sinfo, conn)
	runtime.SetFinalizer(trans, func(t *transport) { t.close() })
	return trans, nil
}

func (c *transPool) Put(trans *transport) {
	var cstack *transStack
	val, ok := c.pool.Load(trans.conn.RemoteAddr())
	if !ok {
		return
	}
	cstack = val.(*transStack)
	cstack.Push(trans)
}
