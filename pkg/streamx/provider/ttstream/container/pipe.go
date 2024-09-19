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
	"sync/atomic"
)

type pipeState = int32

const (
	pipeStateInactive pipeState = 0
	pipeStateActive   pipeState = 1
	pipeStateClosed   pipeState = 2
	pipeStateCanceled pipeState = 3
)

var ErrPipeEOF = io.EOF
var ErrPipeCanceled = fmt.Errorf("pipe canceled")
var stateErrors map[pipeState]error = map[pipeState]error{
	pipeStateClosed:   ErrPipeEOF,
	pipeStateCanceled: ErrPipeCanceled,
}

// Pipe implement a queue that never block on Write but block on Read if there is nothing to read
type Pipe[Item any] struct {
	queue   *Queue[Item]
	trigger chan struct{}
	state   pipeState
}

func NewPipe[Item any]() *Pipe[Item] {
	p := new(Pipe[Item])
	p.queue = NewQueue[Item]()
	p.trigger = make(chan struct{}, 1)
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
	for {
		<-p.trigger
		if p.queue.Size() == 0 {
			err := stateErrors[atomic.LoadInt32(&p.state)]
			if err != nil {
				return 0, err
			}
		}
		goto READ
	}
}

func (p *Pipe[Item]) Write(items ...Item) error {
	if !atomic.CompareAndSwapInt32(&p.state, pipeStateInactive, pipeStateActive) && atomic.LoadInt32(&p.state) != pipeStateActive {
		err := stateErrors[atomic.LoadInt32(&p.state)]
		if err != nil {
			return err
		}
		return fmt.Errorf("unknown state error")
	}

	for _, item := range items {
		p.queue.Add(item)
	}
	// wake up
	select {
	case p.trigger <- struct{}{}:
	default:
	}
	return nil
}

func (p *Pipe[Item]) Close() {
	atomic.StoreInt32(&p.state, pipeStateClosed)
	close(p.trigger)
}

func (p *Pipe[Item]) Cancel() {
	atomic.StoreInt32(&p.state, pipeStateCanceled)
	close(p.trigger)
}
