/*
 * Copyright 2025 CloudWeGo Authors
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

// Package shmipc is the transport extension for shmipc which can be used in local proxy scene
package shmipc

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/cloudwego/shmipc-go"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
)

const (
	// Network
	udsNetwork = "unix"
)

// NewTransServerFactory creates a transport server factory for SHMIPC with given options
func NewTransServerFactory(opts *Options, shmUDSAddr net.Addr) remote.TransServerFactory {
	if opts == nil {
		opts = NewDefaultOptions()
	}
	return &shmIPCTransServerFactory{opts: opts, shmAddr: shmUDSAddr}
}

type shmIPCTransServerFactory struct {
	opts    *Options
	shmAddr net.Addr
}

func (f *shmIPCTransServerFactory) NewTransServer(opt *remote.ServerOption, transHdlr remote.ServerTransHandler) remote.TransServer {
	return &transServer{opts: f.opts, opt: opt, transHdlr: transHdlr, shmAddr: f.shmAddr}
}

type transServer struct {
	opts      *Options
	opt       *remote.ServerOption
	transHdlr remote.ServerTransHandler

	shmLn         *shmipc.Listener
	shmipcHandler *shmipcListenCallback
	connCount     utils.AtomicInt
	shutdown      int32
	sync.Mutex

	shmAddr net.Addr
}

var _ remote.TransServer = &transServer{}

func (ts *transServer) CreateListener(_ net.Addr) (net.Listener, error) {
	if ts.shmAddr.Network() != udsNetwork {
		return nil, fmt.Errorf("invalid addr[%s], just support unix in shmipc", ts.shmAddr)
	}

	syscall.Unlink(ts.shmAddr.String())
	_ = os.MkdirAll(filepath.Dir(ts.shmAddr.String()), os.ModePerm)
	config := shmipc.NewDefaultListenerConfig(ts.shmAddr.String(), udsNetwork)
	ts.shmipcHandler = newShmipcListenCallback(ts)
	ln, err := shmipc.NewListener(ts.shmipcHandler, config)
	if err != nil {
		return nil, err
	}
	ts.shmLn = ln
	klog.Infof("KITEX: shmipc server listen at addr=%s", ts.shmLn.Addr().String())
	return ts.shmLn, err
}

func (ts *transServer) BootstrapServer(_ net.Listener) (err error) {
	if ts.shmLn == nil {
		return fmt.Errorf("listener[addr=%s] is nil in shmipc transport server", ts.shmAddr.String())
	}

	shmSvrErrCh := make(chan error, 1)
	go func() { shmSvrErrCh <- ts.shmLn.Run() }()

	select {
	case err = <-shmSvrErrCh:
	}
	if err != nil {
		klog.Errorf("KITEX: shmipc server failed: error=%s", err.Error())
	}
	return err
}

func (ts *transServer) Shutdown() (err error) {
	ts.Lock()
	defer ts.Unlock()
	if ts.shmLn != nil {
		atomic.StoreInt32(&ts.shutdown, 1)
		err = ts.shmLn.Close()
		ts.shmLn = nil
	}
	return
}

func (ts *transServer) ConnCount() utils.AtomicInt {
	return ts.connCount
}

func (ts *transServer) onConnInactive(s *shmipc.Stream) {
	ts.transHdlr.OnInactive(context.Background(), s)
	ts.connCount.Dec()
}

var (
	_ shmipc.ListenCallback  = &shmipcListenCallback{}
	_ shmipc.StreamCallbacks = &shmipcStreamCallback{}
)

type shmipcListenCallback struct {
	server *transServer
}

type shmipcStreamCallback struct {
	server *transServer
	stream *shmipc.Stream
	ctx    context.Context
}

func newShmipcListenCallback(server *transServer) *shmipcListenCallback {
	return &shmipcListenCallback{server: server}
}

func newShmipcStreamCallback(ctx context.Context, server *transServer, stream *shmipc.Stream) *shmipcStreamCallback {
	return &shmipcStreamCallback{server: server, ctx: ctx, stream: stream}
}

func (c *shmipcListenCallback) OnNewStream(s *shmipc.Stream) {
	ctx, err := c.server.transHdlr.OnActive(context.Background(), s)
	if err != nil {
		c.server.transHdlr.OnError(ctx, err, s)
		c.server.transHdlr.OnInactive(ctx, s)
		return
	}
	streamCallback := newShmipcStreamCallback(ctx, c.server, s)
	s.SetCallbacks(streamCallback)
	c.server.connCount.Inc()
}

func (c *shmipcListenCallback) OnShutdown(reason string) {
	klog.Warnf("shmipc listener shutdown: addr=%s reason=%s",
		c.server.shmLn.Addr().String(), reason)
}

func (c *shmipcStreamCallback) OnData(reader shmipc.BufferReader) {
	if err := c.server.transHdlr.OnRead(c.ctx, c.stream); err != nil {
		c.server.transHdlr.OnError(c.ctx, err, c.stream)
		if c.stream != nil {
			c.stream.Close()
		}
	}
}

func (c *shmipcStreamCallback) OnLocalClose() {
	c.server.onConnInactive(c.stream)
}

func (c *shmipcStreamCallback) OnRemoteClose() {
	c.server.onConnInactive(c.stream)
	c.stream.Close()
}
