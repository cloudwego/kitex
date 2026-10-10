package client_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
)

func TestUnaryLockedPrepareUsesTimeoutWorker(t *testing.T) {
	addr, _ := startUnaryServer(t, nil)
	var done <-chan struct{}
	var finishes atomic.Int32
	cli := unaryClient(t, addr, &unaryHooks{
		prepare: func(ctx context.Context, c *remote.UnaryCall) error {
			_, ok := ctx.Deadline()
			test.Assert(t, ok && ctx.Done() != nil, "prepare is outside the timeout worker")
			done = ctx.Done()
			return nil
		},
		finish: func(ctx context.Context, c *remote.UnaryCall) error {
			test.Assert(t, ctx.Done() == done && c.Err == nil)
			finishes.Add(1)
			return nil
		},
	})
	_, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "worker"})
	test.Assert(t, err == nil && finishes.Load() == 1)
}

func TestUnaryLockedPrepareTimeoutOwnsPendingState(t *testing.T) {
	addr, handler := startUnaryServer(t, nil)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	cli := unaryClient(t, addr, &unaryHooks{
		prepare: func(ctx context.Context, c *remote.UnaryCall) error {
			close(entered)
			<-release
			return nil
		},
		finish: func(ctx context.Context, c *remote.UnaryCall) error {
			test.Assert(t, c.Err != nil && c.FailureStage == remote.UnaryFailurePrepare && c.Conn == nil)
			close(finished)
			return nil
		},
	}, client.WithRPCTimeout(20*time.Millisecond))
	result := make(chan error, 1)
	go func() {
		_, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "blocked-prepare"})
		result <- err
	}()
	defer close(release)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("prepare did not start")
	}
	select {
	case err := <-result:
		test.Assert(t, err != nil && handler.calls.Load() == 0)
	case <-time.After(time.Second):
		t.Fatal("locked RPC timeout did not cover preparation")
	}
	select {
	case <-finished:
		t.Fatal("pending preparation was released")
	default:
	}
	t.Cleanup(func() {
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("pending preparation was not finalized")
		}
	})
}
