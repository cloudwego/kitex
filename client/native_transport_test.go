package client_test

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/internal/mocks"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/mocks/thrift/testservice"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/transport"
)

func TestNativeTransportRejectsMissingBridge(t *testing.T) {
	cli, err := client.NewClient(mocks.ServiceInfo(), client.WithDestService("native.target"),
		client.WithTransportProtocol(transport.TTHeaderFramed), client.WithNativeTransport(nil))
	if err == nil {
		cli.(interface{ Close() error }).Close()
		t.Fatal("native transport without a bridge was accepted")
	}
}

type nativeTestBuffer struct{ data []byte }

func (b *nativeTestBuffer) Bytes() []byte { return b.data }
func (b *nativeTestBuffer) Release() error {
	for i := range b.data {
		b.data[i] = 0xa5
	}
	b.data = nil
	return nil
}

type nativeTestTransport struct {
	*unaryHooks
	address string
	conn    net.Conn
}

func (n *nativeTestTransport) BuiltinMetaHandlers() []remote.MetaHandler { return nil }
func (n *nativeTestTransport) Connect(ctx context.Context, _ *remote.UnaryCall) (remote.NativeClientConnection, remote.NativeIO, error) {
	facts := remote.NativeIO{ConnID: 1, Reused: n.conn != nil, ConnectStart: time.Now()}
	if n.conn == nil {
		var err error
		n.conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", n.address)
		if err != nil {
			return nil, facts, err
		}
	}
	facts.ConnectEnd = time.Now()
	return n, facts, nil
}
func (n *nativeTestTransport) AcquireTx(context.Context) (remote.NativeBuffer, error) {
	return &nativeTestBuffer{data: make([]byte, 1<<20)}, nil
}
func (n *nativeTestTransport) Exchange(ctx context.Context, tx remote.NativeBuffer, length int) (remote.NativeBuffer, remote.NativeIO, error) {
	defer tx.Release()
	facts := remote.NativeIO{ConnID: 1, WriteStart: time.Now()}
	if deadline, ok := ctx.Deadline(); ok {
		n.conn.SetDeadline(deadline)
	}
	count, err := n.conn.Write(tx.Bytes()[:length])
	facts.SentBytes = uint64(count)
	facts.WriteEnd = time.Now()
	if err != nil {
		return nil, facts, err
	}
	var prefix [4]byte
	if _, err = io.ReadFull(n.conn, prefix[:]); err != nil {
		return nil, facts, err
	}
	rx := &nativeTestBuffer{data: make([]byte, int(binary.BigEndian.Uint32(prefix[:]))+4)}
	copy(rx.data, prefix[:])
	if _, err = io.ReadFull(n.conn, rx.data[4:]); err != nil {
		rx.Release()
		return nil, facts, err
	}
	facts.ReceivedBytes = uint64(len(rx.data))
	facts.ReadEnd = time.Now()
	return rx, facts, nil
}
func (n *nativeTestTransport) Release(reuse bool) error {
	if !reuse && n.conn != nil {
		err := n.conn.Close()
		n.conn = nil
		return err
	}
	return nil
}

func TestNativeTransportCallsExistingGoServer(t *testing.T) {
	address, _ := startUnaryServer(t, nil)
	bridge := &nativeTestTransport{unaryHooks: &unaryHooks{}, address: address}
	t.Cleanup(func() { bridge.Release(false) })
	cli, err := testservice.NewClient("native.target", client.WithNativeTransport(bridge),
		client.WithTransportProtocol(transport.TTHeaderFramed), client.WithRPCTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	first, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "native"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: strings.Repeat("b", 16384)})
	if err != nil {
		t.Fatal(err)
	}
	if first.Message != "echo:native" || second.Message != "echo:"+strings.Repeat("b", 16384) {
		t.Fatal("RPC results did not survive release and reuse of the input frame")
	}
}

func TestNativeTransportRejectsCustomDataPlaneInEitherOptionOrder(t *testing.T) {
	bridge := &nativeTestTransport{unaryHooks: &unaryHooks{}}
	for _, custom := range []client.Option{
		client.WithCodec(codec.NewDefaultCodec()),
		client.WithMetaHandler(&unaryMeta{}),
		client.WithUnaryLifecycle(&unaryHooks{}),
		client.WithFailureRetry(retry.NewFailurePolicy()),
		client.WithLoadBalancer(loadbalance.NewWeightedBalancer()),
		client.WithInstanceMW(func(next endpoint.Endpoint) endpoint.Endpoint { return next }),
	} {
		for _, options := range [][]client.Option{
			{custom, client.WithNativeTransport(bridge)},
			{client.WithNativeTransport(bridge), custom},
		} {
			options = append(options, client.WithTransportProtocol(transport.TTHeaderFramed))
			cli, err := testservice.NewClient("native.target", options...)
			if err == nil {
				t.Fatalf("custom transport configuration was accepted: %T", cli)
			}
		}
	}
}

func TestNativeTransportRejectsPerCallRetry(t *testing.T) {
	bridge := &nativeTestTransport{unaryHooks: &unaryHooks{}}
	cli, err := testservice.NewClient("native.target", client.WithNativeTransport(bridge), client.WithTransportProtocol(transport.TTHeaderFramed))
	if err != nil {
		t.Fatal(err)
	}
	_, err = cli.EchoPingPong(context.Background(), &kt.Request{Message: "retry"}, callopt.WithRetryPolicy(retry.BuildFailurePolicy(retry.NewFailurePolicy())))
	if err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("unexpected retry result: %v", err)
	}
}

func TestNativeTransportRejectsPerCallGoAddress(t *testing.T) {
	bridge := &nativeTestTransport{unaryHooks: &unaryHooks{}}
	cli, err := testservice.NewClient("native.target", client.WithNativeTransport(bridge), client.WithTransportProtocol(transport.TTHeaderFramed))
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range []callopt.Option{callopt.WithHostPort("127.0.0.1:1"), callopt.WithURL("http://127.0.0.1:1")} {
		_, err = cli.EchoPingPong(context.Background(), &kt.Request{Message: "route"}, option)
		if err == nil || !strings.Contains(err.Error(), "routing") {
			t.Fatalf("unexpected Go address override result: %v", err)
		}
	}
}
