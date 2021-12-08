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

package netpollmux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/klog"
	np "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
)

// ErrConnClosed .
var ErrConnClosed = errors.New("conn closed")

// SharedSize .
var SharedSize = int32(runtime.GOMAXPROCS(0))

func newMuxCliConn(connection netpoll.Connection) *muxCliConn {
	c := &muxCliConn{
		muxConn:  newMuxConn(connection),
		seqIDMap: newSharedMap(SharedSize),
	}
	connection.SetOnRequest(c.OnRequest)
	return c
}

type muxCliConn struct {
	muxConn
	seqIDMap *sharedMap // (k,v) is (sequenceID, notify)
}

// OnRequest is called when the connection creates.
func (c *muxCliConn) OnRequest(ctx context.Context, connection netpoll.Connection) (err error) {
	// check protocol header
	length, seqID, err := parseHeader(connection.Reader())
	if err != nil {
		err = fmt.Errorf("%w: addr(%s)", err, connection.RemoteAddr())
		return c.onError(err, connection)
	}
	// reader is nil if return error
	reader, err := connection.Reader().Slice(length)
	if err != nil {
		err = fmt.Errorf("mux read package slice failed: addr(%s), %w", connection.RemoteAddr(), err)
		return c.onError(err, connection)
	}
	go func() {
		asyncCallback, ok := c.seqIDMap.load(seqID)
		if !ok {
			reader.(io.Closer).Close()
			return
		}
		bufReader := np.NewReaderByteBuffer(reader)
		asyncCallback.Recv(bufReader, nil)
	}()
	return nil
}

// Close does nothing.
func (c *muxCliConn) Close() error {
	return nil
}

func (c *muxCliConn) close() error {
	c.Connection.Close()
	c.seqIDMap.rangeMap(func(seqID int32, msg EventHandler) {
		msg.Recv(nil, ErrConnClosed)
	})
	return nil
}

func (c *muxCliConn) onError(err error, connection netpoll.Connection) error {
	klog.Errorf("KITEX: error=%s", err.Error())
	connection.Close()
	return err
}

func newMuxSvrConn(connection netpoll.Connection, pool *sync.Pool) *muxSvrConn {
	c := &muxSvrConn{
		muxConn: newMuxConn(connection),
		pool:    pool,
	}
	return c
}

type muxSvrConn struct {
	muxConn
	pool *sync.Pool // ctx with rpcInfo
}

func newMuxConn(connection netpoll.Connection) muxConn {
	c := muxConn{}
	c.Connection = connection
	c.sharedQueue = newSharedQueue(SharedSize, connection)
	return c
}

var (
	_ net.Conn           = &muxConn{}
	_ netpoll.Connection = &muxConn{}
)

type muxConn struct {
	netpoll.Connection              // raw conn
	sharedQueue        *sharedQueue // use for write
}

// Put puts the buffer getter back to the queue.
func (c *muxConn) Put(gt BufferGetter) {
	c.sharedQueue.Add(gt)
}
