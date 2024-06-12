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

// Package remotecli for remote client
package remotecli

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/transport"
)

type StreamCleaner struct {
	Async   bool
	Timeout time.Duration
	Clean   func(st streaming.Stream) error
}

var registeredCleaner = map[transport.Protocol]*StreamCleaner{}

// RegisterCleaner registers a StreamCleaner associated with the given protocol
func RegisterCleaner(protocol transport.Protocol, cleaner *StreamCleaner) {
	registeredCleaner[protocol] = cleaner
}

// NewStream create a client side stream
func NewStream(ctx context.Context, ri rpcinfo.RPCInfo, handler remote.ClientTransHandler, opt *remote.ClientOption) (streaming.Stream, *StreamConnManager, error) {
	cm := NewConnWrapper(opt.ConnPool)
	var err error
	for _, shdlr := range opt.StreamingMetaHandlers {
		ctx, err = shdlr.OnConnectStream(ctx)
		if err != nil {
			return nil, nil, err
		}
	}
	rawConn, err := cm.GetConn(ctx, opt.Dialer, ri)
	if err != nil {
		return nil, nil, err
	}

	allocator, ok := handler.(remote.ClientStreamAllocator)
	if !ok {
		err = fmt.Errorf("remote.ClientStreamAllocator is not implemented by %T", handler)
		return nil, nil, remote.NewTransError(remote.InternalError, err)
	}

	st, err := allocator.NewStream(ctx, opt.SvcInfo, rawConn, handler)
	if err != nil {
		return nil, nil, err
	}
	return st, &StreamConnManager{cm}, nil
}

// NewStreamConnManager returns a new StreamConnManager
func NewStreamConnManager(cr ConnReleaser) *StreamConnManager {
	return &StreamConnManager{ConnReleaser: cr}
}

// StreamConnManager manages the underlying connection of the stream
type StreamConnManager struct {
	ConnReleaser
}

// ReleaseConn releases the raw connection of the stream
func (scm *StreamConnManager) ReleaseConn(st streaming.Stream, err error, ri rpcinfo.RPCInfo) {
	if err != nil {
		// for non-nil error, just pass it to the ConnReleaser
		scm.ConnReleaser.ReleaseConn(err, ri)
		return
	}

	cleaner, exists := registeredCleaner[ri.Config().TransportProtocol()]
	if !exists {
		scm.ConnReleaser.ReleaseConn(nil, ri)
		return
	}

	if cleaner.Async {
		scm.AsyncCleanAndRelease(cleaner, st, ri)
	} else {
		cleanErr := cleaner.Clean(st)
		scm.ConnReleaser.ReleaseConn(cleanErr, ri)
	}
}

// AsyncCleanAndRelease releases the raw connection of the stream asynchronously if an async cleaner is registered
func (scm *StreamConnManager) AsyncCleanAndRelease(cleaner *StreamCleaner, st streaming.Stream, ri rpcinfo.RPCInfo) {
	gopool.Go(func() {
		var finalErr error
		defer func() {
			if panicInfo := recover(); panicInfo != nil {
				finalErr = fmt.Errorf("panic=%v, stack=%s", panicInfo, debug.Stack())
			}
			if finalErr != nil {
				klog.Debugf("cleaner failed: %v", finalErr)
			}
			scm.ConnReleaser.ReleaseConn(finalErr, ri)
		}()
		finished := make(chan error, 1)
		t := time.NewTimer(cleaner.Timeout)
		gopool.Go(func() {
			var cleanErr error
			defer func() {
				if panicInfo := recover(); panicInfo != nil {
					cleanErr = fmt.Errorf("panic=%v, stack=%s", panicInfo, debug.Stack())
				}
				finished <- cleanErr
			}()
			cleanErr = cleaner.Clean(st)
		})
		select {
		case <-t.C:
			finalErr = context.DeadlineExceeded
		case cleanErr := <-finished:
			finalErr = cleanErr
		}
	})
}
