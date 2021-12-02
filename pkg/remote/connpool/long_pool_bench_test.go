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

package connpool

import (
	"context"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/mocks"
	dialer "github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
)

func newLongPoolWithoutDelayDestroyForTest(peerAddr, global int, timeout time.Duration) *LongPool {
	lp := newLongPoolForTest(peerAddr, global, timeout)
	limit := utils.NewMaxCounter(global)
	lp.newPeer = func(addr net.Addr) *peer {
		return newPeer("test", addr, peerAddr, timeout, limit, false)
	}
	return lp
}

func newDialerForBench() *dialer.SynthesizedDialer {
	d := &dialer.SynthesizedDialer{}
	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
		na := utils.NewNetAddr(network, address)
		return &mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
		}, nil
	}
	return d
}

func BenchmarkLongPoolDelayDestroy(b *testing.B) {
	p := newLongPoolForTest(1, 1, time.Second)
	d := newDialerForBench()
	b.SetParallelism(1000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := p.Get(context.TODO(), "tcp", mockAddr0, dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second})
			if err != nil {
				b.Fatal(err)
			}
			// send request
			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			err = p.Put(conn)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkLongPoolWithoutDelayDestroy(b *testing.B) {
	p := newLongPoolWithoutDelayDestroyForTest(1, 1, time.Second)
	d := newDialerForBench()
	b.SetParallelism(1000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := p.Get(context.TODO(), "tcp", mockAddr0, dialer.ConnOption{Dialer: d, ConnectTimeout: time.Second})
			if err != nil {
				b.Fatal(err)
			}
			// send request
			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			err = p.Put(conn)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
