package server_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
)

type nativeCapacityRequest struct {
	*nativeFixtureRequest
	capacities chan int
}

func (r *nativeCapacityRequest) AcquireTxSized(_ context.Context, capacity int) (remote.NativeBuffer, error) {
	r.capacities <- capacity
	return &nativeFixtureBuffer{data: make([]byte, capacity)}, nil
}

func (r *nativeCapacityRequest) Send(ctx context.Context, tx remote.NativeBuffer, length int) (remote.NativeIO, error) {
	if len(tx.Bytes()) != length {
		capacity := len(tx.Bytes())
		tx.Release()
		return remote.NativeIO{}, fmt.Errorf("native response capacity=%d wire length=%d", capacity, length)
	}
	return r.nativeFixtureRequest.Send(ctx, tx, length)
}

func TestNativeServerAcquiresExactWireCapacity(t *testing.T) {
	capacities := make(chan int, 3)
	bridge := &nativeServerFixture{completed: make(chan nativeFixtureCompletion, 3), request: func(request *nativeFixtureRequest) remote.NativeServerRequest {
		return &nativeCapacityRequest{nativeFixtureRequest: request, capacities: capacities}
	}}
	cli, _ := startNativeFixture(t, bridge, new(nativeFixtureEcho))
	for _, size := range []int{1024, 16384, 131 << 10} {
		value := strings.Repeat("b", size)
		response, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: value})
		if err != nil {
			select {
			case completion := <-bridge.completed:
				t.Fatalf("native response failed: %v completion=%v stage=%v", err, completion.err, completion.stage)
			case <-time.After(time.Second):
				t.Fatal(err)
			}
		}
		select {
		case capacity := <-capacities:
			if response.Message != "native:"+value || capacity <= len(value) || capacity >= size+4096 {
				t.Fatalf("native response lost capacity or data: size=%d response=%+v capacity=%d", size, response, capacity)
			}
		default:
			t.Fatal("native response did not use sized TX")
		}
	}

}

func TestNativeServerAcquiresSizedException(t *testing.T) {
	capacities := make(chan int, 1)
	bridge := &nativeServerFixture{request: func(request *nativeFixtureRequest) remote.NativeServerRequest {
		return &nativeCapacityRequest{nativeFixtureRequest: request, capacities: capacities}
	}}
	cli, _ := startNativeFixture(t, bridge, &nativeFixtureEcho{err: errors.New("controlled sized exception")})
	if _, err := cli.EchoPingPong(context.Background(), &kt.Request{Message: "exception"}); err == nil || !strings.Contains(err.Error(), "controlled sized exception") {
		t.Fatalf("native exception changed: %v", err)
	}
	select {
	case capacity := <-capacities:
		if capacity <= 4 || capacity >= 4096 {
			t.Fatalf("native exception capacity=%d", capacity)
		}
	default:
		t.Fatal("native exception did not use sized TX")
	}
}
