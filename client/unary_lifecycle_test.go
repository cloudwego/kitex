package client_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/gopkg/protocol/thrift"

	"github.com/cloudwego/kitex/client"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/mocks/thrift/testservice"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/remote"
	thriftcodec "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/transport"
)

type unaryHooks struct {
	prepare, finish, begin, response, complete func(context.Context, *remote.UnaryCall) error
}

func (h *unaryHooks) ClientPrepare(ctx context.Context, c *remote.UnaryCall) error {
	if h.prepare != nil {
		return h.prepare(ctx, c)
	}
	return nil
}
func (h *unaryHooks) ClientFinish(ctx context.Context, c *remote.UnaryCall) error {
	if h.finish != nil {
		return h.finish(ctx, c)
	}
	return nil
}
func (h *unaryHooks) ServerBegin(ctx context.Context, c *remote.UnaryCall) error {
	if h.begin != nil {
		return h.begin(ctx, c)
	}
	return nil
}
func (h *unaryHooks) ServerResponse(ctx context.Context, c *remote.UnaryCall) error {
	if h.response != nil {
		return h.response(ctx, c)
	}
	return nil
}
func (h *unaryHooks) ServerComplete(ctx context.Context, c *remote.UnaryCall) error {
	if h.complete != nil {
		return h.complete(ctx, c)
	}
	return nil
}

type unaryEcho struct {
	kt.TestService
	calls atomic.Int32
}

func (h *unaryEcho) EchoPingPong(ctx context.Context, r *kt.Request) (*kt.Response, error) {
	h.calls.Add(1)
	return &kt.Response{Message: "echo:" + r.Message}, nil
}

func startUnaryServer(t testing.TB, h *unaryHooks, extra ...server.Option) (string, *unaryEcho) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handler := new(unaryEcho)
	ready, exit := make(chan struct{}), make(chan error)
	opts := []server.Option{server.WithListener(ln), server.WithExitWaitTime(time.Millisecond), server.WithExitSignal(func() <-chan error {
		close(ready)
		return exit
	})}
	if h != nil {
		opts = append(opts, server.WithUnaryLifecycle(h))
	}
	opts = append(opts, extra...)
	svr := testservice.NewServer(handler, opts...)
	done := make(chan error, 1)
	go func() { done <- svr.Run() }()
	t.Cleanup(func() {
		close(exit)
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	})
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("server did not start")
	}
	return ln.Addr().String(), handler
}

func unaryClient(t testing.TB, addr string, h *unaryHooks, opts ...client.Option) testservice.Client {
	t.Helper()
	prepare := h.prepare
	h.prepare = func(ctx context.Context, c *remote.UnaryCall) error {
		remoteinfo.AsRemoteInfo(c.Request.RPCInfo().To()).SetInstance(discovery.NewInstance("tcp", addr, 1, nil))
		if prepare != nil {
			return prepare(ctx, c)
		}
		return nil
	}
	base := []client.Option{client.WithUnaryLifecycle(h), client.WithTransportProtocol(transport.TTHeaderFramed), client.WithRPCTimeout(time.Second)}
	cli, err := testservice.NewClient("meshlib.test", append(base, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return cli
}

type countingPayload struct {
	remote.PayloadCodec
	writes             atomic.Int32
	err                error
	malformedException bool
}

func (p *countingPayload) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	p.writes.Add(1)
	if p.err != nil {
		return p.err
	}
	if p.malformedException && msg.MessageType() == remote.Exception {
		buf := remote.NewWriterBuffer(256)
		defer buf.Release(nil)
		if err := p.PayloadCodec.Marshal(ctx, msg, buf); err != nil {
			return err
		}
		wire, err := buf.Bytes()
		if err != nil {
			return err
		}
		_, _, _, n, err := thrift.Binary.ReadMessageBegin(wire)
		if err != nil || n >= len(wire) {
			return errors.New("missing exception body")
		}
		wire[n] = 127
		_, err = out.WriteBinary(wire)
		return err
	}
	return p.PayloadCodec.Marshal(ctx, msg, out)
}

type unaryMeta struct {
	writes   atomic.Int32
	writeErr error
	read     func(remote.Message) error
}

func (m *unaryMeta) WriteMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	m.writes.Add(1)
	msg.TransInfo().TransStrInfo()["phase"] = "metadata"
	return ctx, m.writeErr
}
func (m *unaryMeta) ReadMeta(ctx context.Context, msg remote.Message) (context.Context, error) {
	if m.read != nil {
		return ctx, m.read(msg)
	}
	return ctx, nil
}

func TestUnaryLifecycleNetwork(t *testing.T) {
	payload := strings.Repeat("x", 16<<10)
	var prepare, finish, begin, response, complete atomic.Int32
	cp := &countingPayload{PayloadCodec: thriftcodec.NewThriftCodec()}
	sp := &countingPayload{PayloadCodec: thriftcodec.NewThriftCodec()}
	cm, sm := new(unaryMeta), new(unaryMeta)
	sm.read = func(m remote.Message) error {
		if m.TransInfo().TransStrInfo()["phase"] != "server-begin" {
			return errors.New("begin delta was not applied before ReadMeta")
		}
		return nil
	}
	cm.read = func(m remote.Message) error {
		if m.TransInfo().TransStrInfo()["phase"] != "client-finish" {
			return errors.New("finish delta was not applied before ReadMeta")
		}
		return nil
	}
	sh := &unaryHooks{
		begin: func(ctx context.Context, c *remote.UnaryCall) error {
			begin.Add(1)
			if c.RequestWire.MessageType != uint32(remote.Call) || !c.RequestWire.Framed || c.Request.PayloadLen() <= len(payload) {
				return errors.New("request wire information unavailable")
			}
			if c.Request.TransInfo().TransStrInfo()["phase"] != "client-prepare" {
				return errors.New("request delta overwritten")
			}
			c.Request.TransInfo().TransStrInfo()["phase"] = "server-begin"
			return nil
		},
		response: func(ctx context.Context, c *remote.UnaryCall) error {
			n := response.Add(1)
			if sp.writes.Load() != n-1 || c.Response.PayloadLen() != 0 || c.ResponseWire.MessageType != uint32(remote.Reply) {
				return errors.New("response encoded before header processing")
			}
			c.Response.TransInfo().TransStrInfo()["phase"] = "server-response"
			return nil
		},
		complete: func(ctx context.Context, c *remote.UnaryCall) error {
			complete.Add(1)
			if c.HandlerStart.Before(c.ReadEnd) || c.HandlerEnd.Before(c.HandlerStart) || c.WriteStart.Before(c.HandlerEnd) {
				return errors.New("invalid handler timing")
			}
			if c.WriteStart.IsZero() || c.WriteEnd.Before(c.WriteStart) {
				return errors.New("complete before flush")
			}
			if c.Response.PayloadLen() <= len(payload) || c.Request.RPCInfo().Stats().SendSize() <= uint64(c.Response.PayloadLen()) {
				return errors.New("response length missing after flush")
			}
			return c.Err
		},
	}
	addr, handler := startUnaryServer(t, sh, server.WithMetaHandler(sm), server.WithPayloadCodec(sp))
	ch := &unaryHooks{
		prepare: func(ctx context.Context, c *remote.UnaryCall) error {
			n := prepare.Add(1)
			if cp.writes.Load() != n-1 || c.Request.PayloadLen() != 0 || c.RequestWire.MessageType != uint32(remote.Call) {
				return errors.New("request encoded before header processing")
			}
			c.Request.TransInfo().TransStrInfo()["phase"] = "client-prepare"
			return nil
		},
		finish: func(ctx context.Context, c *remote.UnaryCall) error {
			n := finish.Add(1)
			if c.Reused != (n > 1) {
				return errors.New("incorrect connection reuse flag")
			}
			if c.Err != nil {
				return c.Err
			}
			if c.ResponseWire.MessageType != uint32(remote.Reply) || c.Response.PayloadLen() <= len(payload) || c.Request.RPCInfo().Stats().SendSize() <= uint64(c.Request.PayloadLen()) {
				return errors.New("response wire information unavailable at finish")
			}
			if c.Response.TransInfo().TransStrInfo()["phase"] != "server-response" {
				return errors.New("response delta overwritten")
			}
			c.Response.TransInfo().TransStrInfo()["phase"] = "client-finish"
			return nil
		},
	}
	cli := unaryClient(t, addr, ch, client.WithMetaHandler(cm), client.WithPayloadCodec(cp))
	for i := 0; i < 2; i++ {
		r, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: payload})
		if err != nil {
			t.Fatal(err)
		}
		if r.Message != "echo:"+payload {
			t.Fatal(r)
		}
	}
	for name, v := range map[string]int32{"prepare": prepare.Load(), "finish": finish.Load(), "begin": begin.Load(), "response": response.Load(), "complete": complete.Load(), "business": handler.calls.Load(), "client marshal": cp.writes.Load(), "server marshal": sp.writes.Load(), "client meta": cm.writes.Load(), "server meta": sm.writes.Load()} {
		if v != 2 {
			t.Errorf("%s = %d, want 2", name, v)
		}
	}
}

func TestUnaryLifecycleReject(t *testing.T) {
	var complete, finish atomic.Int32
	rejected := errors.New("mesh ACL denied")
	addr, handler := startUnaryServer(t, &unaryHooks{
		begin: func(context.Context, *remote.UnaryCall) error { return rejected },
		complete: func(ctx context.Context, c *remote.UnaryCall) error {
			complete.Add(1)
			if !errors.Is(c.Err, rejected) {
				t.Errorf("complete error: %v", c.Err)
			}
			return nil
		},
	})
	cli := unaryClient(t, addr, &unaryHooks{finish: func(ctx context.Context, c *remote.UnaryCall) error {
		finish.Add(1)
		if c.Err == nil {
			t.Error("missing reject error")
		}
		return nil
	}})
	_, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "deny"})
	if err == nil {
		t.Fatal("expected ACL rejection")
	}
	if handler.calls.Load() != 0 || complete.Load() != 1 || finish.Load() != 1 {
		t.Fatal("reject lifecycle incomplete")
	}
}

func TestUnaryLifecycleConnIdentity(t *testing.T) {
	for _, short := range []bool{false, true} {
		name := "long"
		if short {
			name = "short"
		}
		t.Run(name, func(t *testing.T) {
			type identity struct {
				id     uint64
				reused bool
			}
			ingress, egress := make(chan identity, 4), make(chan identity, 4)
			addr, _ := startUnaryServer(t, &unaryHooks{begin: func(ctx context.Context, c *remote.UnaryCall) error {
				ingress <- identity{c.ConnID, c.Reused}
				if c.Begin.IsZero() || c.ReadEnd.Before(c.Begin) || c.Request.RPCInfo().Stats().RecvSize() <= uint64(c.Request.PayloadLen()) {
					return errors.New("ingress metadata unavailable before begin")
				}
				return nil
			}})
			n := 0
			h := &unaryHooks{finish: func(ctx context.Context, c *remote.UnaryCall) error {
				egress <- identity{c.ConnID, c.Reused}
				n++
				if !short && n%2 == 0 {
					rpcinfo.AsTaggable(c.Request.RPCInfo().To()).SetTag(rpcinfo.ConnResetTag, "1")
				}
				return nil
			}}
			var opts []client.Option
			if short {
				opts = append(opts, client.WithShortConnection())
			}
			cli := unaryClient(t, addr, h, opts...)
			previous := [2]identity{}
			for i := 0; i < 4; i++ {
				if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "identity"}); err != nil {
					t.Fatal(err)
				}
				current := [2]identity{<-ingress, <-egress}
				reused := !short && i%2 != 0
				for side, got := range current {
					if got.id == 0 || got.reused != reused {
						t.Fatalf("request %d side %d: %+v, want reused=%v and nonzero ID", i, side, got, reused)
					}
					if i > 0 && (got.id == previous[side].id) != reused {
						t.Fatalf("request %d side %d: ID %d -> %d, reused=%v", i, side, previous[side].id, got.id, reused)
					}
				}
				if current[0].id == current[1].id {
					t.Fatal("client and server IDs collided")
				}
				previous = current
			}
		})
	}
}

func TestUnaryLifecycleTimeoutOwnership(t *testing.T) {
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	addr, _ := startUnaryServer(t, &unaryHooks{begin: func(ctx context.Context, c *remote.UnaryCall) error {
		if c.Request.Data().(*kt.TestServiceEchoPingPongArgs).Req.Message == "timeout" {
			close(entered)
			<-release
		}
		return nil
	}})
	cli := unaryClient(t, addr, &unaryHooks{
		prepare: func(ctx context.Context, c *remote.UnaryCall) error {
			c.State = c.Request.Data().(*kt.TestServiceEchoPingPongArgs).Req.Message == "timeout"
			return nil
		},
		finish: func(ctx context.Context, c *remote.UnaryCall) error {
			if c.State == true {
				if c.Err == nil {
					t.Error("missing timeout at finish")
				}
				close(finished)
			} else if c.Err != nil {
				t.Errorf("second call failed: %v", c.Err)
			}
			return nil
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := cli.EchoPingPong(ctx, &kt.Request{Message: "timeout"})
	if err == nil {
		t.Fatal("expected timeout")
	}
	<-entered
	select {
	case <-finished:
		t.Fatal("native state finished while IO was pending")
	default:
	}
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "second"}); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("native state was not finalized")
	}
}

func TestUnaryLifecyclePrepareTimeout(t *testing.T) {
	var finishes atomic.Int32
	var changed atomic.Bool
	addr, handler := startUnaryServer(t, &unaryHooks{})
	cli := unaryClient(t, addr, &unaryHooks{
		prepare: func(ctx context.Context, c *remote.UnaryCall) error {
			if err := rpcinfo.AsMutableRPCConfig(c.Request.RPCInfo().Config()).SetRPCTimeout(time.Nanosecond); err != nil {
				return err
			}
			changed.Store(true)
			return nil
		},
		finish: func(ctx context.Context, c *remote.UnaryCall) error {
			finishes.Add(1)
			if c.Err == nil || c.FailureStage != remote.UnaryFailurePrepare || c.Conn != nil {
				t.Errorf("bad prepare timeout: %+v", c)
			}
			return nil
		},
	}, client.Option{F: func(o *client.Options, _ *utils.Slice) { o.Locks.Bits &^= rpcinfo.BitRPCTimeout }})
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "expired"}); err == nil {
		t.Fatal("expected prepare timeout")
	}
	if !changed.Load() || finishes.Load() != 1 || handler.calls.Load() != 0 {
		t.Fatal("prepare timeout started IO or failed to finish")
	}
}

func TestUnaryLifecycleNoReuse(t *testing.T) {
	var previous net.Conn
	var calls atomic.Int32
	addr, _ := startUnaryServer(t, &unaryHooks{})
	cli := unaryClient(t, addr, &unaryHooks{finish: func(ctx context.Context, c *remote.UnaryCall) error {
		if c.Conn == previous {
			t.Error("discarded connection reused")
		}
		previous = c.Conn
		calls.Add(1)
		rpcinfo.AsTaggable(c.Request.RPCInfo().To()).SetTag(rpcinfo.ConnResetTag, "1")
		return nil
	}})
	for i := 0; i < 2; i++ {
		if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "fresh"}); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 2 {
		t.Fatal("finish count")
	}
}

func TestUnaryLifecycleEarlyErrors(t *testing.T) {
	addr, _ := startUnaryServer(t, &unaryHooks{})
	for _, stage := range []string{"prepare", "marshal", "metadata", "connect"} {
		t.Run(stage, func(t *testing.T) {
			var finishes atomic.Int32
			failed := errors.New(stage)
			h := &unaryHooks{finish: func(ctx context.Context, c *remote.UnaryCall) error {
				finishes.Add(1)
				if c.Err == nil {
					t.Error("missing error")
				}
				want := map[string]remote.UnaryFailureStage{"prepare": remote.UnaryFailurePrepare, "marshal": remote.UnaryFailureSend, "connect": remote.UnaryFailureConnect}[stage]
				if c.FailureStage != want {
					t.Errorf("failure stage = %v, want %v", c.FailureStage, want)
				}
				return nil
			}}
			var opts []client.Option
			switch stage {
			case "prepare":
				h.prepare = func(context.Context, *remote.UnaryCall) error { return failed }
			case "marshal":
				opts = append(opts, client.WithPayloadCodec(&countingPayload{PayloadCodec: thriftcodec.NewThriftCodec(), err: failed}))
			case "metadata":
				opts = append(opts, client.WithMetaHandler(&unaryMeta{writeErr: failed}))
			}
			target := addr
			if stage == "connect" {
				target = "127.0.0.1:1"
			}
			cli := unaryClient(t, target, h, opts...)
			if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "error"}); err == nil {
				t.Fatal("expected error")
			}
			want := int32(0)
			if stage != "metadata" {
				want = 1
			}
			if finishes.Load() != want {
				t.Fatalf("finish = %d, want %d", finishes.Load(), want)
			}
		})
	}
}

type unaryOutbound func(context.Context, net.Conn, remote.Message) (context.Context, error)

func (f unaryOutbound) Write(ctx context.Context, conn net.Conn, m remote.Message) (context.Context, error) {
	return f(ctx, conn, m)
}

func TestUnaryLifecycleIOErrors(t *testing.T) {
	for _, stage := range []string{"outbound", "flush", "read"} {
		t.Run(stage, func(t *testing.T) {
			var sh unaryHooks
			if stage == "read" {
				sh.begin = func(ctx context.Context, c *remote.UnaryCall) error { return c.Conn.Close() }
			}
			addr, _ := startUnaryServer(t, &sh)
			finished := make(chan remote.UnaryFailureStage, 1)
			h := &unaryHooks{finish: func(ctx context.Context, c *remote.UnaryCall) error {
				if c.Err == nil {
					t.Error("missing IO error")
				}
				finished <- c.FailureStage
				return nil
			}}
			var opts []client.Option
			if stage != "read" {
				opts = append(opts, client.WithBoundHandler(unaryOutbound(func(ctx context.Context, conn net.Conn, m remote.Message) (context.Context, error) {
					if remote.UnaryCallFromContext(ctx).ConnectEnd.IsZero() {
						t.Error("outbound ran before connect")
					}
					if stage == "outbound" {
						return ctx, errors.New("outbound failed")
					}
					conn.Close()
					return ctx, nil
				})))
			}
			cli := unaryClient(t, addr, h, opts...)
			if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "error"}); err == nil {
				t.Fatal("expected IO error")
			}
			want := remote.UnaryFailureSend
			if stage == "read" {
				want = remote.UnaryFailureReceive
			}
			select {
			case got := <-finished:
				if got != want {
					t.Fatalf("stage = %v, want %v", got, want)
				}
			case <-time.After(time.Second):
				t.Fatal("finish missing")
			}
		})
	}
}

func TestUnaryLifecycleResponseErrors(t *testing.T) {
	for _, stage := range []string{"metadata", "response", "marshal", "encode", "flush"} {
		t.Run(stage, func(t *testing.T) {
			completed := make(chan remote.UnaryFailureStage, 1)
			sh := &unaryHooks{complete: func(ctx context.Context, c *remote.UnaryCall) error {
				if c.Err == nil {
					t.Error("missing server error")
				}
				if stage == "flush" && (c.WriteStart.IsZero() || c.WriteEnd.Before(c.WriteStart)) {
					t.Error("missing failed flush timing")
				}
				completed <- c.FailureStage
				return nil
			}}
			var opts []server.Option
			switch stage {
			case "metadata":
				opts = append(opts, server.WithMetaHandler(&unaryMeta{writeErr: errors.New("server metadata")}))
			case "response":
				sh.response = func(context.Context, *remote.UnaryCall) error { return errors.New("server response") }
			case "marshal":
				opts = append(opts, server.WithPayloadCodec(&countingPayload{PayloadCodec: thriftcodec.NewThriftCodec(), err: errors.New("server marshal")}))
			case "encode":
				sh.response = func(ctx context.Context, c *remote.UnaryCall) error {
					c.Response.TransInfo().TransStrInfo()["too-long"] = strings.Repeat("x", 1<<17)
					return nil
				}
			case "flush":
				sh.response = func(ctx context.Context, c *remote.UnaryCall) error { return c.Conn.Close() }
			}
			addr, _ := startUnaryServer(t, sh, opts...)
			cli := unaryClient(t, addr, &unaryHooks{})
			if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "error"}); err == nil {
				t.Fatal("expected response error")
			}
			select {
			case got := <-completed:
				want := remote.UnaryFailureSend
				if stage == "response" {
					want = remote.UnaryFailurePrepare
				}
				if got != want {
					t.Fatalf("stage = %v", got)
				}
			case <-time.After(time.Second):
				t.Fatal("complete missing")
			}
		})
	}
}

func TestUnaryLifecycleRetry(t *testing.T) {
	addr, _ := startUnaryServer(t, &unaryHooks{})
	var prepares, finishes atomic.Int32
	h := &unaryHooks{
		prepare: func(ctx context.Context, c *remote.UnaryCall) error {
			n := prepares.Add(1)
			c.State = n
			if n == 1 {
				remoteinfo.AsRemoteInfo(c.Request.RPCInfo().To()).SetInstance(discovery.NewInstance("tcp", "127.0.0.1:1", 1, nil))
			}
			return nil
		},
		finish: func(ctx context.Context, c *remote.UnaryCall) error {
			n := finishes.Add(1)
			if c.State != n {
				t.Errorf("attempt state %v, finish %v", c.State, n)
			}
			return nil
		},
	}
	p := retry.NewFailurePolicyWithResultRetry(retry.AllErrorRetry())
	p.WithMaxRetryTimes(1)
	cli := unaryClient(t, addr, h, client.WithFailureRetry(p))
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "retry"}); err != nil {
		t.Fatal(err)
	}
	if prepares.Load() != 2 || finishes.Load() != 2 {
		t.Fatal("attempts were not isolated")
	}
}

func TestUnaryLifecycleRejectProtocol(t *testing.T) {
	_, err := testservice.NewClient("unsupported", client.WithUnaryLifecycle(&unaryHooks{}), client.WithTransportProtocol(transport.Framed))
	if !errors.Is(err, remote.ErrUnaryLifecycleProtocol) {
		t.Fatal(err)
	}
}

func TestUnaryLifecycleBusinessError(t *testing.T) {
	completed := make(chan remote.UnaryFailureStage, 1)
	addr, _ := startUnaryServer(t, &unaryHooks{complete: func(ctx context.Context, c *remote.UnaryCall) error { completed <- c.FailureStage; return nil }}, server.WithMiddleware(func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error { return errors.New("business failed") }
	}))
	cli := unaryClient(t, addr, &unaryHooks{})
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{}); err == nil {
		t.Fatal("expected business error")
	}
	if got := <-completed; got != remote.UnaryFailureBusiness {
		t.Fatalf("stage = %v", got)
	}
}

func TestUnaryLifecycleExceptionClassification(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		name := "valid"
		if malformed {
			name = "malformed"
		}
		t.Run(name, func(t *testing.T) {
			finished := make(chan remote.UnaryFailureStage, 1)
			sh := &unaryHooks{response: func(ctx context.Context, c *remote.UnaryCall) error {
				if c.ResponseWire.MessageType != uint32(remote.Exception) || c.Response.PayloadLen() != 0 {
					return errors.New("invalid exception wire information before encoding")
				}
				return nil
			}}
			addr, _ := startUnaryServer(t, sh, server.WithMiddleware(func(next endpoint.Endpoint) endpoint.Endpoint {
				return func(ctx context.Context, req, resp any) error {
					return remote.NewTransErrorWithMsg(remote.ProtocolError, "peer reported a protocol error")
				}
			}), server.WithPayloadCodec(&countingPayload{PayloadCodec: thriftcodec.NewThriftCodec(), malformedException: malformed}))
			cli := unaryClient(t, addr, &unaryHooks{finish: func(ctx context.Context, c *remote.UnaryCall) error {
				if c.Response.MessageType() != remote.Exception || c.ResponseWire.MessageType != uint32(remote.Exception) {
					t.Errorf("message type = %v, want Exception", c.Response.MessageType())
				}
				var application *thrift.ApplicationException
				if errors.As(c.Err, &application) == malformed {
					t.Errorf("unexpected decoded error type: %T: %v", c.Err, c.Err)
				}
				finished <- c.FailureStage
				return nil
			}})
			if _, err := cli.EchoPingPong(context.Background(), &kt.Request{}); err == nil {
				t.Fatal("expected exception error")
			}
			want := remote.UnaryFailureBusiness
			if malformed {
				want = remote.UnaryFailureReceive
			}
			select {
			case got := <-finished:
				if got != want {
					t.Fatalf("failure stage = %v, want %v", got, want)
				}
			case <-time.After(time.Second):
				t.Fatal("client finish missing")
			}
		})
	}
}

func BenchmarkUnaryLifecycleNetwork(b *testing.B) {
	for _, hooks := range []bool{false, true} {
		name := "direct"
		if hooks {
			name = "hooks"
		}
		b.Run(name, func(b *testing.B) {
			var sh *unaryHooks
			if hooks {
				sh = &unaryHooks{}
			}
			addr, _ := startUnaryServer(b, sh)
			var cli testservice.Client
			if hooks {
				cli = unaryClient(b, addr, &unaryHooks{})
			} else {
				var err error
				cli, err = testservice.NewClient("direct", client.WithHostPorts(addr), client.WithTransportProtocol(transport.TTHeaderFramed), client.WithRPCTimeout(time.Second))
				if err != nil {
					b.Fatal(err)
				}
			}
			req := &kt.Request{Message: "benchmark"}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := cli.EchoPingPong(context.Background(), req); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
