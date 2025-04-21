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

package ttstream

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/streaming"
)

func TestCancelStreamTaskConfig(t *testing.T) {
	// default
	rawCp := NewClientProvider()
	test.Assert(t, ticker != nil, ticker)
	cp := rawCp.(*clientProvider)
	test.Assert(t, cp.cancelStreamTaskConfig == defaultCancelStreamTaskConfig)
	ticker = nil
	// enable
	rawCp = NewClientProvider(WithClientCancelStreamTask(CancelStreamTaskConfig{Disable: true}))
	test.Assert(t, ticker == nil, ticker)
	cp = rawCp.(*clientProvider)
	test.Assert(t, cp.cancelStreamTaskConfig.Disable)
	// interval
	interval := 1 * time.Second
	rawCp = NewClientProvider(WithClientCancelStreamTask(CancelStreamTaskConfig{TriggerInterval: interval}))
	test.Assert(t, ticker != nil, ticker)
	cp = rawCp.(*clientProvider)
	test.Assert(t, cp.cancelStreamTaskConfig.TriggerInterval == interval)
	ticker = nil
	// sync
	rawCp = NewClientProvider(WithClientCancelStreamTask(CancelStreamTaskConfig{Sync: true}))
	test.Assert(t, ticker != nil, ticker)
	cp = rawCp.(*clientProvider)
	test.Assert(t, cp.cancelStreamTaskConfig.Sync)
	ticker = nil
}

func TestCancelStreamTask(t *testing.T) {
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)
	// initialize ticker
	NewClientProvider(WithClientCancelStreamTask(CancelStreamTaskConfig{TriggerInterval: 100 * time.Millisecond}))

	ctrans := newTransport(clientTransport, cconn, nil, true)
	cCtx, cancel := context.WithCancel(context.Background())
	rawClientStream, err := ctrans.WriteStream(cCtx, "ClientStreaming", make(IntHeader), make(streaming.Header))
	test.Assert(t, err == nil, err)
	strans := newTransport(serverTransport, sconn, nil, false)
	sCtx := context.Background()
	rawServerStream, err := strans.ReadStream(sCtx)
	test.Assert(t, err == nil, err)

	cs := newClientStream(rawClientStream)
	ss := newServerStream(rawServerStream)

	// todo: verify err returned by interface
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		go func() {
			time.Sleep(1 * time.Second)
			cancel()
		}()
		for {
			req := new(testRequest)
			req.A = 123
			if sErr := cs.SendMsg(cCtx, req); sErr != nil {
				//test.Assert(t, sErr == defaultCancelStreamTaskConfig.CancelException, sErr)
				wg.Done()
				break
			}
		}
	}()

	for {
		if rErr := ss.RecvMsg(sCtx, new(testResponse)); rErr != nil {
			//test.Assert(t, errors.Is(rErr, newExceptionType(context.Canceled.Error(), nil, 12008)), rErr)
			wg.Done()
			break
		}
	}
	wg.Wait()

	cCtx, cancel = context.WithCancel(context.Background())
	rawClientStream, err = ctrans.WriteStream(cCtx, "ServerStreaming", make(IntHeader), make(streaming.Header))
	test.Assert(t, err == nil, err)
	sCtx = context.Background()
	rawServerStream, err = strans.ReadStream(sCtx)
	test.Assert(t, err == nil, err)

	cs = newClientStream(rawClientStream)
	ss = newServerStream(rawClientStream)

	wg.Add(2)
	go func() {
		go func() {
			time.Sleep(1 * time.Second)
			cancel()
		}()
		for {
			if rErr := cs.RecvMsg(cCtx, new(testResponse)); rErr != nil {
				wg.Done()
				break
			}
		}
	}()
	for {
		resp := new(testResponse)
		if sErr := ss.SendMsg(sCtx, resp); sErr != nil {
			wg.Done()
			break
		}
	}
	wg.Wait()

	cCtx, cancel = context.WithCancel(context.Background())
	rawClientStream, err = ctrans.WriteStream(cCtx, "BidiStreaming", make(IntHeader), make(streaming.Header))
	test.Assert(t, err == nil, err)
	sCtx = context.Background()
	rawServerStream, err = strans.ReadStream(sCtx)
	test.Assert(t, err == nil, err)

	cs = newClientStream(rawClientStream)
	ss = newServerStream(rawClientStream)

	wg.Add(2)
	go func() {
		go func() {
			time.Sleep(1 * time.Second)
			cancel()
		}()
		for {
			req := new(testRequest)
			if sErr := cs.SendMsg(cCtx, req); sErr != nil {
				wg.Done()
				break
			}
			if rErr := cs.RecvMsg(cCtx, new(testResponse)); rErr != nil {
				wg.Done()
				break
			}
		}
	}()
	for {
		if rErr := cs.RecvMsg(sCtx, new(testRequest)); rErr != nil {
			wg.Done()
			break
		}
		resp := new(testResponse)
		if sErr := ss.SendMsg(sCtx, resp); sErr != nil {
			wg.Done()
			break
		}
	}
	wg.Wait()
}
