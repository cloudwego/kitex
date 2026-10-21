package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/internal/transportcheck"
	"github.com/cloudwego/kitex/pkg/consts"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	thriftcodec "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/transport"
)

type nativeServer struct {
	server    *server
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	listener  remote.NativeListener
	errCh     chan error
	serveDone chan struct{}
	jobs      sync.WaitGroup
}

func newNativeServer(s *server) *nativeServer {
	ctx, cancel := context.WithCancel(fillContext(s.opt))
	return &nativeServer{server: s, ctx: ctx, cancel: cancel, errCh: make(chan error, 1), serveDone: make(chan struct{})}
}

func (n *nativeServer) Start() chan error {
	o := n.server.opt.RemoteOpt
	listener, err := n.server.opt.NativeTransport.Listen(n.ctx, o.Address, remote.NativeListenerOptions{MaxConnectionIdleTime: o.MaxConnectionIdleTime, ReadWriteTimeout: o.ReadWriteTimeout})
	if err != nil {
		n.errCh <- err
		close(n.serveDone)
		return n.errCh
	}
	n.mu.Lock()
	n.listener = listener
	n.mu.Unlock()
	go n.serve(listener)
	return n.errCh
}
func (n *nativeServer) serve(listener remote.NativeListener) {
	defer close(n.serveDone)
	if concurrent, ok := listener.(remote.NativeConcurrentListener); ok && concurrent.ConcurrentAccept() {
		var stopped sync.Once
		for i := 0; i < 64; i++ {
			n.jobs.Add(1)
			go func() {
				defer n.jobs.Done()
				for {
					request, err := listener.Accept(n.ctx)
					if err != nil {
						if n.ctx.Err() != nil {
							err = nil
						}
						stopped.Do(func() { n.errCh <- err })
						return
					}
					n.handle(request)
				}
			}()
		}
		return
	}
	slots := make(chan struct{}, 64)
	for {
		select {
		case slots <- struct{}{}:
		case <-n.ctx.Done():
			n.errCh <- nil
			return
		}
		request, err := listener.Accept(n.ctx)
		if err != nil {
			if n.ctx.Err() != nil {
				err = nil
			}
			n.errCh <- err
			return
		}
		n.jobs.Add(1)
		go func() { defer n.jobs.Done(); defer func() { <-slots }(); n.handle(request) }()
	}
}
func (n *nativeServer) Address() net.Addr {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.listener != nil {
		return n.listener.Address()
	}
	return nil
}
func (n *nativeServer) Stop() error {
	n.cancel()
	n.mu.Lock()
	listener := n.listener
	n.mu.Unlock()
	var err error
	if listener != nil {
		err = listener.Close()
	}
	finished := make(chan struct{})
	go func() { <-n.serveDone; n.jobs.Wait(); close(finished) }()
	timeout := n.server.opt.RemoteOpt.ExitWaitTime
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	select {
	case <-finished:
		return err
	case <-time.After(timeout):
		return errors.New("native server is draining active request tasks")
	}
}

func (s *server) checkNativeOptions() error {
	if s.opt.NativeTransport == nil {
		return errors.New("native server requires a transport bridge")
	}
	if len(s.opt.NativeUnsupported) != 0 {
		return fmt.Errorf("native server does not support %v", s.opt.NativeUnsupported)
	}
	if s.opt.Proxy != nil || s.opt.RemoteOpt.Listener != nil || s.opt.Limit.Limits != nil || s.opt.Limit.ConLimit != nil || s.opt.Limit.QPSLimit != nil {
		return errors.New("native server does not support Go listener, proxy or transport limiters")
	}
	info := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil)
	message := remote.NewMessage(nil, info, remote.Call, remote.Server)
	payload, err := remote.GetPayloadCodec(message)
	remote.RecycleMessage(message)
	rpcinfo.PutRPCInfo(info)
	if err != nil || !reflect.DeepEqual(payload, thriftcodec.NewThriftCodec()) {
		return errors.New("native server requires default Fast/owned codec")
	}
	for _, svc := range s.GetServiceInfos() {
		if svc.PayloadCodec != serviceinfo.Thrift {
			return remote.ErrUnaryLifecycleProtocol
		}
		for _, method := range svc.Methods {
			if method.OneWay() || method.StreamingMode() != serviceinfo.StreamingNone {
				continue
			}
			if err := transportcheck.FastCodec(method.NewArgs()); err != nil {
				return err
			}
			if err := transportcheck.FastCodec(method.NewResult()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *nativeServer) handle(request remote.NativeServerRequest) {
	s := n.server
	facts := request.Facts()
	var ctx context.Context
	var cancel context.CancelFunc
	var stop func() bool
	scope, scoped := request.(remote.NativeServerRequestScope)
	if scoped {
		ctx, scoped = scope.RequestContext()
	}
	if !scoped {
		ctx, cancel = context.WithCancel(n.ctx)
		stop = nativeAfterCancel(request.Context(), cancel)
	}
	scopeContext := ctx
	ri := s.opt.RemoteOpt.InitOrResetRPCInfoFunc(nil, facts.Peer)
	ctx = rpcinfo.NewCtxWithRPCInfo(remote.WithServiceSearcher(ctx, s.opt.RemoteOpt.SvcSearcher), ri)
	call := remote.NewUnaryCall(s.opt.NativeTransport)
	call.NativeRequest = request
	call.Begin = facts.Begin
	if call.Begin.IsZero() {
		call.Begin = time.Now()
	}
	call.ConnID, call.Reused, call.ReadEnd = facts.IO.ConnID, facts.IO.Reused, facts.IO.ReadEnd
	call.Request = remote.NewMessage(nil, ri, remote.Call, remote.Server)
	ctx = remote.WithUnaryCall(ctx, call)
	rpcinfo.RecordAt(ctx, ri, stats.RPCStart, nil, call.Begin)
	rpcinfo.RecordAt(ctx, ri, stats.ReadStart, nil, call.Begin)
	rpcinfo.RecordAt(ctx, ri, stats.ReadFinish, nil, facts.IO.ReadEnd)
	ctx = s.opt.TracerCtl.DoStart(ctx, ri)
	ctx = metainfo.WithBackwardValuesToSend(ctx)
	base := ctx
	metadata, deferred := s.opt.NativeTransport.(remote.NativeServerDeferredMetadata)
	deferred = deferred && metadata.DeferIngressMetadata()
	rx := request.Frame()
	cleanup, paired := request.(remote.NativeServerRequestCleanup)
	paired = paired && cleanup.RetainFrameForCleanup()
	var tx remote.NativeBuffer
	var err error
	var cancels []context.CancelFunc
	var projectionStops []func()
	limit, _ := ctx.Deadline()
	defer func() {
		if p := recover(); p != nil {
			err = kerrors.ErrPanic.WithCause(fmt.Errorf("native server request panic: %v", p))
			rpcinfo.AsMutableRPCStats(ri.Stats()).SetPanicked(err)
		}
		completeCtx, releaseComplete := nativeCompleteContext(ctx)
		err = call.CompleteServer(completeCtx, err)
		if releaseComplete != nil {
			releaseComplete()
		}
		var txErr error
		if tx != nil {
			txErr = tx.Release()
		}
		reset, _ := ri.To().Tag(rpcinfo.ConnResetTag)
		reuse := err == nil && txErr == nil && ctx.Err() == nil && reset != "1"
		if stop != nil {
			stop()
		}
		for _, f := range projectionStops {
			f()
		}
		if paired && rx != nil {
			bufferErr, requestErr := cleanup.ReleaseWithBuffer(reuse, rx)
			err = errors.Join(err, bufferErr, txErr, requestErr)
		} else {
			if rx != nil {
				err = errors.Join(err, rx.Release())
			}
			err = errors.Join(err, txErr)
			err = errors.Join(err, request.Release(reuse && err == nil))
		}
		traceErr := err
		if traceErr == nil {
			traceErr = call.Err
		}
		s.opt.TracerCtl.DoFinish(ctx, ri, traceErr)
		for _, f := range cancels {
			f()
		}
		if cancel != nil {
			cancel()
		}
		remote.RecycleMessage(call.Request)
		remote.RecycleMessage(call.Response)
		call.Recycle()
		rpcinfo.PutRPCInfo(ri)
	}()
	budget := func() error {
		if d, ok := ctx.Deadline(); ok && (limit.IsZero() || d.Before(limit)) {
			limit = d
		}
		if timeout := ri.Config().RPCTimeout(); timeout > 0 {
			deadline := call.Begin.Add(timeout)
			if limit.IsZero() || deadline.Before(limit) {
				limit = deadline
			}
		}
		if !limit.IsZero() {
			if scoped {
				deadlineContext := ctx
				derived := remote.UnaryCallFromContext(ctx) == call
				if !derived {
					deadlineContext = scopeContext
				}
				nextCtx, deadlineErr := scope.WithRequestDeadline(deadlineContext, limit)
				if deadlineErr != nil {
					return kerrors.ErrRPCTimeout.WithCause(deadlineErr)
				}
				if nextCtx == nil {
					return errors.New("native request scope returned a nil deadline context")
				}
				if derived {
					ctx = nextCtx
				}
			}
			if current, ok := ctx.Deadline(); !ok || limit.Before(current) {
				var c context.CancelFunc
				ctx, c = context.WithDeadline(ctx, limit)
				cancels = append(cancels, c)
			}
		}
		if ctx.Err() != nil {
			return kerrors.ErrRPCTimeout.WithCause(ctx.Err())
		}
		return nil
	}
	call.FailureStage = remote.UnaryFailureReceive
	payloadCodec, codecErr := remote.GetPayloadCodec(call.Request)
	if codecErr != nil {
		err = codecErr
		return
	}
	if !reflect.DeepEqual(payloadCodec, thriftcodec.NewThriftCodec()) {
		err = errors.New("native server requires default Fast/owned codec")
		return
	}
	if err = budget(); err != nil {
		return
	}
	err = s.opt.RemoteOpt.Codec.Decode(ctx, call.Request, remote.NewNativeReader(rx))
	rpcinfo.AsMutableRPCStats(ri.Stats()).SetRecvSize(facts.IO.ReceivedBytes)
	if err == nil && ri.Config().TransportProtocol() != transport.TTHeaderFramed {
		err = remote.ErrUnaryLifecycleProtocol
	}
	if err == nil {
		err = remote.CheckUnaryLifecycle(ri)
	}
	if err == nil {
		call.Response = remote.NewMessage(ri.Invocation().MethodInfo().NewResult(), ri, remote.Reply, remote.Server)
		if deferred {
			err = metadata.PrepareIngress(call)
		} else {
			for _, h := range s.opt.MetaHandlers {
				ctx, err = h.ReadMeta(ctx, call.Request)
				if err != nil {
					break
				}
			}
		}
	}
	if err == nil {
		err = budget()
	}
	if err == nil {
		call.Started = true
		call.FailureStage = remote.UnaryFailurePrepare
		err = call.Lifecycle.ServerBegin(ctx, call)
		if budgetErr := budget(); err == nil {
			err = budgetErr
		}
		cfg := rpcinfo.AsMutableRPCConfig(ri.Config())
		protocol, payload := ri.Config().TransportProtocol(), ri.Config().PayloadCodec()
		cfg.CopyFrom(s.opt.Configs)
		cfg.SetTransportProtocol(protocol)
		cfg.SetPayloadCodec(payload)
		var projectionErr error
		decoded := ctx
		if deferred {
			ctx, projectionErr = metadata.ReadIngressMetadata(ctx, call, s.opt.MetaHandlers)
		} else {
			ctx, projectionErr = s.opt.NativeTransport.ProjectIngress(base, ctx, call)
		}
		if scoped && decoded.Done() != nil && decoded.Done() != ctx.Done() {
			var stopProjection func()
			var cancelProjection context.CancelFunc
			ctx, stopProjection, cancelProjection = nativeProjectCancellation(ctx, decoded)
			projectionStops = append(projectionStops, stopProjection)
			cancels = append(cancels, cancelProjection)
		}
		if err == nil {
			err = projectionErr
		}
	}
	if err == nil {
		err = budget()
	}
	if err == nil {
		if !paired {
			err = rx.Release()
			rx = nil
		}
	}
	if err == nil {
		ctx = context.WithValue(ctx, consts.CtxKeyMethod, ri.To().Method())
		ctx = metainfo.TransferForward(ctx)
		call.FailureStage = remote.UnaryFailureBusiness
		call.HandlerStart = time.Now()
		err = s.eps(ctx, call.Request.Data(), call.Response.Data())
		call.HandlerEnd = time.Now()
	}
	call.Err = err
	if ctx.Err() != nil {
		err = ctx.Err()
		return
	}
	if err != nil {
		if ri.Invocation().MethodName() == "" {
			return
		}
		if call.Response != nil {
			remote.RecycleMessage(call.Response)
		}
		call.Response = remote.NewMessage(remote.NewTransError(remote.InternalError, err), ri, remote.Exception, remote.Server)
	} else {
		call.FailureStage = remote.UnaryFailureNone
	}
	responseFailure := call.FailureStage
	call.FailureStage = remote.UnaryFailureSend
	remote.PreparePrimaryHeaders(ctx, call.Response, true)
	for _, h := range s.opt.MetaHandlers {
		ctx, err = h.WriteMeta(ctx, call.Response)
		if err != nil {
			return
		}
	}
	preparer, ok := s.opt.RemoteOpt.Codec.(interface {
		PrepareUnaryWire(context.Context, remote.Message, *remote.UnaryCall) error
	})
	if !ok {
		err = remote.ErrUnaryLifecycleProtocol
		return
	}
	if err = preparer.PrepareUnaryWire(ctx, call.Response, call); err != nil {
		return
	}
	if call.Started {
		call.FailureStage = responseFailure
		if err = call.Lifecycle.ServerResponse(ctx, call); err != nil {
			call.FailureStage = remote.UnaryFailurePrepare
			return
		}
	}
	call.FailureStage = remote.UnaryFailureSend
	if err = budget(); err != nil {
		return
	}
	tx, err = remote.AcquireNativeTx(ctx, call.Response, request)
	if err != nil {
		return
	}
	writer := remote.NewNativeWriter(tx)
	if err = s.opt.RemoteOpt.Codec.Encode(ctx, call.Response, writer); err != nil {
		return
	}
	if err = budget(); err != nil {
		return
	}
	owned := tx
	tx = nil
	ioFacts, sendErr := request.Send(ctx, owned, writer.WrittenLen())
	call.WriteStart, call.WriteEnd = ioFacts.WriteStart, ioFacts.WriteEnd
	rpcinfo.RecordAt(ctx, ri, stats.WriteStart, nil, ioFacts.WriteStart)
	rpcinfo.RecordAt(ctx, ri, stats.WriteFinish, sendErr, ioFacts.WriteEnd)
	rpcinfo.AsMutableRPCStats(ri.Stats()).SetSendSize(ioFacts.SentBytes)
	err = sendErr
	if err != nil {
		call.FailureStage = remote.UnaryFailureSend
		if ioFacts.FailureStage != remote.UnaryFailureNone {
			call.FailureStage = ioFacts.FailureStage
		}
	} else {
		call.FailureStage = responseFailure
	}
}
