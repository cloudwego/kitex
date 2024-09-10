package ttstream

import (
	"context"
	"log"
	"runtime"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/streamx"
)

var _ streamx.ClientProvider = (*clientProvider)(nil)

func NewClientProvider(sinfo *serviceinfo.ServiceInfo, opts ...ClientProviderOption) (streamx.ClientProvider, error) {
	cp := new(clientProvider)
	cp.transPool = newTransPool(sinfo)
	cp.sinfo = sinfo
	for _, opt := range opts {
		opt(cp)
	}
	return cp, nil
}

type clientProvider struct {
	transPool    *transPool
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

	trans, err := c.transPool.Get(addr.Network(), addr.String())
	if err != nil {
		return nil, err
	}

	header := map[string]string{
		ttheader.HeaderIDLServiceName: c.sinfo.ServiceName,
	}
	metainfo.SaveMetaInfoToMap(ctx, header)
	s, err := trans.newStream(ctx, method, header)
	if err != nil {
		return nil, err
	}
	cs := newClientStream(s)
	runtime.SetFinalizer(cs, func(cs *clientStream) {
		log.Printf("client stream[%v] closing", cs.sid)
		_ = cs.close()
		// TODO: current using one conn one stream
		//_ = trans.close()
		c.transPool.Put(trans)
	})
	return cs, err
}
