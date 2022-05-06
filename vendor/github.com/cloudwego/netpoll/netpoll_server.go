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

package netpoll

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
)

// newServer wrap listener into server, quit will be invoked when server exit.
func newServer(ln Listener, opts *options, onQuit func(err error)) *server {
	return &server{
		ln:     ln,
		opts:   opts,
		onQuit: onQuit,
	}
}

type server struct {
	operator    FDOperator
	ln          Listener
	opts        *options
	onQuit      func(err error)
	connections sync.Map // key=fd, value=connection
}

// Run this server.
func (s *server) Run() (err error) {
	s.operator = FDOperator{
		FD:     s.ln.Fd(),
		OnRead: s.OnRead,
		OnHup:  s.OnHup,
	}
	s.operator.poll = pollmanager.Pick()
	err = s.operator.Control(PollReadable)
	if err != nil {
		s.onQuit(err)
	}
	return err
}

// Close this server with deadline.
func (s *server) Close(ctx context.Context) error {
	s.operator.Control(PollDetach)
	s.ln.Close()

	var ticker = time.NewTicker(time.Second)
	defer ticker.Stop()
	var hasConn bool
	for {
		hasConn = false
		s.connections.Range(func(key, value interface{}) bool {
			var conn, ok = value.(gracefulExit)
			if !ok || conn.isIdle() {
				value.(Connection).Close()
			}
			hasConn = true
			return true
		})
		if !hasConn { // all connections have been closed
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			continue
		}
	}
}

// OnRead implements FDOperator.
func (s *server) OnRead(p Poll) error {
	// accept socket
	conn, err := s.ln.Accept()
	if err != nil {
		// shut down
		if strings.Contains(err.Error(), "closed") {
			s.operator.Control(PollDetach)
			s.onQuit(err)
			return err
		}
		log.Println("accept conn failed:", err.Error())
		return err
	}
	if conn == nil {
		return nil
	}
	// store & register connection
	var connection = &connection{}
	connection.init(conn.(Conn), s.opts)
	if !connection.IsActive() {
		return nil
	}
	var fd = conn.(Conn).Fd()
	connection.AddCloseCallback(func(connection Connection) error {
		s.connections.Delete(fd)
		return nil
	})
	s.connections.Store(fd, connection)

	// trigger onConnect asynchronously
	connection.onConnect()
	return nil
}

// OnHup implements FDOperator.
func (s *server) OnHup(p Poll) error {
	s.onQuit(errors.New("listener close"))
	return nil
}
