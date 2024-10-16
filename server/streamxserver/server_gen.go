package streamxserver

import (
	"context"
	"errors"
	"reflect"
	"sync"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
)

var invokerCache sync.Map

func InvokeStream[Req, Res any](
	ctx context.Context, smode serviceinfo.StreamingMode,
	handler any, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs,
) (err error) {
	// prepare args
	sArgs := streamx.GetStreamArgsFromContext(ctx)
	if sArgs == nil {
		return errors.New("server stream is nil")
	}
	shandler := handler.(streamx.StreamHandler)
	gs := streamx.NewGenericServerStream[Req, Res](sArgs.Stream())
	gs.SetStreamRecvMiddleware(shandler.StreamRecvMiddleware)
	gs.SetStreamSendMiddleware(shandler.StreamSendMiddleware)

	// before handler
	var req *Req
	var res *Res
	switch smode {
	case serviceinfo.StreamingUnary, serviceinfo.StreamingServer:
		req, err = gs.Recv(ctx)
		if err != nil {
			return err
		}
		reqArgs.SetReq(req)
	default:
	}

	// handler call
	cacheKey := reflect.TypeOf(shandler.Handler).String() + sArgs.Stream().Method()
	var mhandler reflect.Value
	if v, ok := invokerCache.Load(cacheKey); ok {
		mhandler = v.(reflect.Value)
	} else {
		rhandler := reflect.ValueOf(shandler.Handler)
		mhandler = rhandler.MethodByName(sArgs.Stream().Method())
		invokerCache.Store(cacheKey, mhandler)
	}

	streamInvoke := func(ctx context.Context, streamArgs streamx.StreamArgs, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs) (err error) {
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
			if err = gs.Send(ctx, res); err != nil {
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
	}
	if shandler.StreamMiddleware != nil {
		err = shandler.StreamMiddleware(streamInvoke)(ctx, sArgs, reqArgs, resArgs)
	} else {
		err = streamInvoke(ctx, sArgs, reqArgs, resArgs)
	}
	return err
}
