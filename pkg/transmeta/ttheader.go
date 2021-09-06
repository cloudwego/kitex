package transmeta

import (
	"context"
	"strconv"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/jackedelic/kitex/internal/pkg/remote"
	"github.com/jackedelic/kitex/internal/pkg/remote/transmeta"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
	"github.com/jackedelic/kitex/internal/transport"
)

const (
	framedTransportType   = "framed"
	unframedTransportType = "unframed"
)

// TTHeader handlers.
var (
	ClientTTHeaderHandler remote.MetaHandler = &clientTTHeaderHandler{}
	ServerTTHeaderHandler remote.MetaHandler = &serverTTHeaderHandler{}
)

// clientTTHeaderHandler implement remote.MetaHandler
type clientTTHeaderHandler struct{}

// WriteMeta of clientTTHeaderHandler writes headers of TTHeader protocol to transport
func (ch *clientTTHeaderHandler) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	if !isTTHeader(msg) {
		return ctx, nil
	}
	ri := msg.RPCInfo()
	transInfo := msg.TransInfo()

	hd := map[uint16]string{
		transmeta.FromService: ri.From().ServiceName(),
		transmeta.FromMethod:  ri.From().Method(),
		transmeta.ToService:   ri.To().ServiceName(),
		transmeta.ToMethod:    ri.To().Method(),
		transmeta.MsgType:     strconv.Itoa(int(msg.MessageType())),
	}
	if msg.ProtocolInfo().TransProto&transport.Framed == transport.Framed {
		hd[transmeta.TransportType] = framedTransportType
	} else {
		hd[transmeta.TransportType] = unframedTransportType
	}

	transInfo.PutTransIntInfo(hd)

	if metainfo.HasMetaInfo(ctx) {
		hd := make(map[string]string)
		metainfo.SaveMetaInfoToMap(ctx, hd)
		transInfo.PutTransStrInfo(hd)
	}
	return ctx, nil
}

// ReadMeta of clientTTHeaderHandler reads headers of TTHeader protocol from transport
func (ch *clientTTHeaderHandler) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	// do nothing
	return ctx, nil
}

// serverTTHeaderHandler implement remote.MetaHandler
type serverTTHeaderHandler struct{}

// ReadMeta of serverTTHeaderHandler reads headers of TTHeader protocol to transport
func (sh *serverTTHeaderHandler) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	if !isTTHeader(msg) {
		return ctx, nil
	}
	ri := msg.RPCInfo()
	transInfo := msg.TransInfo()
	strInfo := transInfo.TransStrInfo()
	intInfo := transInfo.TransIntInfo()

	ctx = metainfo.SetMetaInfoFromMap(ctx, strInfo)
	ci := rpcinfo.AsMutableEndpointInfo(ri.From())
	if ci != nil {
		if v := intInfo[transmeta.FromService]; v != "" {
			ci.SetServiceName(v)
		}
		if v := intInfo[transmeta.FromMethod]; v != "" {
			ci.SetMethod(v)
		}
	}
	return ctx, nil
}

// WriteMeta of serverTTHeaderHandler writes headers of TTHeader protocol to transport
func (sh *serverTTHeaderHandler) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	if !isTTHeader(msg) {
		return ctx, nil
	}
	hd := map[uint16]string{
		transmeta.MsgType: strconv.Itoa(int(msg.MessageType())),
	}
	msg.TransInfo().PutTransIntInfo(hd)
	return ctx, nil
}

func isTTHeader(msg remote.Message) bool {
	transProto := msg.ProtocolInfo().TransProto
	return transProto&transport.TTHeader == transport.TTHeader
}
