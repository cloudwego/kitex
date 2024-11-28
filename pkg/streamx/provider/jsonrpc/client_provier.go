package jsonrpc

import (
	"context"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	"net"
)

var _ streamx.ClientProvider = (*clientProvider)(nil)

func NewClientProvider(sinfo *serviceinfo.ServiceInfo, opts ...ClientProviderOption) (streamx.ClientProvider, error) {
	cp := new(clientProvider)
	cp.sinfo = sinfo
	for _, opt := range opts {
		opt(cp)
	}
	return cp, nil
}

type clientProvider struct {
	sinfo        *serviceinfo.ServiceInfo
	payloadLimit int
}

func (c clientProvider) NewStream(ctx context.Context, ri rpcinfo.RPCInfo, callOptions ...streamxcallopt.CallOption) (streamx.ClientStream, error) {
	invocation := ri.Invocation()
	method := invocation.MethodName()
	addr := ri.To().Address()
	if addr == nil {
		return nil, kerrors.ErrNoDestAddress
	}
	conn, err := net.Dial(addr.Network(), addr.String())
	if err != nil {
		return nil, err
	}
	trans := newTransport(c.sinfo, conn)
	s, err := trans.newStream(method)
	if err != nil {
		return nil, err
	}
	cs := newClientStream(s)
	return cs, err
}
