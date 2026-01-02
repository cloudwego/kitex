/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package codec

import (
	"context"
	"encoding/binary"
	"errors"
	"math/bits"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// Mock marshaler (gogoproto style) for testing
type mockMarshaler struct {
	data string
	fail bool
}

func (m *mockMarshaler) Size() int {
	return len(m.data)
}

func (m *mockMarshaler) MarshalTo(data []byte) (int, error) {
	if m.fail {
		return 0, errors.New("marshal failed")
	}
	copy(data, m.data)
	return len(m.data), nil
}

// Mock protobufV2MsgCodec for testing
type mockProtobufV2Msg struct {
	data string
	fail bool
}

func (m *mockProtobufV2Msg) XXX_Unmarshal(b []byte) error {
	if m.fail {
		return errors.New("unmarshal failed")
	}
	m.data = string(b)
	return nil
}

func (m *mockProtobufV2Msg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if m.fail {
		return nil, errors.New("marshal failed")
	}
	return []byte(m.data), nil
}

// Mock MessageWriterWithContext for generic protobuf testing
type mockMessageWriter struct {
	data   []byte
	fail   bool
	method string
}

func (m *mockMessageWriter) WritePb(ctx context.Context, method string) (interface{}, error) {
	m.method = method
	if m.fail {
		return nil, errors.New("write failed")
	}
	return m.data, nil
}

// Mock MessageReaderWithMethodWithContext for generic protobuf testing
type mockMessageReader struct {
	data   []byte
	fail   bool
	method string
}

func (m *mockMessageReader) ReadPb(ctx context.Context, method string, payload []byte) error {
	m.method = method
	if m.fail {
		return errors.New("read failed")
	}
	m.data = payload
	return nil
}

// sizeVarint returns the encoded size of a varint.
func sizeVarint(v uint64) int {
	return int(9*uint32(bits.Len64(v))+64) / 64
}

// Mock fastpb.Writer (deprecated but still needs testing)
type mockFastWriter struct {
	num int32
	v   string
}

func (p *mockFastWriter) Size() int {
	n := sizeVarint(uint64(p.num)<<3 | uint64(2))
	n += sizeVarint(uint64(len(p.v)))
	n += len(p.v)
	return n
}

func (p *mockFastWriter) FastWrite(in []byte) int {
	n := binary.PutUvarint(in, uint64(p.num)<<3|uint64(2))
	n += binary.PutUvarint(in[n:], uint64(len(p.v)))
	n += copy(in[n:], p.v)
	return n
}

// Mock fastpb.Reader
type mockFastReader struct {
	num int32
	v   string
}

func (p *mockFastReader) FastRead(buf []byte, t int8, number int32) (int, error) {
	if t != 2 {
		return 0, errors.New("invalid type")
	}
	p.num = number
	sz, n := binary.Uvarint(buf)
	buf = buf[n:]
	p.v = string(buf[:sz])
	return int(sz) + n, nil
}

func newMockRPCInfo(method string) rpcinfo.RPCInfo {
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.NewEndpointInfo("", method, nil, nil),
		rpcinfo.NewEndpointInfo("", method, nil, nil),
		rpcinfo.NewInvocation("", method),
		rpcinfo.NewRPCConfig(),
		rpcinfo.NewRPCStats(),
	)
	return ri
}

func TestGRPCEncodeProtobufStruct(t *testing.T) {
	t.Run("fastpb.Writer", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		msg := &mockFastWriter{
			num: 100,
			v:   "fastpb test",
		}

		// Test without compression (with GRPC header space)
		res, err := GRPCEncodeProtobufStruct(ctx, ri, msg, false)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil, res)
		test.Assert(t, len(res.Payload) == GRPCDataFrameHeaderLen+msg.Size(), res)
		test.Assert(t, res.PreAllocate, res)
		test.Assert(t, res.PreAllocateSize == msg.Size(), res)

		// Test with compression (no header space)
		res2, err := GRPCEncodeProtobufStruct(ctx, ri, msg, true)
		test.Assert(t, err == nil, err)
		test.Assert(t, res2.Payload != nil, res2)
		test.Assert(t, len(res2.Payload) == msg.Size(), res2)
		test.Assert(t, res2.PreAllocate, res2)
		test.Assert(t, res2.PreAllocateSize == msg.Size(), res2)
	})
	t.Run("marshaler", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		msg := &mockMarshaler{data: "test data"}

		// Test without compression
		res, err := GRPCEncodeProtobufStruct(ctx, ri, msg, false)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil)
		test.Assert(t, len(res.Payload) >= GRPCDataFrameHeaderLen, res)
		test.Assert(t, res.PreAllocate)
		test.Assert(t, res.PreAllocateSize == len(msg.data))
		test.Assert(t, string(res.Payload[GRPCDataFrameHeaderLen:]) == msg.data)

		// Test with compression
		res2, err := GRPCEncodeProtobufStruct(ctx, ri, msg, true)
		test.Assert(t, err == nil, err)
		test.Assert(t, res2.Payload != nil)
		test.Assert(t, len(res2.Payload) == len(msg.data), res2)
		test.Assert(t, res2.PreAllocate)
		test.Assert(t, res2.PreAllocateSize == len(msg.data))
		test.Assert(t, string(res2.Payload) == msg.data)

		// Test marshal failure
		msgFail := &mockMarshaler{data: "test", fail: true}
		resFail, errFail := GRPCEncodeProtobufStruct(ctx, ri, msgFail, false)
		test.Assert(t, errFail != nil)
		test.Assert(t, resFail.Payload == nil)
	})
	t.Run("protobufV2MsgCodec", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		msg := &mockProtobufV2Msg{data: "protobuf v2 data"}

		// Test without compression
		res, err := GRPCEncodeProtobufStruct(ctx, ri, msg, false)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil)
		test.Assert(t, string(res.Payload) == msg.data)
		test.Assert(t, !res.PreAllocate)

		// Test with compression
		res2, err := GRPCEncodeProtobufStruct(ctx, ri, msg, true)
		test.Assert(t, err == nil, err)
		test.Assert(t, res2.Payload != nil)
		test.Assert(t, string(res2.Payload) == msg.data)
		test.Assert(t, !res2.PreAllocate)

		// Test encode failure
		msgFail := &mockProtobufV2Msg{data: "test", fail: true}
		resFail, errFail := GRPCEncodeProtobufStruct(ctx, ri, msgFail, false)
		test.Assert(t, errFail != nil)
		test.Assert(t, resFail.Payload == nil)
	})
	t.Run("proto.Message", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		req := &protobuf.MockReq{
			Msg: "test message",
			StrMap: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			StrList: []string{"item1", "item2"},
		}

		// Test without compression (with GRPC header space)
		res, err := GRPCEncodeProtobufStruct(ctx, ri, req, false)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil)
		test.Assert(t, len(res.Payload) >= GRPCDataFrameHeaderLen)
		test.Assert(t, !res.PreAllocate)

		// Test with compression (no header space)
		res2, err := GRPCEncodeProtobufStruct(ctx, ri, req, true)
		test.Assert(t, err == nil, err)
		test.Assert(t, res2.Payload != nil)
		test.Assert(t, !res2.PreAllocate)

		// Decode and verify
		newReq := &protobuf.MockReq{}
		err = DecodeProtobufStruct(ctx, ri, res.Payload, newReq)
		test.Assert(t, err == nil, err)
		test.Assert(t, newReq.Msg == req.Msg)
		test.DeepEqual(t, newReq.StrMap, req.StrMap)
		test.DeepEqual(t, newReq.StrList, req.StrList)
	})
	t.Run("generic", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		data := []byte("generic data")
		msg := &mockMessageWriter{data: data}

		// Test without compression
		res, err := GRPCEncodeProtobufStruct(ctx, ri, msg, false)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil)
		test.DeepEqual(t, res.Payload, data)
		test.Assert(t, msg.method == "TestMethod")

		// Test with compression
		res2, err := GRPCEncodeProtobufStruct(ctx, ri, msg, true)
		test.Assert(t, err == nil, err)
		test.Assert(t, res2.Payload != nil)
		test.DeepEqual(t, res2.Payload, data)

		// Test with empty method name
		riEmpty := newMockRPCInfo("")
		_, errEmpty := GRPCEncodeProtobufStruct(ctx, riEmpty, msg, false)
		test.Assert(t, errEmpty == errEncodePbGenericEmptyMethod, errEmpty)

		// Test write failure
		msgFail := &mockMessageWriter{data: data, fail: true}
		_, errFail := GRPCEncodeProtobufStruct(ctx, ri, msgFail, false)
		test.Assert(t, errFail != nil)
		test.Assert(t, errFail.Error() == "write failed", errFail)
	})
	t.Run("invalid payload", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		_, err := GRPCEncodeProtobufStruct(ctx, ri, 123, false)
		test.Assert(t, err != nil)
		test.Assert(t, err.Error() == "invalid payload int in EncodeProtobufStruct", err)
	})
}

func TestTTStreamEncodeProtobufStruct(t *testing.T) {
	t.Run("fastpb.Writer", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		msg := &mockFastWriter{
			num: 100,
			v:   "fastpb test",
		}

		res, err := TTStreamEncodeProtobufStruct(ctx, ri, msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil, res)
		test.Assert(t, len(res.Payload) == msg.Size(), res)
		test.Assert(t, res.PreAllocate, res)
		test.Assert(t, res.PreAllocateSize == msg.Size(), res)
	})
	t.Run("marshaler", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		msg := &mockMarshaler{data: "ttstream data"}

		res, err := TTStreamEncodeProtobufStruct(ctx, ri, msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil)
		test.Assert(t, len(res.Payload) == len(msg.data), res)
		test.Assert(t, res.PreAllocate)
		test.Assert(t, res.PreAllocateSize == len(msg.data))
		test.Assert(t, string(res.Payload) == msg.data)

		// Test marshal failure
		msgFail := &mockMarshaler{data: "test", fail: true}
		resFail, errFail := TTStreamEncodeProtobufStruct(ctx, ri, msgFail)
		test.Assert(t, errFail != nil)
		test.Assert(t, resFail.Payload == nil)
	})
	t.Run("protobufV2MsgCodec", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		msg := &mockProtobufV2Msg{data: "protobuf v2 data"}

		res, err := TTStreamEncodeProtobufStruct(ctx, ri, msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil)
		test.Assert(t, string(res.Payload) == msg.data)
		test.Assert(t, !res.PreAllocate)

		// Test encode failure
		msgFail := &mockProtobufV2Msg{data: "test", fail: true}
		resFail, errFail := TTStreamEncodeProtobufStruct(ctx, ri, msgFail)
		test.Assert(t, errFail != nil)
		test.Assert(t, resFail.Payload == nil)
	})
	t.Run("proto.Message", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		req := &protobuf.MockReq{
			Msg: "ttstream message",
			StrMap: map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
			StrList: []string{"s1", "s2"},
		}

		res, err := TTStreamEncodeProtobufStruct(ctx, ri, req)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil)
		test.Assert(t, !res.PreAllocate)

		// Decode and verify
		newReq := &protobuf.MockReq{}
		err = DecodeProtobufStruct(ctx, ri, res.Payload, newReq)
		test.Assert(t, err == nil, err)
		test.Assert(t, newReq.Msg == req.Msg)
		test.DeepEqual(t, newReq.StrMap, req.StrMap)
		test.DeepEqual(t, newReq.StrList, req.StrList)
	})
	t.Run("generic", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		data := []byte("generic ttstream data")
		msg := &mockMessageWriter{data: data}

		res, err := TTStreamEncodeProtobufStruct(ctx, ri, msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, res.Payload != nil)
		test.DeepEqual(t, res.Payload, data)
		test.Assert(t, msg.method == "TestMethod")

		// Test with empty method name
		riEmpty := newMockRPCInfo("")
		_, errEmpty := TTStreamEncodeProtobufStruct(ctx, riEmpty, msg)
		test.Assert(t, errEmpty == errEncodePbGenericEmptyMethod, errEmpty)

		// Test write failure
		msgFail := &mockMessageWriter{data: data, fail: true}
		_, errFail := TTStreamEncodeProtobufStruct(ctx, ri, msgFail)
		test.Assert(t, errFail != nil)
		test.Assert(t, errFail.Error() == "write failed", errFail)
	})
	t.Run("invalid payload", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		_, err := TTStreamEncodeProtobufStruct(ctx, ri, "invalid string")
		test.Assert(t, err != nil)
		test.Assert(t, err.Error() == "invalid payload string in EncodeProtobufStruct", err)
	})
}

func TestDecodeProtobufStruct(t *testing.T) {
	t.Run("fastpb.Reader", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		// Test with empty payload (should skip fastpb and use proto.Unmarshal fallback)
		msg := &mockFastReader{}
		err := DecodeProtobufStruct(ctx, ri, []byte{}, msg)
		// This should not use FastRead because of empty payload
		test.Assert(t, err != nil) // Will fail since mockFastReader doesn't implement proto.Message

		// Test with non-empty payload
		buf := make([]byte, 20)
		n := binary.PutUvarint(buf, uint64(5)<<3|uint64(2))
		n += binary.PutUvarint(buf[n:], uint64(4))
		n += copy(buf[n:], "test")

		msg2 := &mockFastReader{}
		err = DecodeProtobufStruct(ctx, ri, buf[:n], msg2)
		test.Assert(t, err == nil, err)
		test.Assert(t, msg2.num == 5)
		test.Assert(t, msg2.v == "test")
	})
	t.Run("protobufV2MsgCodec", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		payload := []byte("v2 payload")
		msg := &mockProtobufV2Msg{}

		err := DecodeProtobufStruct(ctx, ri, payload, msg)
		test.Assert(t, err == nil, err)
		test.Assert(t, msg.data == string(payload))

		// Test decode failure
		msgFail := &mockProtobufV2Msg{fail: true}
		err = DecodeProtobufStruct(ctx, ri, payload, msgFail)
		test.Assert(t, err != nil)
		test.Assert(t, err.Error() == "unmarshal failed", err)
	})
	t.Run("proto.Message", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		req := &protobuf.MockReq{
			Msg:     "decode test",
			StrMap:  map[string]string{"k": "v"},
			StrList: []string{"a", "b"},
		}

		payload, err := proto.Marshal(req)
		test.Assert(t, err == nil, err)

		newReq := &protobuf.MockReq{}
		err = DecodeProtobufStruct(ctx, ri, payload, newReq)
		test.Assert(t, err == nil, err)
		test.Assert(t, newReq.Msg == req.Msg)
		test.DeepEqual(t, newReq.StrMap, req.StrMap)
		test.DeepEqual(t, newReq.StrList, req.StrList)
	})
	t.Run("generic", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		payload := []byte("generic payload")
		msg := &mockMessageReader{}

		err := DecodeProtobufStruct(ctx, ri, payload, msg)
		test.Assert(t, err == nil, err)
		test.DeepEqual(t, msg.data, payload)
		test.Assert(t, msg.method == "TestMethod")

		// Test with empty method name
		riEmpty := newMockRPCInfo("")
		err = DecodeProtobufStruct(ctx, riEmpty, payload, msg)
		test.Assert(t, err == errDecodePbGenericEmptyMethod, err)

		// Test read failure
		msgFail := &mockMessageReader{fail: true}
		err = DecodeProtobufStruct(ctx, ri, payload, msgFail)
		test.Assert(t, err != nil)
		test.Assert(t, err.Error() == "read failed", err)
	})
	t.Run("invalid payload", func(t *testing.T) {
		ctx := context.Background()
		ri := newMockRPCInfo("TestMethod")

		payload := []byte("test")

		// Test with invalid payload type
		err := DecodeProtobufStruct(ctx, ri, payload, "invalid string")
		test.Assert(t, err != nil)
		test.Assert(t, err.Error() == "invalid payload string in DecodeProtobufStruct", err)

		err = DecodeProtobufStruct(ctx, ri, payload, 123)
		test.Assert(t, err != nil)
		test.Assert(t, err.Error() == "invalid payload int in DecodeProtobufStruct", err)
	})
}

func Test_EncodeDecodeRoundTrip(t *testing.T) {
	ctx := context.Background()
	ri := newMockRPCInfo("RoundTripMethod")

	testCases := []struct {
		name string
		msg  *protobuf.MockReq
	}{
		{
			name: "simple message",
			msg: &protobuf.MockReq{
				Msg: "simple",
			},
		},
		{
			name: "message with map",
			msg: &protobuf.MockReq{
				Msg:    "with map",
				StrMap: map[string]string{"a": "1", "b": "2", "c": "3"},
			},
		},
		{
			name: "message with list",
			msg: &protobuf.MockReq{
				Msg:     "with list",
				StrList: []string{"x", "y", "z"},
			},
		},
		{
			name: "complete message",
			msg: &protobuf.MockReq{
				Msg:     "complete",
				StrMap:  map[string]string{"key": "value"},
				StrList: []string{"item"},
			},
		},
		{
			name: "empty message",
			msg:  &protobuf.MockReq{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test GRPC encode/decode
			res, err := GRPCEncodeProtobufStruct(ctx, ri, tc.msg, false)
			test.Assert(t, err == nil, err)

			decoded := &protobuf.MockReq{}
			err = DecodeProtobufStruct(ctx, ri, res.Payload, decoded)
			test.Assert(t, err == nil, err)
			test.DeepEqual(t, decoded.Msg, tc.msg.Msg)
			test.DeepEqual(t, decoded.StrMap, tc.msg.StrMap)
			test.DeepEqual(t, decoded.StrList, tc.msg.StrList)

			// Test TTStream encode/decode
			res2, err := TTStreamEncodeProtobufStruct(ctx, ri, tc.msg)
			test.Assert(t, err == nil, err)

			decoded2 := &protobuf.MockReq{}
			err = DecodeProtobufStruct(ctx, ri, res2.Payload, decoded2)
			test.Assert(t, err == nil, err)
			test.DeepEqual(t, decoded2.Msg, tc.msg.Msg)
			test.DeepEqual(t, decoded2.StrMap, tc.msg.StrMap)
			test.DeepEqual(t, decoded2.StrList, tc.msg.StrList)
		})
	}
}
