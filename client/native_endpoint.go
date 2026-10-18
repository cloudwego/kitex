package client

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/kitex/internal/transportcheck"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	thriftcodec "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

type nativeBeginKey struct{}

func (kc *kClient) nativeEndpoint() (endpoint.Endpoint, error) {
	preparer, ok := kc.opt.RemoteOpt.Codec.(interface {
		PrepareUnaryWire(context.Context, remote.Message, *remote.UnaryCall) error
	})
	if !ok {
		return nil, remote.ErrUnaryLifecycleProtocol
	}
	return func(ctx context.Context, request, response interface{}) (err error) {
		ri := rpcinfo.GetRPCInfo(ctx)
		if err = remote.CheckUnaryLifecycle(ri); err != nil {
			return err
		}
		call := remote.NewUnaryCall(kc.opt.NativeTransport)
		if begin, ok := ctx.Value(nativeBeginKey{}).(time.Time); ok {
			call.Begin = begin
		}
		call.Request = remote.NewMessage(request, ri, remote.Call, remote.Client)
		call.Response = remote.NewMessage(response, ri, remote.Reply, remote.Client)
		ctx = remote.WithUnaryCall(ctx, call)
		var connection remote.NativeClientConnection
		var tx, rx remote.NativeBuffer
		var cancels []context.CancelFunc
		defer func() {
			if p := recover(); p != nil {
				err = rpcinfo.ClientPanicToErr(ctx, p, ri, false)
			}
			err = call.FinishClient(ctx, err)
			reset, _ := ri.To().Tag(rpcinfo.ConnResetTag)
			reuse := err == nil && ctx.Err() == nil && reset != "1"
			if cleanup, ok := connection.(remote.NativeClientCleanup); ok && tx == nil && rx != nil {
				bufferErr, connectionErr := cleanup.ReleaseWithBuffer(reuse, rx)
				if err == nil {
					err = bufferErr
				}
				if err == nil {
					err = connectionErr
				}
			} else {
				if rx != nil {
					if e := rx.Release(); err == nil {
						err = e
					}
				}
				if tx != nil {
					if e := tx.Release(); err == nil {
						err = e
					}
				}
				if connection != nil {
					if e := connection.Release(err == nil && ctx.Err() == nil && reset != "1"); err == nil {
						err = e
					}
				}
			}
			for _, cancel := range cancels {
				cancel()
			}
			remote.RecycleMessage(call.Request)
			remote.RecycleMessage(call.Response)
			call.Recycle()
		}()
		budget := func() error {
			if timeout := ri.Config().RPCTimeout(); timeout > 0 {
				deadline := call.Begin.Add(timeout)
				if current, ok := ctx.Deadline(); !ok || deadline.Before(current) {
					var cancel context.CancelFunc
					ctx, cancel = context.WithDeadline(ctx, deadline)
					cancels = append(cancels, cancel)
				}
			}
			if e := ctx.Err(); e != nil {
				return kerrors.ErrRPCTimeout.WithCause(e)
			}
			return nil
		}
		call.FailureStage = remote.UnaryFailurePrepare
		if err = transportcheck.FastCodec(request); err != nil {
			return err
		}
		if err = transportcheck.FastCodec(response); err != nil {
			return err
		}
		payloadCodec, codecErr := remote.GetPayloadCodec(call.Request)
		if codecErr != nil {
			return codecErr
		}
		if !reflect.DeepEqual(payloadCodec, thriftcodec.NewThriftCodec()) {
			return errors.New("native transport requires the default Fast/owned codec")
		}
		if err = budget(); err != nil {
			return err
		}
		remote.PreparePrimaryHeaders(ctx, call.Request, false)
		for _, h := range kc.opt.MetaHandlers {
			ctx, err = h.WriteMeta(ctx, call.Request)
			if err != nil {
				return err
			}
		}
		call.MetaPrepared = true
		if err = preparer.PrepareUnaryWire(ctx, call.Request, call); err != nil {
			return err
		}
		call.Started = true
		if err = call.Lifecycle.ClientPrepare(ctx, call); err != nil {
			return err
		}
		if err = budget(); err != nil {
			return err
		}
		call.FailureStage = remote.UnaryFailureConnect
		var facts remote.NativeIO
		connection, facts, err = kc.opt.NativeTransport.Connect(ctx, call)
		applyNativeIO(call, facts)
		if err != nil {
			return err
		}
		if h, ok := kc.opt.NativeTransport.(remote.OutboundHandler); ok {
			ctx, err = h.Write(ctx, nil, call.Request)
			if err != nil {
				return err
			}
		}
		call.FailureStage = remote.UnaryFailureSend
		if err = budget(); err != nil {
			return err
		}
		tx, err = remote.AcquireNativeTx(ctx, call.Request, connection)
		if err != nil {
			return err
		}
		writer := remote.NewNativeWriter(tx)
		if err = kc.opt.RemoteOpt.Codec.Encode(ctx, call.Request, writer); err != nil {
			return err
		}
		if err = budget(); err != nil {
			return err
		}
		length := writer.WrittenLen()
		owned := tx
		tx = nil
		rx, facts, err = connection.Exchange(ctx, owned, length)
		applyNativeIO(call, facts)
		if err != nil {
			if facts.FailureStage != remote.UnaryFailureNone {
				call.FailureStage = facts.FailureStage
			}
			return err
		}
		call.FailureStage = remote.UnaryFailureReceive
		if err = budget(); err != nil {
			return err
		}
		err = kc.opt.RemoteOpt.Codec.Decode(ctx, call.Response, remote.NewNativeReader(rx))
		applyNativeIO(call, facts)
		if err == nil {
			err = budget()
		}
		if err != nil {
			var exception *thrift.ApplicationException
			if call.Response.MessageType() == remote.Exception && errors.As(err, &exception) {
				call.FailureStage = remote.UnaryFailureBusiness
			}
			return err
		}
		call.FailureStage = remote.UnaryFailureNone
		if err = call.FinishClient(ctx, nil); err != nil {
			return err
		}
		if err = budget(); err != nil {
			return err
		}
		for _, h := range kc.opt.MetaHandlers {
			ctx, err = h.ReadMeta(ctx, call.Response)
			if err != nil {
				return err
			}
		}
		return budget()
	}, nil
}

func applyNativeIO(call *remote.UnaryCall, facts remote.NativeIO) {
	if facts.ConnID != 0 {
		call.ConnID = facts.ConnID
	}
	if !facts.ConnectStart.IsZero() {
		call.ConnectStart, call.ConnectEnd, call.Reused = facts.ConnectStart, facts.ConnectEnd, facts.Reused
	}
	if !facts.WriteStart.IsZero() {
		call.WriteStart, call.WriteEnd = facts.WriteStart, facts.WriteEnd
	}
	if !facts.ReadEnd.IsZero() {
		call.ReadEnd = facts.ReadEnd
	}
	if stats := rpcinfo.AsMutableRPCStats(call.Request.RPCInfo().Stats()); stats != nil {
		stats.SetSendSize(facts.SentBytes)
		stats.SetRecvSize(facts.ReceivedBytes)
	}
}
