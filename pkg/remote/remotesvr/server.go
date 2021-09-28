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

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/hotrestart"
	"github.com/cloudwego/kitex/pkg/remote"
)

// Server is the interface for remote server.
type Server interface {
	Start() chan error
	Stop() error
	Address() net.Addr
}

type server struct {
	opt        *remote.ServerOption
	listener   net.Listener
	transSvr   remote.TransServer
	inkHdlFunc endpoint.Endpoint
	launcher   hotrestart.Launcher
	sync.Mutex
}

// NewServer creates a remote server.
func NewServer(opt *remote.ServerOption, inkHdlFunc endpoint.Endpoint, transHdlr remote.ServerTransHandler) (Server, error) {
	transSvr := opt.TransServerFactory.NewTransServer(opt, transHdlr)
	s := &server{
		opt:        opt,
		inkHdlFunc: inkHdlFunc,
		transSvr:   transSvr,
		launcher:   hotrestart.NewLauncher(),
	}
	return s, nil
}

// Start starts the server and return chan, the chan receive means server shutdown or err happen
func (s *server) Start() chan error {
	errCh := make(chan error, 1)
	opt := &s.opt.HotRestart

	// hot restart
	if opt.RestartTimes > 0 {
		go func() {
			s.opt.Logger.Infof("KITEX: server restart ...")
			errCh <- s.launcher.Restart(opt.AdminAddr, s.transSvr.BootstrapServer, s.Stop)
			s.opt.Logger.Infof("KITEX: server quit.")
		}()
		return errCh
	}

	// first time
	ln, err := s.buildListener()
	if err != nil {
		errCh <- err
		return errCh
	}
	s.listener = ln

	// common mod
	if opt.AdminAddr == nil {
		go func() {
			s.opt.Logger.Infof("KITEX: server start ...")
			errCh <- s.transSvr.BootstrapServer(ln)
			s.opt.Logger.Infof("KITEX: server quit.")
		}()
		return errCh
	}

	// first with admin
	go func() {
		s.opt.Logger.Infof("KITEX: server start with admin ...")
		errCh <- s.launcher.Start(opt.AdminAddr, ln, s.transSvr.BootstrapServer, s.Stop)
		s.opt.Logger.Infof("KITEX: server start with admin quit.")
	}()
	return errCh
}

func (s *server) buildListener() (ln net.Listener, err error) {
	addr := s.opt.Address
	if addr.Network() == "unix" {
		syscall.Unlink(addr.String())
		os.Chmod(addr.String(), os.ModePerm)
	}

	if ln, err = s.transSvr.CreateListener(addr); err != nil {
		s.opt.Logger.Errorf("KITEX: server listen at %s failed, err=%v", addr.String(), err)
	} else {
		s.opt.Logger.Infof("KITEX: server listen at %s", ln.Addr().String())
	}
	return
}

// Stop stops the server gracefully.
func (s *server) Stop() (err error) {
	s.Lock()
	defer s.Unlock()
	if s.transSvr != nil {
		err = s.transSvr.Shutdown()
		s.listener = nil
	}
	if s.launcher != nil {
		s.opt.Logger.Infof("KITEX: launcher shutdown now ...")
		s.launcher.Shutdown()
	}
	return
}

func (s *server) Address() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return s.opt.Address
}
