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

package remotecli

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// Client implementation may be different between mux and non-mux clients.
type Client interface {
	// RPCInfo as param just avoid to get it from ctx
	Send(ctx context.Context, ri rpcinfo.RPCInfo, req remote.Message) (err error)
	Recv(ctx context.Context, ri rpcinfo.RPCInfo, resp remote.Message) (err error)
	Recycle()
}

var clientPool = &sync.Pool{
	New: func() interface{} {
		return new(client)
	},
}

type client struct {
	transHdlr   remote.TransHandler
	connManager *ConnWrapper
	conn        net.Conn
	unary       bool
}

// NewClient creates a new Client using the given params.
func NewClient(ctx context.Context, ri rpcinfo.RPCInfo, handler remote.TransHandler, opt *remote.ClientOption) (Client, error) {
	cm := NewConnWrapper(opt.ConnPool)
	var err error
	// used by streaming unary
	for _, shdlr := range opt.StreamingMetaHandlers {
		ctx, err = shdlr.OnConnectStream(ctx)
		if err != nil {
			cm.zero()
			connWrapperPool.Put(cm)
			return nil, err
		}
	}
	dialer := opt.Dialer
	call := remote.UnaryCallFromContext(ctx)
	if call != nil {
		call.ConnectStart = time.Now()
		dialer = call.Dialer(dialer)
	}
	rawConn, err := cm.GetConn(ctx, dialer, ri)
	if call != nil {
		call.ConnectEnd, call.Conn = time.Now(), rawConn
		if err != nil {
			call.FailureStage = remote.UnaryFailureConnect
		} else if identified, ok := cm.conn.(interface{ ConnID() uint64 }); ok {
			call.ConnID = identified.ConnID()
		} else if opt.ConnPool == nil {
			call.ConnID = remote.NextUnaryConnID()
		}
	}
	if err != nil {
		cm.zero()
		connWrapperPool.Put(cm)
		if errors.Is(err, kerrors.ErrGetConnection) {
			return nil, err
		}
		return nil, kerrors.ErrGetConnection.WithCause(err)
	}
	cli := clientPool.Get().(*client)
	cli.init(handler, cm, rawConn)
	cli.unary = call != nil
	return cli, nil
}

func (c *client) Recycle() {
	if c.unary && c.connManager != nil {
		c.connManager.ReleaseConn(errors.New("client recycled before IO completed"), nil)
	}
	c.transHdlr = nil
	c.connManager = nil
	c.conn = nil
	c.unary = false
	clientPool.Put(c)
}

func (c *client) init(handler remote.TransHandler, cm *ConnWrapper, conn net.Conn) {
	c.transHdlr = handler
	c.connManager = cm
	c.conn = conn
}

// Send is blocked.
func (c *client) Send(ctx context.Context, ri rpcinfo.RPCInfo, req remote.Message) (err error) {
	_, err = c.transHdlr.Write(ctx, c.conn, req)
	if err != nil {
		if call := remote.UnaryCallFromContext(ctx); call != nil {
			call.FailureStage = remote.UnaryFailureSend
			err = call.FinishClient(ctx, err)
		}
		c.connManager.ReleaseConn(err, ri)
		c.connManager = nil
	}
	return err
}

// Recv is blocked.
func (c *client) Recv(ctx context.Context, ri rpcinfo.RPCInfo, resp remote.Message) (err error) {
	// resp is nil means oneway
	if resp != nil {
		ctx, err = c.transHdlr.Read(ctx, c.conn, resp)
		if call := remote.UnaryCallFromContext(ctx); call != nil {
			err = call.FinishClient(ctx, err)
			if err == nil {
				ctx, err = c.transHdlr.OnMessage(ctx, nil, resp)
			}
		} else {
			c.transHdlr.OnMessage(ctx, nil, resp)
		}
	} else {
		// Wait for the request to be flushed out before closing the connection.
		// 500us is an acceptable duration to keep a minimal loss rate at best effort.
		time.Sleep(time.Millisecond / 2)
	}

	c.connManager.ReleaseConn(err, ri)
	c.connManager = nil
	return err
}
