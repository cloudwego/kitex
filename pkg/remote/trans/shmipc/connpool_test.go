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
	"testing"

	"github.com/cloudwego/shmipc-go"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
)

func TestShmIPCPool(t *testing.T) {
	testLock.Lock()
	defer testLock.Unlock()
	cp := newShmIPCConnPoolForTest()

	conn, err := cp.Get(context.Background(), "", "", remote.ConnOption{
		Dialer: netpoll.NewDialer(),
	})

	test.Assert(t, err == nil, err)
	test.Assert(t, conn.RemoteAddr().String() == shmUDSAddrStr, conn.RemoteAddr().String())
	stream, ok := conn.(*shmipc.Stream)
	test.Assert(t, ok)
	test.Assert(t, stream.IsOpen())

	err = cp.Put(stream)
	test.Assert(t, err == nil, err)
	test.Assert(t, stream.IsOpen())

	err = cp.Discard(stream)
	test.Assert(t, err == nil, err)
	test.Assert(t, !stream.IsOpen())

	cp.Close()
}

func newShmIPCConnPoolForTest() remote.ConnPool {
	return NewShmIPCPool(
		&Config{
			UnixPathBuilder: func(network, address string) string {
				return shmUDS.String()
			},
			SMConfigBuilder: func(network, address string) *shmipc.SessionManagerConfig {
				smc := testShmipcConfig("test", shmUDS)
				return smc
			},
		})
}
