package server_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/internal/mocks"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/mocks/thrift/testservice"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/registry"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	thriftcodec "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/transport"
)

func TestNativeServerRejectsMissingBridge(t *testing.T) {
	svr := server.NewServer(server.WithServiceAddr(utils.NewNetAddr("tcp", "127.0.0.1:0")), server.WithNativeTransport(nil))
	if err := svr.RegisterService(mocks.ServiceInfo(), mocks.MyServiceHandler()); err != nil {
		t.Fatal(err)
	}
	if err := svr.Run(); err == nil || !strings.Contains(err.Error(), "native") {
		t.Fatalf("native server without a bridge was accepted: %v", err)
	}
}

func TestNativeServerRejectsCustomTransportInEitherOrder(t *testing.T) {
	nativeFixtureDefaultCodec(t)
	for _, custom := range []server.Option{server.WithCodec(codec.NewDefaultCodec()), server.WithPayloadCodec(thriftcodec.NewThriftCodecWithConfig(thriftcodec.FrugalReadWrite)), server.WithMetaHandler(nativeFixtureMeta{new(nativeServerFixture)}), server.WithProfiler(nil)} {
		for _, options := range [][]server.Option{{custom, server.WithNativeTransport(new(nativeServerFixture))}, {server.WithNativeTransport(new(nativeServerFixture)), custom}} {
			options = append(options, server.WithServiceAddr(utils.NewNetAddr("tcp", "127.0.0.1:0")), server.WithExitSignal(func() <-chan error { return make(chan error) }))
			svr := server.NewServer(options...)
			if err := svr.RegisterService(testservice.NewServiceInfo(), new(nativeFixtureEcho)); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- svr.Run() }()
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "native") {
					t.Fatalf("unexpected option result: %v", err)
				}
			case <-time.After(100 * time.Millisecond):
				svr.Stop()
				<-done
				t.Fatal("native server accepted custom transport configuration")
			}
		}
	}
}

type nativeCodecObserverSuite struct {
	codec remote.PayloadCodec
	err   error
}

func (s *nativeCodecObserverSuite) Options() []server.Option {
	info := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil)
	message := remote.NewMessage(nil, info, remote.Call, remote.Server)
	s.codec, s.err = remote.GetPayloadCodec(message)
	remote.RecycleMessage(message)
	rpcinfo.PutRPCInfo(info)
	return nil
}

func TestServerPayloadCodecIsVisibleToFollowingSuite(t *testing.T) {
	nativeFixtureDefaultCodec(t)
	want := thriftcodec.NewThriftCodecDisableFastMode(false, true)
	observer := new(nativeCodecObserverSuite)
	server.NewServer(server.WithPayloadCodec(want), server.WithSuite(observer))
	if observer.err != nil || !reflect.DeepEqual(observer.codec, want) {
		t.Fatalf("following suite could not observe the preceding codec option: codec=%v error=%v", observer.codec, observer.err)
	}
}

type nativeServerFixture struct {
	reads, begins, completes atomic.Int32
	txCapacity               int
	beginErr                 error
	readErr                  error
	sendErr                  error
	corrupt                  bool
	released                 atomic.Int32
	begin, readEnd           time.Time
	admissionTimeout         time.Duration
	projectBase              bool
	completed                chan nativeFixtureCompletion
	request                  func(*nativeFixtureRequest) remote.NativeServerRequest
}

type nativeFixtureCompletion struct {
	err   error
	stage remote.UnaryFailureStage
}

func (n *nativeServerFixture) ClientPrepare(context.Context, *remote.UnaryCall) error { return nil }
func (n *nativeServerFixture) ClientFinish(context.Context, *remote.UnaryCall) error  { return nil }
func (n *nativeServerFixture) ServerBegin(ctx context.Context, c *remote.UnaryCall) error {
	if ctx.Value(nativeMetaKey{}) != "read" {
		return io.ErrUnexpectedEOF
	}
	n.begins.Add(1)
	if n.admissionTimeout > 0 {
		rpcinfo.AsMutableRPCConfig(c.Request.RPCInfo().Config()).SetRPCTimeout(n.admissionTimeout)
	}
	return n.beginErr
}
func (n *nativeServerFixture) ServerResponse(context.Context, *remote.UnaryCall) error { return nil }
func (n *nativeServerFixture) ServerComplete(_ context.Context, call *remote.UnaryCall) error {
	n.completes.Add(1)
	if n.completed != nil {
		n.completed <- nativeFixtureCompletion{err: call.Err, stage: call.FailureStage}
	}
	return nil
}
func (n *nativeServerFixture) BuiltinMetaHandlers() []remote.MetaHandler {
	return []remote.MetaHandler{nativeFixtureMeta{n}}
}
func (n *nativeServerFixture) ProjectIngress(base, decoded context.Context, _ *remote.UnaryCall) (context.Context, error) {
	if n.projectBase {
		return base, nil
	}
	return decoded, nil
}
func (n *nativeServerFixture) Listen(_ context.Context, address net.Addr, _ remote.NativeListenerOptions) (remote.NativeListener, error) {
	ln, err := net.Listen(address.Network(), address.String())
	if err != nil {
		return nil, err
	}
	return &nativeFixtureListener{Listener: ln, bridge: n}, nil
}

type nativeMetaKey struct{}
type nativeFixtureMeta struct{ server *nativeServerFixture }

func (m nativeFixtureMeta) ReadMeta(ctx context.Context, _ remote.Message) (context.Context, error) {
	m.server.reads.Add(1)
	ctx = metainfo.WithValue(ctx, "forward-once", "kept")
	return context.WithValue(ctx, nativeMetaKey{}, "read"), m.server.readErr
}
func (nativeFixtureMeta) WriteMeta(ctx context.Context, _ remote.Message) (context.Context, error) {
	return ctx, nil
}

type nativeFixtureListener struct {
	net.Listener
	bridge   *nativeServerFixture
	mu       sync.Mutex
	conn     net.Conn
	previous <-chan struct{}
}

func (l *nativeFixtureListener) Address() net.Addr { return l.Addr() }
func (l *nativeFixtureListener) Accept(ctx context.Context) (remote.NativeServerRequest, error) {
	if l.previous != nil {
		select {
		case <-l.previous:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	l.mu.Lock()
	conn := l.conn
	l.mu.Unlock()
	if conn == nil {
		var err error
		conn, err = l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		l.mu.Lock()
		l.conn = conn
		l.mu.Unlock()
	}
	var prefix [4]byte
	if _, err := io.ReadFull(conn, prefix[:]); err != nil {
		return nil, err
	}
	data := make([]byte, int(binary.BigEndian.Uint32(prefix[:]))+4)
	copy(data, prefix[:])
	if _, err := io.ReadFull(conn, data[4:]); err != nil {
		return nil, err
	}
	if l.bridge.corrupt {
		for i := 4; i < len(data); i++ {
			data[i] = 0
		}
	}
	r := &nativeFixtureRequest{listener: l, conn: conn, frame: &nativeFixtureBuffer{data: data}, done: make(chan struct{})}
	l.previous = r.done
	if l.bridge.request != nil {
		return l.bridge.request(r), nil
	}
	return r, nil
}
func (l *nativeFixtureListener) Close() error {
	l.mu.Lock()
	if l.conn != nil {
		l.conn.Close()
		l.conn = nil
	}
	l.mu.Unlock()
	return l.Listener.Close()
}

type nativeFixtureBuffer struct{ data []byte }

func (b *nativeFixtureBuffer) Bytes() []byte { return b.data }
func (b *nativeFixtureBuffer) Release() error {
	for i := range b.data {
		b.data[i] = 0xa5
	}
	b.data = nil
	return nil
}

type nativeFixtureRequest struct {
	listener *nativeFixtureListener
	conn     net.Conn
	frame    remote.NativeBuffer
	done     chan struct{}
}

func (r *nativeFixtureRequest) Context() context.Context   { return context.Background() }
func (r *nativeFixtureRequest) Frame() remote.NativeBuffer { return r.frame }
func (r *nativeFixtureRequest) Facts() remote.NativeIngress {
	begin, end := r.listener.bridge.begin, r.listener.bridge.readEnd
	if begin.IsZero() {
		begin = time.Now()
	}
	if end.IsZero() {
		end = time.Now()
	}
	return remote.NativeIngress{IO: remote.NativeIO{ConnID: 1, ReadEnd: end, ReceivedBytes: uint64(len(r.frame.Bytes()))}, Begin: begin, Peer: r.conn.RemoteAddr(), Local: r.conn.LocalAddr()}
}

type nativeFixtureTrace struct{ finished chan nativeFixtureTraceFacts }
type nativeFixtureTraceFacts struct{ rpc, read, readEnd, write, writeEnd time.Time }

func (t *nativeFixtureTrace) Start(ctx context.Context) context.Context { return ctx }
func (t *nativeFixtureTrace) Finish(ctx context.Context) {
	st := rpcinfo.GetRPCInfo(ctx).Stats()
	stamp := func(event stats.Event) time.Time {
		if value := st.GetEvent(event); value != nil {
			return value.Time()
		}
		return time.Time{}
	}
	t.finished <- nativeFixtureTraceFacts{rpc: stamp(stats.RPCStart), read: stamp(stats.ReadStart), readEnd: stamp(stats.ReadFinish), write: stamp(stats.WriteStart), writeEnd: stamp(stats.WriteFinish)}
}

func TestNativeServerTraceUsesNativeIOTime(t *testing.T) {
	bridge := &nativeServerFixture{begin: time.Now().Add(-20 * time.Millisecond), readEnd: time.Now().Add(-10 * time.Millisecond)}
	tracer := &nativeFixtureTrace{finished: make(chan nativeFixtureTraceFacts, 1)}
	cli, _ := startNativeFixture(t, bridge, new(nativeFixtureEcho), server.WithTracer(tracer), server.WithStatsLevel(stats.LevelDetailed))
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "trace"}); err != nil {
		t.Fatal(err)
	}
	select {
	case facts := <-tracer.finished:
		if !facts.rpc.Equal(bridge.begin) || !facts.read.Equal(bridge.begin) || !facts.readEnd.Equal(bridge.readEnd) || facts.write.IsZero() || facts.writeEnd.Before(facts.write) {
			t.Fatalf("native I/O was replaced by Go task time: %+v", facts)
		}
	case <-time.After(time.Second):
		t.Fatal("native request did not finish tracing")
	}
}
func (r *nativeFixtureRequest) AcquireTx(context.Context) (remote.NativeBuffer, error) {
	capacity := r.listener.bridge.txCapacity
	if capacity == 0 {
		capacity = 1 << 20
	}
	return &nativeFixtureBuffer{data: make([]byte, capacity)}, nil
}

func TestNativeServerReportsResponseEncodingFailure(t *testing.T) {
	bridge := &nativeServerFixture{txCapacity: 16, completed: make(chan nativeFixtureCompletion, 2)}
	handler := new(nativeFixtureEcho)
	cli, _ := startNativeFixture(t, bridge, handler)
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "small-TX"}); err == nil {
		t.Fatal("response encoding unexpectedly succeeded with an undersized TX lease")
	}
	select {
	case completion := <-bridge.completed:
		if completion.err == nil || completion.stage != remote.UnaryFailureSend || handler.calls.Load() != 1 {
			t.Fatalf("encoding failure lost its stage: %+v, handlers=%d", completion, handler.calls.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("response encoding failure did not complete governance")
	}
}

func TestNativeServerDeniedRequestSkipsHandlerAndCompletes(t *testing.T) {
	bridge := &nativeServerFixture{beginErr: errors.New("native admission denied"), completed: make(chan nativeFixtureCompletion, 2)}
	handler := new(nativeFixtureEcho)
	cli, _ := startNativeFixture(t, bridge, handler)
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "denied"}); err == nil || !strings.Contains(err.Error(), "native admission denied") {
		t.Fatalf("native denial was lost: %v", err)
	}
	select {
	case completion := <-bridge.completed:
		if completion.err == nil || completion.stage != remote.UnaryFailurePrepare || handler.calls.Load() != 0 || bridge.reads.Load() != 1 {
			t.Fatalf("denied request entered handler or lost completion: %+v, handlers=%d", completion, handler.calls.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("denied request did not complete governance")
	}
}

func startNativeFixture(t *testing.T, bridge *nativeServerFixture, handler kt.TestService, options ...server.Option) (testservice.Client, server.Server) {
	t.Helper()
	nativeFixtureDefaultCodec(t)
	reg := &nativeFixtureRegistry{ready: make(chan net.Addr, 1)}
	options = append(options, server.WithNativeTransport(bridge), server.WithServiceAddr(utils.NewNetAddr("tcp", "127.0.0.1:0")), server.WithRegistry(reg), server.WithExitSignal(func() <-chan error { return make(chan error) }))
	svr := server.NewServer(options...)
	if err := svr.RegisterService(testservice.NewServiceInfo(), handler); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- svr.Run() }()
	t.Cleanup(func() {
		svr.Stop()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("native fixture did not stop")
		}
	})
	var address net.Addr
	select {
	case address = <-reg.ready:
	case err := <-done:
		t.Fatalf("native server failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("native registry never became ready")
	}
	cli, err := testservice.NewClient("native.server", client.WithHostPorts(address.String()), client.WithTransportProtocol(transport.TTHeaderFramed), client.WithRPCTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return cli, svr
}
func (r *nativeFixtureRequest) Send(_ context.Context, tx remote.NativeBuffer, length int) (remote.NativeIO, error) {
	defer tx.Release()
	facts := remote.NativeIO{ConnID: 1, WriteStart: time.Now()}
	n, err := r.conn.Write(tx.Bytes()[:length])
	facts.SentBytes, facts.WriteEnd = uint64(n), time.Now()
	if r.listener.bridge.sendErr != nil {
		err = r.listener.bridge.sendErr
		facts.FailureStage = remote.UnaryFailureSend
	}
	return facts, err
}
func (r *nativeFixtureRequest) Release(reuse bool) error {
	r.listener.bridge.released.Add(1)
	if !reuse {
		r.listener.mu.Lock()
		r.conn.Close()
		r.listener.conn = nil
		r.listener.mu.Unlock()
	}
	close(r.done)
	return nil
}

func TestNativeServerFailuresBeforeAdmissionSkipGovernance(t *testing.T) {
	for _, test := range []struct {
		name   string
		bridge *nativeServerFixture
		reads  int32
	}{
		{name: "decode", bridge: &nativeServerFixture{corrupt: true}},
		{name: "ReadMeta", bridge: &nativeServerFixture{readErr: errors.New("metadata failed")}, reads: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := new(nativeFixtureEcho)
			cli, _ := startNativeFixture(t, test.bridge, handler)
			if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "rejected-before-Begin"}); err == nil {
				t.Fatal("invalid request unexpectedly succeeded")
			}
			deadline := time.Now().Add(time.Second)
			for test.bridge.released.Load() == 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if test.bridge.released.Load() != 1 || test.bridge.reads.Load() != test.reads || test.bridge.begins.Load() != 0 || test.bridge.completes.Load() != 0 || handler.calls.Load() != 0 {
				t.Fatalf("failed decode/ReadMeta entered governance or business: released=%d reads=%d begins=%d completes=%d handlers=%d", test.bridge.released.Load(), test.bridge.reads.Load(), test.bridge.begins.Load(), test.bridge.completes.Load(), handler.calls.Load())
			}
		})
	}
}

func TestNativeServerFullSendWithErrorStillReportsSendFailure(t *testing.T) {
	bridge := &nativeServerFixture{sendErr: io.ErrUnexpectedEOF, completed: make(chan nativeFixtureCompletion, 2)}
	cli, _ := startNativeFixture(t, bridge, new(nativeFixtureEcho))
	cli.EchoPingPong(context.Background(), &kt.Request{Message: "full-write-with-error"})
	select {
	case completion := <-bridge.completed:
		if !errors.Is(completion.err, io.ErrUnexpectedEOF) || completion.stage != remote.UnaryFailureSend {
			t.Fatalf("complete byte count hid send failure: %+v", completion)
		}
	case <-time.After(time.Second):
		t.Fatal("send failure did not complete governance")
	}
}

type nativeHeldEcho struct {
	kt.TestService
	entered chan struct{}
	release chan struct{}
	exited  chan string
}

func (h *nativeHeldEcho) EchoPingPong(_ context.Context, request *kt.Request) (*kt.Response, error) {
	close(h.entered)
	<-h.release
	h.exited <- request.Message
	return &kt.Response{Message: request.Message}, nil
}

func TestNativeServerStopRetainsActiveHandlerUntilExit(t *testing.T) {
	bridge := &nativeServerFixture{completed: make(chan nativeFixtureCompletion, 2)}
	handler := &nativeHeldEcho{entered: make(chan struct{}), release: make(chan struct{}), exited: make(chan string, 1)}
	cli, svr := startNativeFixture(t, bridge, handler, server.WithExitWaitTime(20*time.Millisecond))
	done := make(chan error, 1)
	go func() {
		_, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "held-owned-request"})
		done <- err
	}()
	select {
	case <-handler.entered:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	stopErr := svr.Stop()
	if stopErr == nil || !strings.Contains(stopErr.Error(), "draining") || bridge.completes.Load() != 0 || bridge.released.Load() != 0 {
		close(handler.release)
		t.Fatalf("Stop released a live handler: error=%v completes=%d released=%d", stopErr, bridge.completes.Load(), bridge.released.Load())
	}
	close(handler.release)
	select {
	case message := <-handler.exited:
		if message != "held-owned-request" {
			t.Fatalf("handler object was recycled before exit: %q", message)
		}
	case <-time.After(time.Second):
		t.Fatal("held handler did not exit")
	}
	select {
	case completion := <-bridge.completed:
		if !errors.Is(completion.err, context.Canceled) {
			t.Fatalf("Stop did not cancel the active request: %+v", completion)
		}
	case <-time.After(time.Second):
		t.Fatal("handler exit did not complete governance")
	}
	<-done
}

type nativeFixtureRegistry struct {
	ready   chan net.Addr
	stopped atomic.Bool
}

func (r *nativeFixtureRegistry) Register(info *registry.Info) error { r.ready <- info.Addr; return nil }
func (r *nativeFixtureRegistry) Deregister(*registry.Info) error    { r.stopped.Store(true); return nil }

type nativeFixtureEcho struct {
	kt.TestService
	calls atomic.Int32
	err   error
}

func (h *nativeFixtureEcho) EchoPingPong(ctx context.Context, request *kt.Request) (*kt.Response, error) {
	h.calls.Add(1)
	if h.err != nil {
		return nil, h.err
	}
	return &kt.Response{Message: "native:" + request.Message}, nil
}

func TestNativeServerPreservesBusinessChainAndLifecycle(t *testing.T) {
	nativeFixtureDefaultCodec(t)
	bridge := new(nativeServerFixture)
	reg := &nativeFixtureRegistry{ready: make(chan net.Addr, 1)}
	handler := new(nativeFixtureEcho)
	var middlewares atomic.Int32
	svr := server.NewServer(server.WithNativeTransport(bridge), server.WithServiceAddr(utils.NewNetAddr("tcp", "127.0.0.1:0")),
		server.WithRegistry(reg), server.WithExitSignal(func() <-chan error { return make(chan error) }),
		server.WithMiddleware(func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, req, resp interface{}) error {
				if ctx.Value(nativeMetaKey{}) != "read" {
					return io.ErrUnexpectedEOF
				}
				if value, _ := metainfo.GetValue(ctx, "forward-once"); value != "kept" {
					return errors.New("TransferForward discarded metadata before business middleware")
				}
				middlewares.Add(1)
				return next(ctx, req, resp)
			}
		}))
	if err := svr.RegisterService(testservice.NewServiceInfo(), handler); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- svr.Run() }()
	t.Cleanup(func() {
		if err := svr.Stop(); err != nil {
			t.Error(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("native server did not stop")
		}
	})
	var address net.Addr
	select {
	case address = <-reg.ready:
	case err := <-done:
		t.Fatalf("native server failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("native registry never became ready")
	}
	cli, err := testservice.NewClient("native.server", client.WithHostPorts(address.String()), client.WithTransportProtocol(transport.TTHeaderFramed))
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{"one", strings.Repeat("z", 8192)} {
		response, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: message})
		if err != nil || response.Message != "native:"+message {
			t.Fatalf("native RPC failed: response=%v error=%v", response, err)
		}
	}
	deadline := time.Now().Add(time.Second)
	for bridge.completes.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if handler.calls.Load() != 2 || middlewares.Load() != 2 || bridge.reads.Load() != 2 || bridge.begins.Load() != 2 || bridge.completes.Load() != 2 {
		t.Fatalf("lifecycle mismatch: handler=%d middleware=%d read=%d begin=%d complete=%d", handler.calls.Load(), middlewares.Load(), bridge.reads.Load(), bridge.begins.Load(), bridge.completes.Load())
	}
}

func nativeFixtureDefaultCodec(t *testing.T) {
	t.Helper()
	info := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil)
	message := remote.NewMessage(nil, info, remote.Call, remote.Server)
	original, err := remote.GetPayloadCodec(message)
	remote.RecycleMessage(message)
	rpcinfo.PutRPCInfo(info)
	if err != nil {
		t.Fatal(err)
	}
	remote.PutPayloadCode(serviceinfo.Thrift, thriftcodec.NewThriftCodec())
	t.Cleanup(func() { remote.PutPayloadCode(serviceinfo.Thrift, original) })
}

func TestNativeServerPreservesBusinessErrorHandler(t *testing.T) {
	bridge := &nativeServerFixture{completed: make(chan nativeFixtureCompletion, 2)}
	var errorHandled atomic.Int32
	cli, _ := startNativeFixture(t, bridge, &nativeFixtureEcho{err: errors.New("business failure")}, server.WithErrorHandler(func(_ context.Context, err error) error {
		errorHandled.Add(1)
		return fmt.Errorf("handled: %w", err)
	}))
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "business"}); err == nil || !strings.Contains(err.Error(), "handled:") || !strings.Contains(err.Error(), "business failure") {
		t.Fatalf("native server bypassed the business error handler: %v", err)
	}
	select {
	case completion := <-bridge.completed:
		if completion.err == nil || completion.stage != remote.UnaryFailureBusiness || errorHandled.Load() != 1 {
			t.Fatalf("business failure became a transport failure: %+v, error handler=%d", completion, errorHandled.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("business error did not complete governance")
	}
}

func TestNativeServerKeepsAbsoluteDeadlineAcrossProjection(t *testing.T) {
	bridge := &nativeServerFixture{admissionTimeout: 100 * time.Millisecond, projectBase: true}
	cli, _ := startNativeFixture(t, bridge, new(nativeFixtureEcho), server.WithEnableContextTimeout(true), server.WithMiddleware(func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) error {
			deadline, ok := ctx.Deadline()
			call := remote.UnaryCallFromContext(ctx)
			if !ok || !deadline.Equal(call.Begin.Add(bridge.admissionTimeout)) {
				return errors.New("projection extended the admitted absolute deadline")
			}
			return next(ctx, req, resp)
		}
	}))
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "absolute-deadline"}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeServerRejectsChangedGlobalCodecBeforeListen(t *testing.T) {
	nativeFixtureDefaultCodec(t)
	remote.PutPayloadCode(serviceinfo.Thrift, thriftcodec.NewThriftCodecWithConfig(thriftcodec.FrugalReadWrite))
	svr := server.NewServer(server.WithNativeTransport(new(nativeServerFixture)), server.WithServiceAddr(utils.NewNetAddr("tcp", "127.0.0.1:0")))
	if err := svr.RegisterService(testservice.NewServiceInfo(), new(nativeFixtureEcho)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- svr.Run() }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "Fast/owned") {
			t.Fatalf("unexpected global codec validation: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		svr.Stop()
		<-done
		t.Fatal("native listener started with a globally registered borrowing codec")
	}
}
