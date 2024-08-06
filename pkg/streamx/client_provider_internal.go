package streamx

import (
	"context"

	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func NewClientProvider(cs ClientProvider) ClientProvider {
	return internalClientProvider{ClientProvider: cs}
}

type internalClientProvider struct {
	ClientProvider
}

func (p internalClientProvider) NewStream(ctx context.Context, ri rpcinfo.RPCInfo, callOptions ...streamxcallopt.CallOption) (ClientStream, error) {
	cs, err := p.ClientProvider.NewStream(ctx, ri, callOptions...)
	if err != nil {
		return nil, err
	}
	return cs, nil
}
