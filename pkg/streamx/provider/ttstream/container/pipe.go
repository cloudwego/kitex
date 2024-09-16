package container

import (
	"io"
	"sync"
)

var PipeEOF = io.EOF

type Pipe[Item any] struct {
	locker sync.Locker
	cond   sync.Cond
	queue  *Queue[Item]
	closed bool
}

func NewPipe[Item any]() *Pipe[Item] {
	p := new(Pipe[Item])
	p.locker = new(sync.Mutex)
	p.cond = sync.Cond{L: p.locker}
	p.queue = NewQueueWithLocker[Item](p.locker)
	return p
}

// Read will block if there is nothing to read
func (p *Pipe[Item]) Read(items []Item) (n int, err error) {
READ:
	for n = 0; n < len(items); n++ {
		val, ok := p.queue.Get()
		if !ok {
			break
		}
		items[n] = val
	}
	if n > 0 {
		return n, nil
	}

	// no data to read, waiting writes
	p.cond.L.Lock()
	var empty, closed bool
	for {
		empty, closed = p.queue.Size() == 0, p.closed
		// Important: check empty first and then check EOF
		if !empty {
			break
		}
		if closed {
			p.cond.L.Unlock()
			return 0, PipeEOF
		}
		p.cond.Wait()
		// when call Close(), cond.Wait will be wake up,
		// and then break the loop
	}
	p.cond.L.Unlock()
	goto READ
}

func (p *Pipe[Item]) Write(items ...Item) {
	for _, item := range items {
		p.queue.Add(item)
	}
	p.cond.Signal()
}

func (p *Pipe[Item]) Close() {
	p.cond.L.Lock()
	p.closed = true
	p.cond.L.Unlock()
	p.cond.Signal()
}
