package ttstream

import (
	"context"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/streamx"
	"github.com/cloudwego/netpoll"
	"log"
	"runtime"
	"time"
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
	connPool     remote.ConnPool
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

	conn, err := netpoll.DialConnection(addr.Network(), addr.String(), time.Second)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(time.Hour))
	_ = conn.SetReadTimeout(time.Hour)
	trans := newTransport(clientTransport, c.sinfo, conn)
	s, err := trans.newStream(ctx, method)
	if err != nil {
		return nil, err
	}
	cs := newClientStream(s)
	runtime.SetFinalizer(cs, func(cs *clientStream) {
		log.Printf("client stream[%v] closing", cs.sid)
		_ = cs.close()
		// TODO: current using one conn one stream
		_ = trans.close()
	})
	return cs, err
}
