package streamx

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"log"
	"reflect"
)

func ServerInvoke[Header, Trailer, Req, Res any](
	ctx context.Context, smode serviceinfo.StreamingMode,
	shandler StreamHandler, reqArgs StreamReqArgs, resArgs StreamResArgs) (err error) {
	// prepare args
	sArgs := GetStreamArgsFromContext(ctx)
	if sArgs == nil {
		return errors.New("server stream is nil")
	}
	gs := NewGenericServerStream[Header, Trailer, Req, Res](sArgs.Stream())

	// before handler
	var req *Req
	var res *Res
	switch smode {
	case serviceinfo.StreamingUnary, serviceinfo.StreamingServer:
		req, err = gs.Recv(ctx)
		if err != nil {
			return err
		}
		reqArgs.(StreamReqArgs).SetReq(req)
	default:
	}

	// handler call
	// TODO: cache handler
	rhandler := reflect.ValueOf(shandler.Handler)
	mhandler := rhandler.MethodByName(sArgs.Stream().Method())
	return shandler.Middleware(
		func(ctx context.Context, reqArgs StreamReqArgs, resArgs StreamResArgs, streamArgs StreamArgs) (err error) {
			switch smode {
			case serviceinfo.StreamingUnary:
				called := mhandler.Call([]reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(req),
				})
				_res, _err := called[0].Interface(), called[1].Interface()
				if _err != nil {
					return _err.(error)
				}
				res = _res.(*Res)
				if err = gs.SendAndClose(ctx, res); err != nil {
					return err
				}
				resArgs.SetRes(res)
			case serviceinfo.StreamingClient:
				called := mhandler.Call([]reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(gs),
				})
				_res, _err := called[0].Interface(), called[1].Interface()
				if _err != nil {
					return _err.(error)
				}
				res = _res.(*Res)
				if err = gs.SendMsg(ctx, res); err != nil {
					return err
				}
				resArgs.SetRes(res)
			case serviceinfo.StreamingServer:
				called := mhandler.Call([]reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(req),
					reflect.ValueOf(gs),
				})
				_err := called[0].Interface()
				if _err != nil {
					return _err.(error)
				}
			case serviceinfo.StreamingBidirectional:
				called := mhandler.Call([]reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(gs),
				})
				_err := called[0].Interface()
				if _err != nil {
					return _err.(error)
				}
			}
			return nil
		},
	)(ctx, reqArgs.(StreamReqArgs), resArgs.(StreamResArgs), sArgs)
}

func ClientInvoke[Header, Trailer, Req, Res any](
	ctx context.Context, cli ClientStreamProvider, smode serviceinfo.StreamingMode, method string,
	req *Req, res *Res, callOptions ...streamxcallopt.CallOption) (stream *GenericClientStream[Header, Trailer, Req, Res], err error) {
	reqArgs, resArgs, sArgs := NewStreamReqArgs(nil), NewStreamResArgs(nil), NewStreamArgs(nil)
	// important notes: please don't set a typed nil value into interface arg like NewStreamReqArgs({typ: *Res, ptr: nil})
	// otherwise, reqArgs.Req() may not ==nil forever
	if req != nil {
		reqArgs.SetReq(req)
	}
	if res != nil {
		resArgs.SetRes(res)
	}
	err = cli.StreamMiddleware(
		func(ctx context.Context, reqArgs StreamReqArgs, resArgs StreamResArgs, streamArgs StreamArgs) (err error) {
			log.Printf("Client invoke new stream start: method=%s", method)
			cs, err := cli.NewStream(ctx, method, req, callOptions...)
			log.Printf("Client invoke new stream start: method=%s", method)
			if err != nil {
				return err
			}
			stream = NewGenericClientStream[Header, Trailer, Req, Res](cs)
			AsMutableStreamArgs(streamArgs).SetStream(stream)

			switch smode {
			case serviceinfo.StreamingUnary:
				if err = stream.SendMsg(ctx, req); err != nil {
					return err
				}
				if err = stream.RecvMsg(ctx, res); err != nil {
					return err
				}
				resArgs.SetRes(res)
				return stream.CloseSend(ctx)
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
		},
	)(ctx, reqArgs, resArgs, sArgs)
	if err != nil {
		return nil, err
	}
	return stream, nil
}
