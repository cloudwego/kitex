package remote_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	thriftcodec "github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

type capacityBuffer struct{ data []byte }

func (b *capacityBuffer) Bytes() []byte  { return b.data }
func (b *capacityBuffer) Release() error { b.data = nil; return nil }

type capacitySource struct {
	capacity, calls int
}

func (*capacitySource) AcquireTx(context.Context) (remote.NativeBuffer, error) {
	return nil, errors.New("unexpected legacy allocation")
}
func (s *capacitySource) AcquireTxSized(_ context.Context, capacity int) (remote.NativeBuffer, error) {
	s.capacity, s.calls = capacity, s.calls+1
	return &capacityBuffer{data: make([]byte, capacity)}, nil
}

type countedNativePayload struct {
	*kt.Request
	length        *int
	sizes, writes int
	cancel        context.CancelFunc
}

func (p *countedNativePayload) BLength() int {
	p.sizes++
	if p.cancel != nil {
		p.cancel()
	}
	if p.length != nil {
		return *p.length
	}
	return p.Request.BLength()
}
func (p *countedNativePayload) FastWriteNocopy(out []byte, writer thrift.NocopyWriter) int {
	p.writes++
	return p.Request.FastWriteNocopy(out, writer)
}

func nativeCapacityMessage(data interface{}, protocol transport.Protocol, kind remote.MessageType) remote.Message {
	config := rpcinfo.NewRPCConfig()
	rpcinfo.AsMutableRPCConfig(config).SetTransportProtocol(protocol)
	rpcinfo.AsMutableRPCConfig(config).SetPayloadCodec(serviceinfo.Thrift)
	info := rpcinfo.NewRPCInfo(rpcinfo.NewEndpointInfo("caller", "Test", nil, nil), rpcinfo.NewEndpointInfo("server", "Test", nil, nil), rpcinfo.NewInvocation("Mock", "Test"), config, nil)
	message := remote.NewMessage(data, info, kind, remote.Client)
	message.SetPayloadCodec(thriftcodec.NewThriftCodec())
	return message
}

func TestNativeTxCapacityMatchesDefaultCodec(t *testing.T) {
	for _, protocol := range []transport.Protocol{transport.Framed, transport.TTHeader, transport.TTHeaderFramed} {
		for _, primary := range []bool{false, true} {
			for _, size := range []int{1024, 16384, 131 << 10} {
				t.Run(fmt.Sprintf("protocol%d-primary%v-size%d", protocol, primary, size), func(t *testing.T) {
					payload := &countedNativePayload{Request: &kt.Request{Message: strings.Repeat("x", size)}}
					message := nativeCapacityMessage(payload, protocol, remote.Call)
					defer remote.RecycleMessage(message)
					headers := remote.AccessHeaders(message.TransInfo())
					if primary && !headers.EnablePrimary(false) {
						t.Fatal("primary metadata unavailable")
					}
					headers.SetInt(7, "old")
					headers.SetInt(7, "current")
					headers.SetString(ttheader.GDPRToken, "token-value")
					headers.SetString("meta", "padding-value")
					headers.SetString("deleted", "obsolete")
					headers.DeleteString("deleted")
					snapshot := func() (map[uint16]string, map[string]string) {
						ints, strs := make(map[uint16]string), make(map[string]string)
						headers.RangeInt(func(key uint16, value string) bool { ints[key] = value; return true })
						headers.RangeString(func(key, value string) bool { strs[key] = value; return true })
						return ints, strs
					}
					beforeInts, beforeStrings := snapshot()
					beforePrimary := headers.Primary()
					source := new(capacitySource)
					tx, err := remote.AcquireNativeTx(context.Background(), message, source)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Release()
					afterInts, afterStrings := snapshot()
					if !reflect.DeepEqual(beforeInts, afterInts) || !reflect.DeepEqual(beforeStrings, afterStrings) || beforePrimary != headers.Primary() {
						t.Fatal("capacity sizing changed transport headers")
					}
					if payload.sizes != 1 || payload.writes != 0 || source.calls != 1 {
						t.Fatalf("sizing encoded payload or acquired twice: sizes=%d writes=%d calls=%d", payload.sizes, payload.writes, source.calls)
					}
					out := remote.NewWriterBuffer(size + 4096)
					defer out.Release(nil)
					if err := codec.NewDefaultCodec().Encode(context.Background(), message, out); err != nil {
						t.Fatal(err)
					}
					if source.capacity != out.WrittenLen() || len(tx.Bytes()) != out.WrittenLen() || payload.sizes != 2 || payload.writes != 1 {
						t.Fatalf("native capacity=%d actual wire=%d sizes=%d writes=%d", source.capacity, out.WrittenLen(), payload.sizes, payload.writes)
					}
					if protocol&transport.TTHeader != 0 {
						wire, err := out.Bytes()
						if err != nil {
							t.Fatal(err)
						}
						decoded, err := ttheader.Decode(context.Background(), bufiox.NewBytesReader(wire))
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(beforeInts, decoded.IntInfo) || !reflect.DeepEqual(beforeStrings, decoded.StrInfo) {
							t.Fatal("sized TX encoded different transport headers")
						}
					}
				})
			}
		}
	}
}

func TestNativeTxCapacityRejectsBeforeAllocation(t *testing.T) {
	maximum := int(^uint(0) >> 1)
	for _, test := range []struct {
		name     string
		length   int
		protocol transport.Protocol
	}{
		{"negative", -1, transport.TTHeaderFramed},
		{"payload-overflow", maximum, transport.Framed},
		{"framed-overflow", maximum - thrift.Binary.MessageBeginLength("Test"), transport.Framed},
		{"header-overflow", maximum - thrift.Binary.MessageBeginLength("Test"), transport.TTHeader},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := &countedNativePayload{Request: new(kt.Request), length: &test.length}
			message := nativeCapacityMessage(payload, test.protocol, remote.Call)
			defer remote.RecycleMessage(message)
			source := new(capacitySource)
			if buffer, err := remote.AcquireNativeTx(context.Background(), message, source); err == nil || buffer != nil || source.calls != 0 || payload.writes != 0 {
				t.Fatalf("invalid capacity reached native allocation: buffer=%v error=%v calls=%d writes=%d", buffer, err, source.calls, payload.writes)
			}
		})
	}
}

func TestNativeTxCapacityCancellationDuringSizing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	payload := &countedNativePayload{Request: new(kt.Request), cancel: cancel}
	message := nativeCapacityMessage(payload, transport.TTHeaderFramed, remote.Call)
	defer remote.RecycleMessage(message)
	source := new(capacitySource)
	if buffer, err := remote.AcquireNativeTx(ctx, message, source); !errors.Is(err, context.Canceled) || buffer != nil || source.calls != 0 || payload.writes != 0 {
		t.Fatalf("canceled sizing acquired TX: buffer=%v error=%v calls=%d writes=%d", buffer, err, source.calls, payload.writes)
	}
}

func TestNativeTxCapacityMatchesExceptionCodec(t *testing.T) {
	for _, data := range []error{remote.NewTransErrorWithMsg(remote.InternalError, "native exception"), errors.New("business exception")} {
		message := nativeCapacityMessage(data, transport.TTHeaderFramed, remote.Exception)
		headers := remote.AccessHeaders(message.TransInfo())
		headers.EnablePrimary(true)
		headers.SetString(ttheader.GDPRToken, "response-token")
		source := new(capacitySource)
		tx, err := remote.AcquireNativeTx(context.Background(), message, source)
		if err != nil {
			t.Fatal(err)
		}
		out := remote.NewWriterBuffer(4096)
		if err := codec.NewDefaultCodec().Encode(context.Background(), message, out); err != nil {
			t.Fatal(err)
		}
		if source.capacity != out.WrittenLen() {
			t.Fatalf("exception capacity=%d actual wire=%d", source.capacity, out.WrittenLen())
		}
		tx.Release()
		out.Release(nil)
		remote.RecycleMessage(message)
	}
}

func TestNativeTxCapacityRejectsInvalidHeader(t *testing.T) {
	for _, primary := range []bool{false, true} {
		payload := &countedNativePayload{Request: new(kt.Request)}
		message := nativeCapacityMessage(payload, transport.TTHeaderFramed, remote.Call)
		headers := remote.AccessHeaders(message.TransInfo())
		if primary {
			headers.EnablePrimary(false)
		}
		headers.SetString("oversized", strings.Repeat("x", 1<<16))
		source := new(capacitySource)
		buffer, err := remote.AcquireNativeTx(context.Background(), message, source)
		if err == nil || buffer != nil || source.calls != 0 || payload.writes != 0 {
			t.Fatalf("invalid header reached native allocation: primary=%v buffer=%v error=%v calls=%d writes=%d", primary, buffer, err, source.calls, payload.writes)
		}
		out := remote.NewWriterBuffer(1 << 17)
		if err := codec.NewDefaultCodec().Encode(context.Background(), message, out); err == nil {
			t.Fatal("reference codec accepted the invalid header")
		}
		out.Release(nil)
		remote.RecycleMessage(message)
	}
}

type legacyCapacitySource struct{ calls int }

func (s *legacyCapacitySource) AcquireTx(context.Context) (remote.NativeBuffer, error) {
	s.calls++
	return &capacityBuffer{data: make([]byte, 65536)}, nil
}

func TestNativeTxCapacityKeepsLegacyAcquire(t *testing.T) {
	payload := &countedNativePayload{Request: new(kt.Request)}
	message := nativeCapacityMessage(payload, transport.TTHeaderFramed, remote.Call)
	defer remote.RecycleMessage(message)
	source := new(legacyCapacitySource)
	buffer, err := remote.AcquireNativeTx(context.Background(), message, source)
	if err != nil {
		t.Fatal(err)
	}
	defer buffer.Release()
	if source.calls != 1 || len(buffer.Bytes()) != 65536 || payload.sizes != 0 || payload.writes != 0 {
		t.Fatalf("legacy Acquire changed: calls=%d bytes=%d sizes=%d writes=%d", source.calls, len(buffer.Bytes()), payload.sizes, payload.writes)
	}
	out := remote.NewWriterBuffer(4096)
	defer out.Release(nil)
	if err := codec.NewDefaultCodec().Encode(context.Background(), message, out); err != nil {
		t.Fatal(err)
	}
	if payload.sizes != 1 || payload.writes != 1 {
		t.Fatalf("legacy Encode changed: sizes=%d writes=%d", payload.sizes, payload.writes)
	}
}

type benchmarkNativeSource struct{ buffer capacityBuffer }

func (s *benchmarkNativeSource) AcquireTx(context.Context) (remote.NativeBuffer, error) {
	return &s.buffer, nil
}

type benchmarkSizedNativeSource struct{ benchmarkNativeSource }

func (s *benchmarkSizedNativeSource) AcquireTxSized(context.Context, int) (remote.NativeBuffer, error) {
	return &s.buffer, nil
}

func BenchmarkNativeTxCapacity(b *testing.B) {
	for _, size := range []int{1024, 16384, 131 << 10} {
		for _, sized := range []bool{false, true} {
			b.Run(fmt.Sprintf("size%d-sized%v", size, sized), func(b *testing.B) {
				payload := &countedNativePayload{Request: &kt.Request{Message: strings.Repeat("x", size)}}
				message := nativeCapacityMessage(payload, transport.TTHeaderFramed, remote.Call)
				defer remote.RecycleMessage(message)
				headers := remote.AccessHeaders(message.TransInfo())
				headers.EnablePrimary(false)
				headers.SetString("meta", "fixed-benchmark-header")
				var source interface {
					AcquireTx(context.Context) (remote.NativeBuffer, error)
				} = &benchmarkNativeSource{buffer: capacityBuffer{data: make([]byte, 256<<10)}}
				if sized {
					source = &benchmarkSizedNativeSource{benchmarkNativeSource: benchmarkNativeSource{buffer: capacityBuffer{data: make([]byte, 256<<10)}}}
				}
				b.ReportAllocs()
				b.ResetTimer()
				for index := 0; index < b.N; index++ {
					if _, err := remote.AcquireNativeTx(context.Background(), message, source); err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				if payload.writes != 0 {
					b.Fatal("capacity benchmark encoded payload")
				}
				b.ReportMetric(float64(payload.sizes)/float64(b.N), "BLength/op")
			})
		}
	}
}
