package jsonrpc

import (
	"context"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/streamx"
	"net"
)

type serverTransCtxKey struct{}

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
	sinfo        *serviceinfo.ServiceInfo
	payloadLimit int
}

func (s serverProvider) Available(ctx context.Context, conn net.Conn) bool {
	return true
}

func (s serverProvider) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	trans := newTransport(s.sinfo, conn)
	return context.WithValue(ctx, serverTransCtxKey{}, trans), nil
}

func (s serverProvider) OnInactive(ctx context.Context, conn net.Conn) (context.Context, error) {
	return ctx, nil
}

func (s serverProvider) OnStream(ctx context.Context, conn net.Conn) (context.Context, streamx.ServerStream, error) {
	trans, _ := ctx.Value(serverTransCtxKey{}).(*transport)
	if trans == nil {
		return nil, nil, nil
	}
	st, err := trans.readStream()
	if err != nil {
		return nil, nil, err
	}
	ss := newServerStream(st)
	return ctx, ss, nil
}

func (s serverProvider) OnStreamFinish(ctx context.Context, ss streamx.ServerStream) (context.Context, error) {
	sst := ss.(*serverStream)
	err := sst.sendEOF()
	return ctx, err
}
