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

package netpoll

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/jackedelic/kitex/internal/mocks"
	"github.com/jackedelic/kitex/internal/pkg/remote"
	"github.com/jackedelic/kitex/internal/pkg/remote/trans"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
	"github.com/jackedelic/kitex/internal/test"
)

var cliTransHdlr remote.ClientTransHandler

func init() {
	var opt = &remote.ClientOption{
		Codec: &MockCodec{
			EncodeFunc: nil,
			DecodeFunc: nil,
		},
	}
	cliTransHdlr, _ = newCliTransHandler(opt)
}

func TestWrite(t *testing.T) {
	var isWriteBufFlushed bool
	conn := &MockNetpollConn{
		WriterFunc: func() (r netpoll.Writer) {
			writer := &MockNetpollWriter{
				FlushFunc: func() (err error) {
					isWriteBufFlushed = true
					return nil
				},
			}
			return writer
		},
	}
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())
	ctx := context.Background()
	var req interface{}
	var msg = remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Client)
	err := cliTransHdlr.Write(ctx, conn, msg)
	test.Assert(t, err == nil, err)
	test.Assert(t, isWriteBufFlushed)
}

func TestRead(t *testing.T) {
	rwTimeout := time.Second

	var readTimeout time.Duration
	var isReaderBufReleased bool
	conn := &MockNetpollConn{
		SetReadTimeoutFunc: func(timeout time.Duration) (e error) {
			readTimeout = timeout
			return nil
		},
		ReaderFunc: func() (r netpoll.Reader) {
			reader := &MockNetpollReader{
				ReleaseFunc: func() (err error) {
					isReaderBufReleased = true
					return nil
				},
			}
			return reader
		},
	}
	cfg := rpcinfo.NewRPCConfig()
	rpcinfo.AsMutableRPCConfig(cfg).SetReadWriteTimeout(rwTimeout)
	ri := rpcinfo.NewRPCInfo(nil, nil, nil, cfg, rpcinfo.NewRPCStats())
	ctx := context.Background()
	var req interface{}
	var msg = remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Reply, remote.Client)
	err := cliTransHdlr.Read(ctx, conn, msg)
	test.Assert(t, err == nil, err)
	test.Assert(t, readTimeout == trans.GetReadTimeout(ri.Config()))
	test.Assert(t, isReaderBufReleased)
}
