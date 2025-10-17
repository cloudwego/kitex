//go:build linux
// +build linux

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

package shmipc

import (
	"context"
	"net"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
)

func TestFallbackConnpool(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	shmUDS, _ := net.ResolveUnixAddr("unix", "fakeshmipc.sock")
	fbpool := NewFallbackShmIPCPool(NewDefaultOptions(), shmUDS, uds, "test", nil)

	conn, err := fbpool.Get(context.Background(), "", "", remote.ConnOption{
		Dialer: netpoll.NewDialer(),
	})
	test.Assert(t, err == nil, err)

	test.Assert(t, conn.RemoteAddr().String() == uds.String(), conn.RemoteAddr())
}

func TestNonFallbackConnpool(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()

	fbpool := NewFallbackShmIPCPool(NewDefaultOptions(), shmUDS, uds, "test", nil)

	conn, err := fbpool.Get(context.Background(), "", "", remote.ConnOption{
		Dialer: netpoll.NewDialer(),
	})
	test.Assert(t, err == nil, err)

	test.Assert(t, conn.RemoteAddr().String() == shmUDS.String(), conn.RemoteAddr())
}
