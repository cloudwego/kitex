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

package remotesvr

import (
	"net"
	"os"
	"sync"
	"syscall"

	"github.com/jackedelic/kitex/internal/pkg/endpoint"
	"github.com/jackedelic/kitex/internal/pkg/remote"
)

// Server is the interface for remote server.
type Server interface {
	Start() chan error
	Stop() error
}

type server struct {
	opt      *remote.ServerOption
	listener net.Listener
	transSvr remote.TransServer

	inkHdlFunc endpoint.Endpoint
	sync.Mutex
}

// NewServer creates a remote server.
func NewServer(opt *remote.ServerOption, inkHdlFunc endpoint.Endpoint, transHdlr remote.ServerTransHandler) (Server, error) {
	transSvr := opt.TransServerFactory.NewTransServer(opt, transHdlr)
	s := &server{
		opt:        opt,
		inkHdlFunc: inkHdlFunc,
		transSvr:   transSvr,
	}
	return s, nil
}

// Start starts the server and return chan, the chan receive means server shutdown or err happen
func (s *server) Start() chan error {
	errCh := make(chan error, 1)
	ln, err := s.buildListener()
	if err != nil {
		errCh <- err
		return errCh
	}

	s.Lock()
	s.listener = ln
	s.Unlock()

	go func() { errCh <- s.transSvr.BootstrapServer() }()
	return errCh
}

func (s *server) buildListener() (ln net.Listener, err error) {
	addr := s.opt.Address
	if addr.Network() == "unix" {
		syscall.Unlink(addr.String())
		os.Chmod(addr.String(), os.ModePerm)
	}

	s.opt.Logger.Infof("KITEX: server listen at: %s", addr.String())
	return s.transSvr.CreateListener(addr)
}

// Stop stops the server gracefully.
func (s *server) Stop() (err error) {
	s.Lock()
	defer s.Unlock()
	if s.transSvr != nil {
		err = s.transSvr.Shutdown()
		s.listener = nil
	}
	return
}
