// Package transmeta metadata handler for translation
package transmeta

import (
	"context"

	"github.com/jackedelic/kitex/internal/pkg/remote"
	"github.com/jackedelic/kitex/internal/pkg/remote/trans/nphttp2/metadata"
	"github.com/jackedelic/kitex/internal/pkg/remote/transmeta"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
	"github.com/jackedelic/kitex/internal/transport"

	"github.com/bytedance/gopkg/cloud/metainfo"
)

// ClientHTTP2Handler default global client metadata handler
var ClientHTTP2Handler = &clientHTTP2Handler{}

type clientHTTP2Handler struct{}

func (*clientHTTP2Handler) OnConnectStream(ctx context.Context) (context.Context, error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if !isGRPC(ri) {
		return ctx, nil
	}
	md := metadata.MD{}
	md.Append(transmeta.HTTPDestService, ri.To().ServiceName())
	md.Append(transmeta.HTTPDestMethod, ri.To().Method())
	md.Append(transmeta.HTTPSourceService, ri.From().ServiceName())
	md.Append(transmeta.HTTPSourceMethod, ri.From().Method())
	// get custom kv from metainfo and set to grpc metadata
	metainfo.ToHTTPHeader(ctx, metainfo.HTTPHeader(md))
	return metadata.NewOutgoingContext(ctx, md), nil
}

func (*clientHTTP2Handler) OnReadStream(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (ch *clientHTTP2Handler) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	return ctx, nil
}

func (ch *clientHTTP2Handler) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	return ctx, nil
}

// ServerHTTP2Handler default global server metadata handler
var ServerHTTP2Handler = &serverHTTP2Handler{}

type serverHTTP2Handler struct{}

func (*serverHTTP2Handler) OnConnectStream(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (*serverHTTP2Handler) OnReadStream(ctx context.Context) (context.Context, error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if !isGRPC(ri) {
		return ctx, nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, nil
	}
	// get custom kv from metadata and set to context
	ctx = metainfo.FromHTTPHeader(ctx, metainfo.HTTPHeader(md))
	ctx = metainfo.TransferForward(ctx)
	ci := rpcinfo.AsMutableEndpointInfo(ri.From())
	if ci != nil {
		if v := md.Get(transmeta.HTTPSourceService); len(v) != 0 {
			ci.SetServiceName(v[0])
		}
		if v := md.Get(transmeta.HTTPSourceMethod); len(v) != 0 {
			ci.SetMethod(v[0])
		}
	}
	return ctx, nil
}

func (sh *serverHTTP2Handler) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	return ctx, nil
}

func (sh *serverHTTP2Handler) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	return ctx, nil
}

func isGRPC(ri rpcinfo.RPCInfo) bool {
	return ri.Config().TransportProtocol()&transport.GRPC == transport.GRPC
}
