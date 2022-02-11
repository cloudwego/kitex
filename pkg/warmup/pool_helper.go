/*
 * Copyright 2021 CloudWeGo Authors
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

package warmup

import (
	"context"
	"net"
	"sync"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
)

// PoolHelper is trivial implementation to do warm-ups for connection pools.
type PoolHelper struct {
	ErrorHandling
}

// WarmUp warms up the given connection pool.
func (p *PoolHelper) WarmUp(po *PoolOption, pool remote.ConnPool, co remote.ConnOption) (err error) {
	num := 1
	if po.Parallel > num {
		num = po.Parallel
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	mgr := newManager(ctx, po, p.ErrorHandling)
	go mgr.watch()
	var wg sync.WaitGroup
	for i := 0; i < num; i++ {
		new(worker).setup(
			&wg, mgr.jobs, mgr.errs, pool, co, po).goToWork(mgr.control)
	}
	wg.Wait()
	cancel()
	return mgr.report()
}

type manager struct {
	ErrorHandling
	work    context.Context
	control context.Context
	cancel  context.CancelFunc
	errs    chan *job
	jobs    chan *job
	bads    []*job
}

func newManager(ctx context.Context, po *PoolOption, eh ErrorHandling) *manager {
	control, cancel := context.WithCancel(ctx)
	return &manager{
		ErrorHandling: eh,
		work:          ctx,
		control:       control,
		cancel:        cancel,
		errs:          make(chan *job),
		jobs:          chop(po.Targets, po.ConnNum),
	}
}

func (m *manager) report() error {
	<-m.work.Done()
	close(m.errs)
	switch {
	case len(m.bads) == 0:
		return nil
	case m.ErrorHandling == FailFast:
		return m.bads[0].err
	default:
		return m.bads[0].err // TODO: assemble all errors
	}
}

func (m *manager) watch() {
	go func() {
		for {
			select {
			case <-m.work.Done():
				return
			case tmp := <-m.errs:
				m.bads = append(m.bads, tmp)
				if m.ErrorHandling == FailFast {
					m.cancel()  // stop workers
					go func() { // clean up
						discard(m.errs)
						discard(m.jobs)
					}()
					return
				}
			default:
				continue
			}
		}
	}()
}

func discard(ch chan *job) {
	for range ch {
	}
}

func chop(targets map[string][]string, dup int) (js chan *job) {
	js = make(chan *job)
	go func() {
		for network, addresses := range targets {
			for _, address := range addresses {
				for i := 0; i < dup; i++ {
					js <- &job{network: network, address: address}
				}
			}
		}
		close(js)
	}()
	return
}

type job struct {
	network string
	address string
	err     error
}

type oven struct {
	woods chan *job
	ashes chan *job
	pool  remote.ConnPool
	co    remote.ConnOption
	po    *PoolOption
}

func (o *oven) fire(ctx context.Context) {
	var conns []net.Conn
	defer func() {
		for _, conn := range conns {
			o.pool.Put(conn)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case wood := <-o.woods:
			if wood == nil {
				return
			}
			var conn net.Conn
			conn, wood.err = o.pool.Get(
				ctx, wood.network, wood.address, o.co)
			if wood.err == nil {
				conns = append(conns, conn)
			} else {
				o.ashes <- wood
			}
		}
	}
}

type worker struct {
	oven  *oven
	group *sync.WaitGroup
}

func (w *worker) setup(g *sync.WaitGroup, woods, ashes chan *job, pool remote.ConnPool, co remote.ConnOption, po *PoolOption) *worker {
	w.group = g
	w.oven = &oven{
		woods: woods,
		ashes: ashes,
		pool:  pool,
		po:    po,
		co:    co,
	}
	return w
}

func (w *worker) goToWork(ctx context.Context) {
	w.group.Add(1)
	go func() {
		defer w.group.Done()
		defer func() {
			if x := recover(); x != nil {
				klog.Errorf("warmup: unexpected error: %+v", x)
			}
		}()
		w.oven.fire(ctx)
	}()
}
