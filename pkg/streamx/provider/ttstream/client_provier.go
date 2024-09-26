/*
 * Copyright 2024 CloudWeGo Authors
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
	"runtime"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/ktx"
)

var _ streamx.ClientProvider = (*clientProvider)(nil)

func NewClientProvider(sinfo *serviceinfo.ServiceInfo, opts ...ClientProviderOption) (streamx.ClientProvider, error) {
	cp := new(clientProvider)
	cp.sinfo = sinfo
	cp.transPool = newMuxTransPool()
	for _, opt := range opts {
		opt(cp)
	}
	return cp, nil
}

type clientProvider struct {
	transPool     transPool
	sinfo         *serviceinfo.ServiceInfo
	metaHandler   MetaFrameHandler
	headerHandler HeaderFrameHandler
}

func (c clientProvider) NewStream(ctx context.Context, ri rpcinfo.RPCInfo, callOptions ...streamxcallopt.CallOption) (streamx.ClientStream, error) {
	rconfig := ri.Config()
	invocation := ri.Invocation()
	method := invocation.MethodName()
	addr := ri.To().Address()
	if addr == nil {
		return nil, kerrors.ErrNoDestAddress
	}

	var strHeader streamx.Header
	var intHeader IntHeader
	var err error
	if c.headerHandler != nil {
		intHeader, strHeader, err = c.headerHandler.OnStream(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		intHeader = IntHeader{}
		strHeader = map[string]string{}
	}
	strHeader[ttheader.HeaderIDLServiceName] = c.sinfo.ServiceName
	metainfo.SaveMetaInfoToMap(ctx, strHeader)

	trans, err := c.transPool.Get(c.sinfo, addr.Network(), addr.String())
	if err != nil {
		return nil, err
	}

	s, err := trans.newStream(ctx, method, intHeader, strHeader)
	if err != nil {
		return nil, err
	}
	s.setRecvTimeout(rconfig.StreamRecvTimeout())
	// only client can set meta frame handler
	s.setMetaFrameHandler(c.metaHandler)

	// if ctx from server side, we should cancel the stream when server handler already returned
	// TODO: this canceling transmit should be configurable
	ktx.RegisterCancelCallback(ctx, func() {
		s.cancel()
	})

	cs := newClientStream(s)
	runtime.SetFinalizer(cs, func(cstream *clientStream) {
		_ = cstream.close()
		c.transPool.Put(trans)
	})
	return cs, err
}
