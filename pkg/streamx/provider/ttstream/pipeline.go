package ttstream

import (
	"io"
	"sync"
)

type pipeNode[Item any] struct {
	val  Item
	next *pipeNode[Item]
}

func newPipeNode[Item any](item Item) *pipeNode[Item] {
	return &pipeNode[Item]{val: item, next: nil}
}

var PipeEOF = io.EOF

type Pipe[Item any] struct {
	head   *pipeNode[Item]
	tail   *pipeNode[Item]
	closed bool
	cond   sync.Cond
}

func NewPipe[Item any]() *Pipe[Item] {
	p := new(Pipe[Item])
	p.cond = sync.Cond{L: &sync.Mutex{}}
	return p
}

func (p *Pipe[Item]) status() (empty, closed bool) {
	return p.head == nil, p.closed
}

func (p *Pipe[Item]) write(items ...Item) {
	for i := 0; i < len(items); i++ {
		node := newPipeNode[Item](items[i])
		if p.tail == nil { // empty
			p.head = node
			p.tail = p.head
		} else {
			p.tail.next = node
			p.tail = p.tail.next
		}
	}
}

func (p *Pipe[Item]) read(items []Item) int {
	if p.head == nil {
		return 0
	}
	node := p.head
	read := 0
	for ; read < len(items); read++ {
		if node == nil {
			break
		}
		items[read] = node.val
		node = node.next
	}
	if node == nil {
		// read all nodes
		p.head = nil
		p.tail = nil
	} else {
		// not read all nodes
		p.head = node
	}
	return read
}

func (p *Pipe[Item]) Write(items ...Item) {
	p.cond.L.Lock()
	p.write(items...)
	p.cond.L.Unlock()
	p.cond.Signal()
}

func (p *Pipe[Item]) Read(items []Item) (n int, err error) {
	p.cond.L.Lock()
	var empty, closed bool
	for {
		empty, closed = p.status()
		// Important: check empty first and then check EOF
		if !empty {
			break
		}
		if closed {
			return 0, PipeEOF
		}
		p.cond.Wait()
		// when call Close(), cond.Wait will be wake up,
		// and then break the loop
	}
	n = p.read(items)
	p.cond.L.Unlock()
	return n, nil
}

func (p *Pipe[Item]) Close() {
	p.cond.L.Lock()
	p.closed = true
	p.cond.L.Unlock()
	p.cond.Broadcast()
}
