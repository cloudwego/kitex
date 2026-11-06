package client

import (
	"context"
	"errors"
	"testing"
	"time"

	internalclient "github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

type callFactory struct {
	remote.ClientTransHandlerFactory
	kinds       []remote.ClientCallType
	unsupported bool
	failure     error
}

func (f *callFactory) NewClientCall(_ context.Context, cfg remote.ClientCallConfig, next endpoint.Endpoint) (endpoint.Endpoint, error) {
	f.kinds = append(f.kinds, cfg.Type)
	if f.unsupported && cfg.Type == remote.ClientCallStreaming {
		return nil, remote.ErrUnsupportedClientCall
	}
	if f.failure != nil {
		return nil, f.failure
	}
	return next, nil
}

func TestCallFactorySupportsBothCallTypes(t *testing.T) {
	o := internalclient.NewOptions(nil)
	t.Cleanup(func() { _ = o.RemoteOpt.ConnPool.Close() })
	factory := &callFactory{}
	o.RemoteOpt.CliHandlerFactory = factory
	kc := &kClient{opt: o}
	for _, kind := range []remote.ClientCallType{remote.ClientCallUnary, remote.ClientCallStreaming} {
		invoked := false
		ep, err := kc.buildClientCall(context.Background(), kind, nil, func(context.Context, interface{}, interface{}) error { invoked = true; return nil })
		if err != nil {
			t.Fatal(err)
		}
		if err = ep(context.Background(), nil, nil); err != nil || !invoked {
			t.Fatalf("kind=%v invoked=%v err=%v", kind, invoked, err)
		}
	}
	if len(factory.kinds) != 2 || factory.kinds[0] != remote.ClientCallUnary || factory.kinds[1] != remote.ClientCallStreaming {
		t.Fatal(factory.kinds)
	}
}

func TestUnsupportedCallTypeDoesNotRejectOtherCalls(t *testing.T) {
	o := internalclient.NewOptions(nil)
	t.Cleanup(func() { _ = o.RemoteOpt.ConnPool.Close() })
	o.RemoteOpt.CliHandlerFactory = &callFactory{unsupported: true}
	kc := &kClient{opt: o}
	for _, kind := range []remote.ClientCallType{remote.ClientCallUnary, remote.ClientCallStreaming} {
		invoked := false
		ep, err := kc.buildClientCall(context.Background(), kind, nil, func(context.Context, interface{}, interface{}) error { invoked = true; return nil })
		if err != nil {
			t.Fatal(err)
		}
		err = ep(context.Background(), nil, nil)
		if kind == remote.ClientCallStreaming {
			if !errors.Is(err, remote.ErrUnsupportedClientCall) || invoked {
				t.Fatalf("%v %v", err, invoked)
			}
		} else if err != nil || !invoked {
			t.Fatalf("%v %v", err, invoked)
		}
	}
}

func TestDefaultCallExecutionKeepsTimeoutAndStreamingIndependent(t *testing.T) {
	o := internalclient.NewOptions(nil)
	t.Cleanup(func() { _ = o.RemoteOpt.ConnPool.Close() })
	_ = rpcinfo.AsMutableRPCConfig(o.Configs).SetRPCTimeout(time.Second)
	kc := &kClient{opt: o}
	for _, kind := range []remote.ClientCallType{remote.ClientCallUnary, remote.ClientCallStreaming} {
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", "Echo"), o.Configs, rpcinfo.NewRPCStats()))
		deadline := false
		ep, err := kc.buildClientCall(context.Background(), kind, nil, func(ctx context.Context, _, _ interface{}) error { _, deadline = ctx.Deadline(); return nil })
		if err != nil {
			t.Fatal(err)
		}
		if err = ep(ctx, nil, nil); err != nil {
			t.Fatal(err)
		}
		if deadline != (kind == remote.ClientCallUnary) {
			t.Fatalf("kind=%v deadline=%v", kind, deadline)
		}
	}
}
