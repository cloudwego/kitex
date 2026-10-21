package server_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
)

type nativeFinalMetaFixture struct {
	*nativeServerFixture
	prepared bool
}

func (*nativeFinalMetaFixture) DeferIngressMetadata() bool { return true }
func (f *nativeFinalMetaFixture) PrepareIngress(c *remote.UnaryCall) error {
	f.prepared = true
	return rpcinfo.AsMutableRPCConfig(c.Request.RPCInfo().Config()).SetRPCTimeout(100 * time.Millisecond)
}
func (f *nativeFinalMetaFixture) ServerBegin(ctx context.Context, c *remote.UnaryCall) error {
	if !f.prepared || f.reads.Load() != 0 || ctx.Value(nativeMetaKey{}) != nil {
		return errors.New("metadata was projected before governance")
	}
	deadline, ok := ctx.Deadline()
	if !ok || !deadline.Equal(c.Begin.Add(100*time.Millisecond)) {
		return errors.New("original absolute budget missing before governance")
	}
	f.begins.Add(1)
	if f.admissionTimeout > 0 {
		rpcinfo.AsMutableRPCConfig(c.Request.RPCInfo().Config()).SetRPCTimeout(f.admissionTimeout)
	}
	return f.beginErr
}
func (f *nativeFinalMetaFixture) ReadIngressMetadata(ctx context.Context, c *remote.UnaryCall, handlers []remote.MetaHandler) (context.Context, error) {
	for _, h := range handlers {
		var err error
		ctx, err = h.ReadMeta(ctx, c.Request)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}
func (*nativeFinalMetaFixture) ProjectIngress(_ context.Context, ctx context.Context, _ *remote.UnaryCall) (context.Context, error) {
	return ctx, errors.New("duplicate projection")
}

func TestNativeFinalMetadataOrder(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "metadata-error"}[fail], func(t *testing.T) {
			base := &nativeServerFixture{completed: make(chan nativeFixtureCompletion, 2), admissionTimeout: time.Second}
			if fail {
				base.readErr = errors.New("final metadata failed")
			}
			bridge := &nativeFinalMetaFixture{nativeServerFixture: base}
			handler := new(nativeFixtureEcho)
			cli, _ := startNativeFixture(t, bridge, handler, server.WithEnableContextTimeout(true), server.WithMiddleware(func(next endpoint.Endpoint) endpoint.Endpoint {
				return func(ctx context.Context, req, resp interface{}) error {
					call := remote.UnaryCallFromContext(ctx)
					d, ok := ctx.Deadline()
					if !ok || !d.Equal(call.Begin.Add(100*time.Millisecond)) || ctx.Value(nativeMetaKey{}) != "read" {
						return errors.New("final metadata or original budget lost")
					}
					return next(ctx, req, resp)
				}
			}))
			_, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "final-meta"})
			if fail && (err == nil || !strings.Contains(err.Error(), "final metadata failed")) {
				t.Fatalf("wrong metadata error: %v", err)
			}
			if !fail && err != nil {
				t.Fatal(err)
			}
			select {
			case <-base.completed:
			case <-time.After(time.Second):
				t.Fatal("missing completion")
			}
			want := int32(1)
			if fail {
				want = 0
			}
			if base.reads.Load() != 1 || base.begins.Load() != 1 || base.completes.Load() != 1 || handler.calls.Load() != want {
				t.Fatalf("wrong counts: read=%d begin=%d complete=%d handler=%d", base.reads.Load(), base.begins.Load(), base.completes.Load(), handler.calls.Load())
			}
		})
	}
}
