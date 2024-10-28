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
	"net"
	"strconv"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/ktx"
	"github.com/cloudwego/kitex/pkg/utils"
)

type (
	serverTransCtxKey        struct{}
	serverStreamCancelCtxKey struct{}
)

func NewServerProvider(sinfo *serviceinfo.ServiceInfo, opts ...ServerProviderOption) (streamx.ServerProvider, error) {
	sp := new(serverProvider)
	sp.sinfo = sinfo
	for _, opt := range opts {
		opt(sp)
	}
	return sp, nil
}

var _ streamx.ServerProvider = (*serverProvider)(nil)

type serverProvider struct {
	sinfo         *serviceinfo.ServiceInfo
	metaHandler   MetaFrameHandler
	headerHandler HeaderFrameReadHandler
}

func (s serverProvider) Available(ctx context.Context, conn net.Conn) bool {
	data, err := conn.(netpoll.Connection).Reader().Peek(8)
	if err != nil {
		return false
	}
	return ttheader.IsStreaming(data)
}

func (s serverProvider) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	nconn := conn.(netpoll.Connection)
	trans := newTransport(serverTransport, s.sinfo, nconn)
	trans.setMetaFrameHandler(s.metaHandler)
	_ = nconn.(onDisConnectSetter).SetOnDisconnect(func(ctx context.Context, connection netpoll.Connection) {
		// server only close transport when peer connection closed
		_ = trans.Close()
	})
	return context.WithValue(ctx, serverTransCtxKey{}, trans), nil
}

func (s serverProvider) OnInactive(ctx context.Context, conn net.Conn) (context.Context, error) {
	trans, _ := ctx.Value(serverTransCtxKey{}).(*transport)
	if trans == nil {
		return ctx, nil
	}
	trans.WaitClosed()
	return ctx, nil
}

func (s serverProvider) OnStream(ctx context.Context, conn net.Conn) (context.Context, streamx.ServerStream, error) {
	trans, _ := ctx.Value(serverTransCtxKey{}).(*transport)
	if trans == nil {
		return nil, nil, nil
	}
	st, err := trans.readStream(ctx)
	if err != nil {
		return nil, nil, err
	}
	if s.headerHandler != nil {
		ctx, err = s.headerHandler.OnReadStream(ctx, st.meta, st.header)
		if err != nil {
			return nil, nil, err
		}
	}
	ctx = metainfo.SetMetaInfoFromMap(ctx, st.header)
	ss := newServerStream(st)

	ctx, cancelFunc := ktx.WithCancel(ctx)
	ctx = context.WithValue(ctx, serverStreamCancelCtxKey{}, cancelFunc)
	return ctx, ss, nil
}

func (s serverProvider) OnStreamFinish(ctx context.Context, ss streamx.ServerStream, err error) (context.Context, error) {
	sst := ss.(*serverStream)
	var exception tException
	if err != nil {
		switch terr := err.(type) {
		case kerrors.BizStatusErrorIface:
			bizStatus := strconv.Itoa(int(terr.BizStatusCode()))
			bizMsg := terr.BizMessage()
			if terr.BizExtra() == nil {
				err = sst.writeTrailer(streamx.Trailer{
					"biz-status":  bizStatus,
					"biz-message": bizMsg,
				})
			} else {
				bizExtra, _ := utils.Map2JSONStr(terr.BizExtra())
				err = sst.writeTrailer(streamx.Trailer{
					"biz-status":  bizStatus,
					"biz-message": bizMsg,
					"biz-extra":   bizExtra,
				})
			}
			if err != nil {
				return nil, err
			}
		case tException:
			exception = terr
		default:
			exception = thrift.NewApplicationException(remote.InternalError, terr.Error())
		}
	}
	if err = sst.close(exception); err != nil {
		return nil, err
	}

	cancelFunc, _ := ctx.Value(serverStreamCancelCtxKey{}).(context.CancelFunc)
	if cancelFunc != nil {
		cancelFunc()
	}

	return ctx, nil
}

type onDisConnectSetter interface {
	SetOnDisconnect(onDisconnect netpoll.OnDisconnect) error
}
