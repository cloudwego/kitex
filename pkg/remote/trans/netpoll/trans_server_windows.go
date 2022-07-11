//go:build windows
// +build windows

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

// Package netpoll contains server and client implementation for netpoll.

package netpoll

import (
	"net"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
)

// NewTransServerFactory creates a default netpoll transport server factory.
func NewTransServerFactory() remote.TransServerFactory {
	return &fakeTransServerFactory{}
}

type fakeTransServerFactory struct{}

// NewTransServer implements the remote.TransServerFactory interface.
func (f *fakeTransServerFactory) NewTransServer(opt *remote.ServerOption, transHdlr remote.ServerTransHandler) remote.TransServer {
	return nil
}

type fakeTransServer struct{}

var _ remote.TransServer = &fakeTransServer{}

// CreateListener implements the remote.TransServer interface.
func (ts *fakeTransServer) CreateListener(addr net.Addr) (_ net.Listener, err error) {
	return nil, nil
}

// BootstrapServer implements the remote.TransServer interface.
func (ts *fakeTransServer) BootstrapServer() (err error) {
	return nil
}

// Shutdown implements the remote.TransServer interface.
func (ts *fakeTransServer) Shutdown() (err error) {
	return nil
}

// ConnCount implements the remote.TransServer interface.
func (ts *fakeTransServer) ConnCount() utils.AtomicInt {
	return 0
}
