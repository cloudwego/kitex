package thrift

import (
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
	gthrift "github.com/cloudwego/gopkg/protocol/thrift"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
)

var _ bufiox.Writer = (*countingWriter)(nil)
var _ bufiox.Writer = (*countingNocopyWriter)(nil)
var _ remote.NocopyWrite = (*countingNocopyWriter)(nil)

type countingWriter struct {
	buf []byte
}

func (w *countingWriter) Malloc(n int) ([]byte, error) {
	start := len(w.buf)
	w.buf = append(w.buf, make([]byte, n)...)
	return w.buf[start : start+n], nil
}

func (w *countingWriter) WriteBinary(bs []byte) (int, error) {
	w.buf = append(w.buf, bs...)
	return len(bs), nil
}

func (w *countingWriter) WrittenLen() int {
	return len(w.buf)
}

func (w *countingWriter) Flush() error {
	return nil
}

type countingNocopyWriter struct {
	countingWriter
	mallocAckCalls int
	writeDirects   int
}

func (w *countingNocopyWriter) WriteDirect(_ []byte, _ int) error {
	w.writeDirects++
	return nil
}

func (w *countingNocopyWriter) MallocAck(_ int) error {
	w.mallocAckCalls++
	return nil
}

type mockAdaptiveCodec struct {
	value          string
	info           adaptiveNocopyInfo
	legacyCalls    int
	streamingCalls int
}

func (m *mockAdaptiveCodec) BLength() int {
	return gthrift.Binary.FieldBeginLength() + gthrift.Binary.StringLength(m.value) + gthrift.Binary.FieldStopLength()
}

func (m *mockAdaptiveCodec) FastWriteNocopy(buf []byte, bw gthrift.NocopyWriter) int {
	m.legacyCalls++
	off := 0
	off += gthrift.Binary.WriteFieldBegin(buf[off:], gthrift.STRING, 1)
	off += gthrift.Binary.WriteStringNocopy(buf[off:], bw, m.value)
	off += gthrift.Binary.WriteFieldStop(buf[off:])
	return off
}

func (m *mockAdaptiveCodec) FastWriteTo(out bufiox.Writer) error {
	m.streamingCalls++
	buf, err := out.Malloc(gthrift.Binary.FieldBeginLength())
	if err != nil {
		return err
	}
	gthrift.Binary.WriteFieldBegin(buf, gthrift.STRING, 1)
	buf, err = out.Malloc(gthrift.Binary.StringLength(m.value))
	if err != nil {
		return err
	}
	gthrift.Binary.WriteString(buf, m.value)
	buf, err = out.Malloc(gthrift.Binary.FieldStopLength())
	if err != nil {
		return err
	}
	gthrift.Binary.WriteFieldStop(buf)
	return nil
}

func (m *mockAdaptiveCodec) FastNocopyInfo() (int, int, int) {
	return m.info.Length, m.info.DirectCount, m.info.DirectBytes
}

func (m *mockAdaptiveCodec) FastRead(buf []byte) (int, error) {
	return len(buf), nil
}

type mockLegacyCodec struct {
	value       string
	legacyCalls int
}

func (m *mockLegacyCodec) BLength() int {
	return gthrift.Binary.FieldBeginLength() + gthrift.Binary.StringLength(m.value) + gthrift.Binary.FieldStopLength()
}

func (m *mockLegacyCodec) FastWriteNocopy(buf []byte, bw gthrift.NocopyWriter) int {
	m.legacyCalls++
	off := 0
	off += gthrift.Binary.WriteFieldBegin(buf[off:], gthrift.STRING, 1)
	off += gthrift.Binary.WriteStringNocopy(buf[off:], bw, m.value)
	off += gthrift.Binary.WriteFieldStop(buf[off:])
	return off
}

func (m *mockLegacyCodec) FastRead(buf []byte) (int, error) {
	return len(buf), nil
}

func TestFastMarshalAdaptiveUsesStreamingForNocopyWriter(t *testing.T) {
	value := strings.Repeat("x", 32*1024)
	codec := &mockAdaptiveCodec{
		value: value,
		info: adaptiveNocopyInfo{
			Length:      gthrift.Binary.FieldBeginLength() + gthrift.Binary.StringLength(value) + gthrift.Binary.FieldStopLength(),
			DirectCount: adaptiveFastMarshalMinDirectCount,
			DirectBytes: adaptiveFastMarshalMinDirectBytes,
		},
	}
	out := &countingNocopyWriter{}

	err := fastMarshal(out, "mock", remote.Call, 1, codec)
	test.Assert(t, err == nil, err)
	test.Assert(t, codec.streamingCalls == 1)
	test.Assert(t, codec.legacyCalls == 0)
	test.Assert(t, out.mallocAckCalls == 0)
}

func TestFastMarshalAdaptiveFallsBackToLegacyForSmallInfo(t *testing.T) {
	codec := &mockAdaptiveCodec{
		value: "small",
		info: adaptiveNocopyInfo{
			Length:      gthrift.Binary.FieldBeginLength() + gthrift.Binary.StringLength("small") + gthrift.Binary.FieldStopLength(),
			DirectCount: 0,
			DirectBytes: 0,
		},
	}
	out := &countingNocopyWriter{}

	err := fastMarshal(out, "mock", remote.Call, 1, codec)
	test.Assert(t, err == nil, err)
	test.Assert(t, codec.legacyCalls == 1)
	test.Assert(t, codec.streamingCalls == 0)
	test.Assert(t, out.mallocAckCalls == 1)
}

func TestFastMarshalAdaptiveKeepsLegacyForPlainWriter(t *testing.T) {
	value := strings.Repeat("x", 32*1024)
	codec := &mockAdaptiveCodec{
		value: value,
		info: adaptiveNocopyInfo{
			Length:      gthrift.Binary.FieldBeginLength() + gthrift.Binary.StringLength(value) + gthrift.Binary.FieldStopLength(),
			DirectCount: adaptiveFastMarshalMinDirectCount,
			DirectBytes: adaptiveFastMarshalMinDirectBytes,
		},
	}
	out := &countingWriter{}

	err := fastMarshal(out, "mock", remote.Call, 1, codec)
	test.Assert(t, err == nil, err)
	test.Assert(t, codec.legacyCalls == 1)
	test.Assert(t, codec.streamingCalls == 0)
}

func TestFastMarshalAdaptiveKeepsLegacyForNonStreamingCodec(t *testing.T) {
	codec := &mockLegacyCodec{value: "small"}
	out := &countingNocopyWriter{}

	err := fastMarshal(out, "mock", remote.Call, 1, codec)
	test.Assert(t, err == nil, err)
	test.Assert(t, codec.legacyCalls == 1)
	test.Assert(t, out.mallocAckCalls == 1)
}
