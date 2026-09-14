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

package gonet

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	mocksnet "github.com/cloudwego/kitex/internal/mocks/net"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func TestGonetConnExtensionSetReadTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		realDeadline time.Time
		isSet        bool
	)
	mc := mocksnet.NewMockConn(ctrl)
	mc.EXPECT().SetReadDeadline(gomock.Any()).DoAndReturn(func(t time.Time) error {
		realDeadline = t
		isSet = true
		return nil
	}).AnyTimes()

	cfg := rpcinfo.NewRPCConfig()
	mcfg := rpcinfo.AsMutableRPCConfig(cfg)

	// client
	// 1. with timeout
	e := &gonetConnExtension{}
	timeout := time.Second
	mcfg.SetRPCTimeout(timeout)
	expected := time.Now().Add(timeout) // now + timeout + trans.readMoreTimeout(5ms)
	expectedGap := 10 * time.Millisecond
	e.SetReadTimeout(context.Background(), mc, cfg, remote.Client)
	test.Assert(t, realDeadline.Sub(expected) < expectedGap+5*time.Millisecond) // expected gap + trans.readMoreTimeout(5ms)
	test.Assert(t, isSet)
	// 2. no timeout
	isSet = false
	mcfg.SetRPCTimeout(0)
	e.SetReadTimeout(context.Background(), mc, cfg, remote.Client)
	test.Assert(t, realDeadline == time.Time{})
	test.Assert(t, isSet)

	// server, no effect
	isSet = false
	mcfg.SetReadWriteTimeout(timeout)
	e.SetReadTimeout(context.Background(), mc, cfg, remote.Server)
	test.Assert(t, !isSet)
}

// A custom wrapper can hide netpoll's duration timeout API while retaining its
// unsupported SetReadDeadline. It must fail closed instead of stranding a read.
type unsupportedDeadlineConn struct{ net.Conn }

func (c unsupportedDeadlineConn) SetReadDeadline(time.Time) error {
	return errors.New("read deadlines unsupported")
}

func TestGonetClosesConnectionWhenReadTimeoutUnsupported(t *testing.T) {
	c, peer := net.Pipe()
	defer peer.Close()
	conn := NewClientConn(unsupportedDeadlineConn{c})
	defer conn.Close()
	cfg := rpcinfo.NewRPCConfig()
	rpcinfo.AsMutableRPCConfig(cfg).SetRPCTimeout(time.Millisecond)
	NewGonetExtension().SetReadTimeout(context.Background(), conn, cfg, remote.Client)
	done := make(chan error, 1)
	go func() { _, err := conn.Read(make([]byte, 1)); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected closed connection")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("unsupported deadline left read blocked")
	}
}
