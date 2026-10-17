package client_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/mocks/thrift/testservice"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/transport"
)

type sizedNativeTestTransport struct {
	*nativeTestTransport
	capacities []int
	acquire    func(context.Context, int) (remote.NativeBuffer, error)
}

func (n *sizedNativeTestTransport) Connect(ctx context.Context, call *remote.UnaryCall) (remote.NativeClientConnection, remote.NativeIO, error) {
	_, facts, err := n.nativeTestTransport.Connect(ctx, call)
	return n, facts, err
}

func (n *sizedNativeTestTransport) AcquireTxSized(ctx context.Context, capacity int) (remote.NativeBuffer, error) {
	n.capacities = append(n.capacities, capacity)
	if n.acquire != nil {
		return n.acquire(ctx, capacity)
	}
	return &nativeTestBuffer{data: make([]byte, capacity)}, nil
}

func (n *sizedNativeTestTransport) Exchange(ctx context.Context, tx remote.NativeBuffer, length int) (remote.NativeBuffer, remote.NativeIO, error) {
	if len(tx.Bytes()) != length {
		capacity := len(tx.Bytes())
		tx.Release()
		return nil, remote.NativeIO{}, fmt.Errorf("native capacity=%d wire length=%d", capacity, length)
	}
	return n.nativeTestTransport.Exchange(ctx, tx, length)
}

func TestNativeTransportAcquiresExactWireCapacity(t *testing.T) {
	address, _ := startUnaryServer(t, nil)
	bridge := &sizedNativeTestTransport{nativeTestTransport: &nativeTestTransport{unaryHooks: &unaryHooks{prepare: func(_ context.Context, call *remote.UnaryCall) error {
		headers := remote.AccessHeaders(call.Request.TransInfo())
		headers.SetString("late-governance", "after-prepare")
		return nil
	}}, address: address}}
	t.Cleanup(func() { bridge.Release(false) })
	cli, err := testservice.NewClient("native.target", client.WithNativeTransport(bridge), client.WithTransportProtocol(transport.TTHeaderFramed), client.WithRPCTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for index, size := range []int{1024, 16384, 131 << 10} {
		value := strings.Repeat("a", size)
		response, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: value})
		if err != nil {
			t.Fatal(err)
		}
		if response.Message != "echo:"+value || len(bridge.capacities) != index+1 || bridge.capacities[index] <= len(value) || bridge.capacities[index] >= size+4096 {
			t.Fatalf("native exact capacity lost the request or headers: size=%d response=%+v capacities=%v", size, response, bridge.capacities)
		}
	}
}

type capacityReleaseBuffer struct {
	nativeTestBuffer
	releases *atomic.Int32
}

func (b *capacityReleaseBuffer) Release() error {
	b.releases.Add(1)
	return b.nativeTestBuffer.Release()
}

func TestNativeTransportReleasesSizedTxAfterCancellation(t *testing.T) {
	address, _ := startUnaryServer(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var releases, exchanges atomic.Int32
	bridge := &sizedNativeTestTransport{nativeTestTransport: &nativeTestTransport{unaryHooks: &unaryHooks{}, address: address,
		exchange: func(context.Context, remote.NativeBuffer, int) (remote.NativeBuffer, remote.NativeIO, error) {
			exchanges.Add(1)
			return nil, remote.NativeIO{}, fmt.Errorf("canceled TX reached Exchange")
		}}, acquire: func(context.Context, int) (remote.NativeBuffer, error) {
		cancel()
		return &capacityReleaseBuffer{nativeTestBuffer: nativeTestBuffer{data: make([]byte, 4096)}, releases: &releases}, nil
	}}
	t.Cleanup(func() { bridge.Release(false) })
	cli, err := testservice.NewClient("native.target", client.WithNativeTransport(bridge), client.WithTransportProtocol(transport.TTHeaderFramed), client.WithRPCTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cli.EchoPingPong(ctx, &kt.Request{Message: "cancel-after-acquire"}); err == nil || releases.Load() != 1 || exchanges.Load() != 0 {
		t.Fatalf("canceled sized TX escaped cleanup: error=%v releases=%d exchanges=%d", err, releases.Load(), exchanges.Load())
	}
}
