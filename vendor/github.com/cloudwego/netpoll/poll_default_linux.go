// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !race
// +build !race

package netpoll

import (
	"log"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Includes defaultPoll/multiPoll/uringPoll...
func openPoll() Poll {
	return openDefaultPoll()
}

func openDefaultPoll() *defaultPoll {
	var poll = defaultPoll{}
	poll.buf = make([]byte, 8)
	var p, err = syscall.EpollCreate1(0)
	if err != nil {
		panic(err)
	}
	poll.fd = p
	var r0, _, e0 = syscall.Syscall(syscall.SYS_EVENTFD2, 0, 0, 0)
	if e0 != 0 {
		syscall.Close(p)
		panic(err)
	}

	poll.Reset = poll.reset
	poll.Handler = poll.handler

	poll.wop = &FDOperator{FD: int(r0)}
	poll.Control(poll.wop, PollReadable)
	return &poll
}

type defaultPoll struct {
	pollArgs
	fd      int         // epoll fd
	wop     *FDOperator // eventfd, wake epoll_wait
	buf     []byte      // read wfd trigger msg
	trigger uint32      // trigger flag
	// fns for handle events
	Reset   func(size, caps int)
	Handler func(events []epollevent) (closed bool)
}

type pollArgs struct {
	size     int
	caps     int
	events   []epollevent
	barriers []barrier
}

func (a *pollArgs) reset(size, caps int) {
	a.size, a.caps = size, caps
	a.events, a.barriers = make([]epollevent, size), make([]barrier, size)
	for i := range a.barriers {
		a.barriers[i].bs = make([][]byte, a.caps)
		a.barriers[i].ivs = make([]syscall.Iovec, a.caps)
	}
}

// Wait implements Poll.
func (p *defaultPoll) Wait() (err error) {
	// init
	var caps, msec, n = barriercap, -1, 0
	p.Reset(128, caps)
	// wait
	for {
		if n == p.size && p.size < 128*1024 {
			p.Reset(p.size<<1, caps)
		}
		n, err = EpollWait(p.fd, p.events, msec)
		if err != nil && err != syscall.EINTR {
			return err
		}
		if n <= 0 {
			msec = -1
			runtime.Gosched()
			continue
		}
		msec = 0
		if p.Handler(p.events[:n]) {
			return nil
		}
	}
}

func (p *defaultPoll) handler(events []epollevent) (closed bool) {
	var hups []*FDOperator // TODO: maybe can use sync.Pool
	for i := range events {
		var operator = *(**FDOperator)(unsafe.Pointer(&events[i].data))
		// trigger or exit gracefully
		if operator.FD == p.wop.FD {
			// must clean trigger first
			syscall.Read(p.wop.FD, p.buf)
			atomic.StoreUint32(&p.trigger, 0)
			// if closed & exit
			if p.buf[0] > 0 {
				syscall.Close(p.wop.FD)
				syscall.Close(p.fd)
				return true
			}
			continue
		}
		if !operator.do() {
			continue
		}

		evt := events[i].events
		// check poll in
		if evt&syscall.EPOLLIN != 0 {
			if operator.OnRead != nil {
				// for non-connection
				operator.OnRead(p)
			} else {
				// for connection
				var bs = operator.Inputs(p.barriers[i].bs)
				if len(bs) > 0 {
					var n, err = readv(operator.FD, bs, p.barriers[i].ivs)
					operator.InputAck(n)
					if err != nil && err != syscall.EAGAIN && err != syscall.EINTR {
						log.Printf("readv(fd=%d) failed: %s", operator.FD, err.Error())
						hups = append(hups, operator)
						operator.done()
						continue
					}
				}
			}
		}

		// check hup
		if evt&(syscall.EPOLLHUP|syscall.EPOLLRDHUP) != 0 {
			hups = append(hups, operator)
			operator.done()
			continue
		}
		if evt&syscall.EPOLLERR != 0 {
			// Under block-zerocopy, the kernel may give an error callback, which is not a real error, just an EAGAIN.
			// So here we need to check this error, if it is EAGAIN then do nothing, otherwise still mark as hup.
			if _, _, _, _, err := syscall.Recvmsg(operator.FD, nil, nil, syscall.MSG_ERRQUEUE); err != syscall.EAGAIN {
				hups = append(hups, operator)
				operator.done()
				continue
			}
		}

		// check poll out
		if evt&syscall.EPOLLOUT != 0 {
			if operator.OnWrite != nil {
				// for non-connection
				operator.OnWrite(p)
			} else {
				// for connection
				var bs, supportZeroCopy = operator.Outputs(p.barriers[i].bs)
				if len(bs) > 0 {
					// TODO: Let the upper layer pass in whether to use ZeroCopy.
					var n, err = sendmsg(operator.FD, bs, p.barriers[i].ivs, false && supportZeroCopy)
					operator.OutputAck(n)
					if err != nil && err != syscall.EAGAIN {
						log.Printf("sendmsg(fd=%d) failed: %s", operator.FD, err.Error())
						hups = append(hups, operator)
					}
				}
			}
		}
		operator.done()
	}
	// hup conns together to avoid blocking the poll.
	if len(hups) > 0 {
		p.detaches(hups)
	}
	return false
}

// Close will write 10000000
func (p *defaultPoll) Close() error {
	_, err := syscall.Write(p.wop.FD, []byte{1, 0, 0, 0, 0, 0, 0, 0})
	return err
}

// Trigger implements Poll.
func (p *defaultPoll) Trigger() error {
	if atomic.AddUint32(&p.trigger, 1) > 1 {
		return nil
	}
	// MAX(eventfd) = 0xfffffffffffffffe
	_, err := syscall.Write(p.wop.FD, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	return err
}

// Control implements Poll.
func (p *defaultPoll) Control(operator *FDOperator, event PollEvent) error {
	var op int
	var evt epollevent
	*(**FDOperator)(unsafe.Pointer(&evt.data)) = operator
	switch event {
	case PollReadable:
		operator.inuse()
		op, evt.events = syscall.EPOLL_CTL_ADD, syscall.EPOLLIN|syscall.EPOLLRDHUP|syscall.EPOLLERR
	case PollModReadable:
		operator.inuse()
		op, evt.events = syscall.EPOLL_CTL_MOD, syscall.EPOLLIN|syscall.EPOLLRDHUP|syscall.EPOLLERR
	case PollDetach:
		defer operator.unused()
		op, evt.events = syscall.EPOLL_CTL_DEL, syscall.EPOLLIN|syscall.EPOLLOUT|syscall.EPOLLRDHUP|syscall.EPOLLERR
	case PollWritable:
		operator.inuse()
		op, evt.events = syscall.EPOLL_CTL_ADD, EPOLLET|syscall.EPOLLOUT|syscall.EPOLLRDHUP|syscall.EPOLLERR
	case PollR2RW:
		op, evt.events = syscall.EPOLL_CTL_MOD, syscall.EPOLLIN|syscall.EPOLLOUT|syscall.EPOLLRDHUP|syscall.EPOLLERR
	case PollRW2R:
		op, evt.events = syscall.EPOLL_CTL_MOD, syscall.EPOLLIN|syscall.EPOLLRDHUP|syscall.EPOLLERR
	}
	return EpollCtl(p.fd, op, operator.FD, &evt)
}

func (p *defaultPoll) detaches(hups []*FDOperator) error {
	var onhups = make([]func(p Poll) error, len(hups))
	for i := range hups {
		onhups[i] = hups[i].OnHup
		p.Control(hups[i], PollDetach)
	}
	go func(onhups []func(p Poll) error) {
		for i := range onhups {
			if onhups[i] != nil {
				onhups[i](p)
			}
		}
	}(onhups)
	return nil
}
