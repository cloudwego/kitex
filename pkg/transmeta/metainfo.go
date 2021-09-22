package transmeta

import (
	"context"

	"github.com/cloudwego/kitex/pkg/remote"

	"github.com/bytedance/gopkg/cloud/metainfo"
)

// singletons .
var (
	MetainfoClientHandler = new(metainfoClientHandler)
	MetainfoServerHandler = new(metainfoServerHandler)
)

type metainfoClientHandler struct{}

func (mi *metainfoClientHandler) WriteMeta(ctx context.Context, sendMsg remote.Message) (context.Context, error) {
	if sendMsg.ProtocolInfo().TransProto.WithMeta() && metainfo.HasMetaInfo(ctx) {
		kvs := make(map[string]string)
		metainfo.SaveMetaInfoToMap(ctx, kvs)
		sendMsg.TransInfo().PutTransStrInfo(kvs)
	}
	return ctx, nil
}

func (mi *metainfoClientHandler) ReadMeta(ctx context.Context, recvMsg remote.Message) (context.Context, error) {
	if info := recvMsg.TransInfo(); info != nil {
		if kvs := info.TransStrInfo(); len(kvs) > 0 {
			metainfo.SetBackwardValuesFromMap(ctx, kvs)
		}
	}
	return ctx, nil
}

type metainfoServerHandler struct{}

func (mi *metainfoServerHandler) ReadMeta(ctx context.Context, recvMsg remote.Message) (context.Context, error) {
	if recvMsg.ProtocolInfo().TransProto.WithMeta() {
		if kvs := recvMsg.TransInfo().TransStrInfo(); len(kvs) > 0 {
			ctx = metainfo.SetMetaInfoFromMap(ctx, kvs)
			ctx = metainfo.TransferForward(ctx)
		}
		ctx = metainfo.WithBackwardValuesToSend(ctx)
	}

	return ctx, nil
}

func (mi *metainfoServerHandler) WriteMeta(ctx context.Context, sendMsg remote.Message) (context.Context, error) {
	if sendMsg.ProtocolInfo().TransProto.WithMeta() {
		if kvs := metainfo.AllBackwardValuesToSend(ctx); len(kvs) > 0 {
			sendMsg.TransInfo().PutTransStrInfo(kvs)
		}
	}

	return ctx, nil
}
