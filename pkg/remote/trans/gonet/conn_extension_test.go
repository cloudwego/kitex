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
