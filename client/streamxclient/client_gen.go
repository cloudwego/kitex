package streamxclient

import (
	"context"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/streamx"
	"log"
)

func InvokeStream[Header, Trailer, Req, Res any](
	ctx context.Context, cli client.StreamX, smode serviceinfo.StreamingMode, method string,
	req *Req, res *Res, callOptions ...streamxcallopt.CallOption,
) (stream *streamx.GenericClientStream[Header, Trailer, Req, Res], err error) {
	reqArgs, resArgs := streamx.NewStreamReqArgs(nil), streamx.NewStreamResArgs(nil)
	streamArgs := streamx.NewStreamArgs(nil)
	// important notes: please don't set a typed nil value into interface arg like NewStreamReqArgs({typ: *Res, ptr: nil})
	// otherwise, reqArgs.Req() may not ==nil forever
	if req != nil {
		reqArgs.SetReq(req)
	}
	if res != nil {
		resArgs.SetRes(res)
	}

	log.Printf("Client invoke new stream start: method=%s", method)
	cs, err := cli.NewStream(ctx, method, req, callOptions...)
	log.Printf("Client invoke new stream end: method=%s", method)
	if err != nil {
		return nil, err
	}
	stream = streamx.NewGenericClientStream[Header, Trailer, Req, Res](cs)
	streamx.AsMutableStreamArgs(streamArgs).SetStream(stream)

	streamMW, recvMW, sendMW := cli.Middlewares()
	stream.SetStreamRecvMiddleware(recvMW)
	stream.SetStreamSendMiddleware(sendMW)

	err = streamMW(
		func(ctx context.Context, streamArgs streamx.StreamArgs, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs) (err error) {
			// assemble streaming args depend on each stream mode
			switch smode {
			case serviceinfo.StreamingUnary:
				if err = stream.SendMsg(ctx, req); err != nil {
					return err
				}
				if err = stream.RecvMsg(ctx, res); err != nil {
					return err
				}
				resArgs.SetRes(res)
				if err = stream.CloseSend(ctx); err != nil {
					return err
				}
			case serviceinfo.StreamingClient:
			case serviceinfo.StreamingServer:
				if err = stream.SendMsg(ctx, req); err != nil {
					return err
				}
				if err = stream.CloseSend(ctx); err != nil {
					return err
				}
			case serviceinfo.StreamingBidirectional:
			default:
			}
			return nil
		})(ctx, streamArgs, reqArgs, resArgs)
	if err != nil {
		return nil, err
	}
	return stream, nil
}
