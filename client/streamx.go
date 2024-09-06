package client

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/streamx"
)

type StreamX interface {
	NewStream(ctx context.Context, method string, req any, callOptions ...streamxcallopt.CallOption) (streamx.ClientStream, error)
	StreamMiddleware(next streamx.StreamEndpoint) streamx.StreamEndpoint
}

// TODO: 使用这个中间件
func (kc *kClient) invokeStreamXIOMiddleware() streamx.StreamMiddleware {
	return func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
		return func(ctx context.Context, req streamx.StreamReqArgs, res streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
			// 这个函数要放到最开始，也就是说所有其他中间件都调用完了，这个才能调用
			if streamArgs == nil {
				return errors.New("stream cannot be nil")
			}
			st := streamArgs.Stream()
			ri := rpcinfo.GetRPCInfo(ctx)
			method := ri.Invocation().MethodName()
			switch kc.svcInfo.Methods[method].StreamingMode() {
			case serviceinfo.StreamingUnary:
				if req == nil || res == nil {
					return errors.New("req or res is nil")
				}
				err = st.SendMsg(ctx, req)
				if err != nil {
					return err
				}
				err = st.RecvMsg(ctx, res)
				if err != nil {
					return err
				}
			case serviceinfo.StreamingServer:
				err = st.SendMsg(ctx, req)
				if err != nil {
					return err
				}
			case serviceinfo.StreamingClient: // do nothing
			case serviceinfo.StreamingBidirectional: // do nothing
			}
			return next(ctx, req, res, streamArgs)
		}
	}
}

// return a bottom next function
// bottom next function will create a stream and change the streamx.Args
func (kc *kClient) invokeStreamXEndpoint() (endpoint.Endpoint, error) {
	// TODO: implement trans handler layer and use trans factory
	//transPipl, err := newCliTransHandler(kc.opt.RemoteOpt)
	//if err != nil {
	//	return nil, err
	//}
	clientProvider, _ := kc.opt.RemoteOpt.Provider.(streamx.ClientProvider)
	clientProvider = streamx.NewClientProvider(clientProvider) // wrap client provider

	return func(ctx context.Context, req, resp interface{}) (err error) {
		ri := rpcinfo.GetRPCInfo(ctx)
		cs, err := clientProvider.NewStream(ctx, ri)
		if err != nil {
			return err
		}
		streamArgs := resp.(streamx.StreamArgs)
		// 此后的中间件才会有 Stream
		streamx.AsMutableStreamArgs(streamArgs).SetStream(cs)
		return nil
	}, nil
}

// NewStream create stream for streamx mode
func (kc *kClient) NewStream(ctx context.Context, method string, req any, callOptions ...streamxcallopt.CallOption) (streamx.ClientStream, error) {
	if !kc.inited {
		panic("client not initialized")
	}
	if kc.closed {
		panic("client is already closed")
	}
	if ctx == nil {
		panic("ctx is nil")
	}
	var ri rpcinfo.RPCInfo
	ctx, ri, _ = kc.initRPCInfo(ctx, method, 0, nil)

	err := rpcinfo.AsMutableRPCConfig(ri.Config()).SetInteractionMode(rpcinfo.Streaming)
	if err != nil {
		return nil, err
	}
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ctx = kc.opt.TracerCtl.DoStart(ctx, ri)

	streamArgs := streamx.NewStreamArgs(nil)
	// put streamArgs into response arg
	// it's an ugly trick but if we don't want to refactor too much,
	// this is the only way to compatible with current endpoint design
	err = kc.sxEps(ctx, req, streamArgs)
	if err != nil {
		return nil, err
	}
	return streamArgs.Stream().(streamx.ClientStream), nil
}

func (kc *kClient) StreamMiddleware(next streamx.StreamEndpoint) streamx.StreamEndpoint {
	if kc.sxMW == nil {
		return next
	}
	if next == nil {
		return kc.sxMW(func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
			return nil
		})
	}
	return kc.sxMW(next)
}
