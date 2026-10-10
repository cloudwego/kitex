package client

import (
	"context"
	"time"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/remotecli"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func (kc *kClient) unaryLifecycle() remote.UnaryLifecycle {
	if kc.opt.RemoteOpt == nil {
		return nil
	}
	return kc.opt.RemoteOpt.UnaryLifecycle
}

func (kc *kClient) unaryLifecycleEndpoint(pipe remote.ClientTransHandler) (endpoint.Endpoint, error) {
	preparer, ok := kc.opt.RemoteOpt.Codec.(interface {
		PrepareUnaryWire(context.Context, remote.Message, *remote.UnaryCall) error
	})
	if !ok {
		return nil, remote.ErrUnaryLifecycleProtocol
	}
	prepare := func(ctx context.Context, call *remote.UnaryCall) (context.Context, error) {
		var err error
		remote.PreparePrimaryHeaders(ctx, call.Request, false)
		for _, h := range kc.opt.MetaHandlers {
			ctx, err = h.WriteMeta(ctx, call.Request)
			if err != nil {
				return ctx, err
			}
		}
		call.MetaPrepared = true
		if err = preparer.PrepareUnaryWire(ctx, call.Request, call); err != nil {
			return ctx, err
		}
		call.Started = true
		if err = call.Lifecycle.ClientPrepare(ctx, call); err != nil {
			call.FailureStage = remote.UnaryFailurePrepare
			return ctx, err
		}
		return ctx, nil
	}
	run := func(ctx context.Context, request, response any) (err error) {
		call := request.(*remote.UnaryCall)
		ri := call.Request.RPCInfo()
		var cli remotecli.Client
		stage := remote.UnaryFailureConnect
		defer func() {
			if p := recover(); p != nil {
				err = rpcinfo.ClientPanicToErr(ctx, p, ri, false)
				call.FailureStage = stage
			}
			err = call.FinishClient(ctx, err)
			if cli != nil {
				cli.Recycle()
			}
			remote.RecycleMessage(call.Request)
			remote.RecycleMessage(call.Response)
			call.Recycle()
		}()
		if ctx.Err() != nil {
			call.FailureStage = remote.UnaryFailurePrepare
			return ctx.Err()
		}
		if !call.MetaPrepared {
			ctx, err = prepare(ctx, call)
			if err != nil {
				return err
			}
			if err = ctx.Err(); err != nil {
				call.FailureStage = remote.UnaryFailurePrepare
				return err
			}
			if timeout := ri.Config().RPCTimeout(); timeout > 0 && time.Since(call.Begin) >= timeout {
				call.FailureStage = remote.UnaryFailurePrepare
				return kerrors.ErrRPCTimeout.WithCause(context.DeadlineExceeded)
			}
		}
		cli, err = remotecli.NewClient(ctx, ri, pipe, kc.opt.RemoteOpt)
		if err != nil {
			return err
		}
		stage = remote.UnaryFailureSend
		if err = cli.Send(ctx, ri, call.Request); err != nil {
			return err
		}
		stage = remote.UnaryFailureReceive
		return cli.Recv(ctx, ri, call.Response)
	}
	return func(ctx context.Context, request, response any) (err error) {
		ri := rpcinfo.GetRPCInfo(ctx)
		if err = remote.CheckUnaryLifecycle(ri); err != nil {
			return err
		}
		call := remote.NewUnaryCall(kc.opt.RemoteOpt.UnaryLifecycle)
		ctx = remote.WithUnaryCall(ctx, call)
		call.Request = remote.NewMessage(request, ri, remote.Call, remote.Client)
		call.Response = remote.NewMessage(response, ri, remote.Reply, remote.Client)
		call.Request.SetPayloadCodec(kc.opt.RemoteOpt.PayloadCodec)
		handedOff := false
		defer func() {
			if !handedOff {
				if p := recover(); p != nil {
					err = rpcinfo.ClientPanicToErr(ctx, p, ri, false)
					call.FailureStage = remote.UnaryFailurePrepare
				}
				err = call.FinishClient(ctx, err)
				remote.RecycleMessage(call.Request)
				remote.RecycleMessage(call.Response)
				call.Recycle()
			}
		}()
		if cfg := rpcinfo.AsMutableRPCConfig(ri.Config()); cfg == nil || !cfg.IsRPCTimeoutLocked() {
			ctx, err = prepare(ctx, call)
			if err != nil {
				return err
			}
		}
		timeout := ri.Config().RPCTimeout()
		if timeout > 0 {
			timeout -= time.Since(call.Begin)
			if timeout <= 0 {
				call.FailureStage = remote.UnaryFailurePrepare
				return kerrors.ErrRPCTimeout.WithCause(context.DeadlineExceeded)
			}
		}
		handedOff = true
		if ctx.Done() == nil && timeout <= 0 {
			return run(ctx, call, nil)
		}
		workerCtx, err := workerPool.RunTask(ctx, timeout, call, nil, run)
		if err != nil && err == workerCtx.Err() {
			return kerrors.ErrRPCTimeout.WithCause(err)
		}
		return err
	}, nil
}
