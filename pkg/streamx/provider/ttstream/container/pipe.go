package container

import (
	"fmt"
	"io"
	"runtime"
	"sync"
)

var ErrPipeEOF = io.EOF
var ErrPipeCanceled = fmt.Errorf("pipe canceled")

const (
	pipeStateActive   = 0
	pipeStateClosed   = 1
	pipeStateCanceled = 2
)

type Pipe[Item any] struct {
	locker sync.Locker
	cond   sync.Cond
	queue  *Queue[Item]
	state  int
}

func NewPipe[Item any]() *Pipe[Item] {
	p := new(Pipe[Item])
	p.locker = new(sync.Mutex)
	p.cond = sync.Cond{L: p.locker}
	p.queue = NewQueueWithLocker[Item](p.locker)
	return p
}

// Read will block if there is nothing to read
func (p *Pipe[Item]) Read(items []Item) (int, error) {
READ:
	var n int
	for i := 0; i < len(items); i++ {
		val, ok := p.queue.Get()
		if !ok {
			if i > 0 {
				break
			}
			// let other goroutine read first, and then try again
			runtime.Gosched()
			val, ok = p.queue.Get()
			if !ok {
				break
			}
			// continue
		}
		items[i] = val
		n++
	}
	if n > 0 {
		return n, nil
	}

	// no data to read, waiting writes
	p.cond.L.Lock()
	for {
		empty, state := p.queue.Size() == 0, p.state
		// Important: check empty first and then check state
		if !empty {
			break
		}
		switch state {
		case pipeStateActive:
		case pipeStateClosed:
			p.cond.L.Unlock()
			return 0, ErrPipeEOF
		case pipeStateCanceled:
			p.cond.L.Unlock()
			return 0, ErrPipeCanceled
		}
		p.cond.Wait()
		// when call Close(), cond.Wait will be wake up,
		// and then break the loop
	}
	p.cond.L.Unlock()
	goto READ
}

func (p *Pipe[Item]) Write(items ...Item) error {
	for _, item := range items {
		p.queue.Add(item)
	}
	p.cond.Signal()
	return nil
}

func (p *Pipe[Item]) Close() {
	p.cond.L.Lock()
	p.state = pipeStateClosed
	p.cond.L.Unlock()
	p.cond.Signal()
}

func (p *Pipe[Item]) Cancel() {
	p.cond.L.Lock()
	p.state = pipeStateCanceled
	p.cond.L.Unlock()
	p.cond.Signal()
}
