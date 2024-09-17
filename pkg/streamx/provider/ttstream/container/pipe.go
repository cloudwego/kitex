/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package container

import (
	"fmt"
	"io"
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
			break
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
