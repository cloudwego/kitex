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

package ttstream

import (
	"context"
	"encoding/binary"
	"math/bits"
	"testing"

	"github.com/cloudwego/gopkg/protocol/ttheader"

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
		return 0, errInvalidMessage
	}
	copy(data, m.data)
	return len(m.data), nil
}

type mockFastCodecMsg struct {
	num int32
	v   string
}

// sizeVarint returns the encoded size of a varint.
// The size is guaranteed to be within 1 and 10, inclusive.
func sizeVarint(v uint64) int {
	// This computes 1 + (bits.Len64(v)-1)/7.
	// 9/64 is a good enough approximation of 1/7
	return int(9*uint32(bits.Len64(v))+64) / 64
}

func (p *mockFastCodecMsg) Size() (n int) {
	n += sizeVarint(uint64(p.num)<<3 | uint64(2))
	n += sizeVarint(uint64(len(p.v)))
	n += len(p.v)
	return n
}

func (p *mockFastCodecMsg) FastWrite(in []byte) (n int) {
	n += binary.PutUvarint(in, uint64(p.num)<<3|uint64(2)) // Tag
	n += binary.PutUvarint(in[n:], uint64(len(p.v)))       // varint len of string
	n += copy(in[n:], p.v)

	return n
}

func (p *mockFastCodecMsg) FastRead(buf []byte, t int8, number int32) (int, error) {
	if t != 2 {
		panic(t)
	}
	p.num = number
	sz, n := binary.Uvarint(buf)
	buf = buf[n:]
	p.v = string(buf[:sz])
	return int(sz) + n, nil
}

func (p *mockFastCodecMsg) Marshal(_ []byte) ([]byte, error) { panic("not in use") }

func (p *mockFastCodecMsg) Unmarshal(_ []byte) error { panic("not in use") }

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

func Test_getCodec(t *testing.T) {
	tests := []struct {
		desc       string
		protocolID ttheader.ProtocolID
		expectType string
	}{
		{
			desc:       "thrift codec",
			protocolID: ttheader.ProtocolIDThriftStruct,
			expectType: "thriftCodec",
		},
		{
			desc:       "protobuf codec",
			protocolID: ttheader.ProtocolIDProtobufStruct,
			expectType: "protobufCodec",
		},
		{
			desc:       "unknown defaults to thrift",
			protocolID: 99,
			expectType: "thriftCodec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			c := getCodec(tt.protocolID)
			test.Assert(t, c != nil)
			// Check type by encoding behavior
			switch tt.expectType {
			case "thriftCodec":
				_, ok := c.(thriftCodec)
				test.Assert(t, ok, c)
			case "protobufCodec":
				_, ok := c.(protobufCodec)
				test.Assert(t, ok, c)
			}
		})
	}
}

func Test_thriftCodec_FastCodec(t *testing.T) {
	c := thriftCodec{}
	ctx := context.Background()
	ri := newMockRPCInfo("TestMethod")

	req := &testRequest{
		A: 123,
		B: "test",
	}

	payload, needRecycle, err := c.encode(ctx, ri, req)
	test.Assert(t, err == nil, err)
	test.Assert(t, payload != nil)
	test.Assert(t, !needRecycle)

	newReq := &testRequest{}
	err = c.decode(ctx, ri, payload, newReq)
	test.Assert(t, err == nil, err)
	test.Assert(t, newReq.A == req.A)
	test.Assert(t, newReq.B == req.B)
}

func Test_thriftCodec_InvalidMessage(t *testing.T) {
	c := thriftCodec{}
	ctx := context.Background()
	ri := newMockRPCInfo("TestMethod")

	_, _, err := c.encode(ctx, ri, "invalid")
	test.Assert(t, err.Error() == "invalid payload type string in ttstream thrift Encode", err)

	err = c.decode(ctx, ri, []byte("data"), "invalid")
	test.Assert(t, err.Error() == "invalid payload type string in ttstream thrift Decode", err)
}

func Test_protobufCodec_ProtoMessage(t *testing.T) {
	c := protobufCodec{}
	ctx := context.Background()
	ri := newMockRPCInfo("TestMethod")

	req := &protobuf.MockReq{
		Msg: "ProtoMessage",
		StrMap: map[string]string{
			"TestKey": "TestVal",
		},
		StrList: []string{
			"String",
		},
	}

	payload, needRecycle, err := c.encode(ctx, ri, req)
	test.Assert(t, err == nil, err)
	test.Assert(t, payload != nil)
	test.Assert(t, !needRecycle)

	newReq := &protobuf.MockReq{}
	err = c.decode(ctx, ri, payload, newReq)
	test.Assert(t, err == nil, err)
	test.Assert(t, newReq.Msg == req.Msg)
	test.DeepEqual(t, newReq.StrMap, req.StrMap)
	test.DeepEqual(t, newReq.StrList, req.StrList)
}

func Test_protobufCodec_FastPb(t *testing.T) {
	c := protobufCodec{}
	ctx := context.Background()
	ri := newMockRPCInfo("TestMethod")

	msg := &mockFastCodecMsg{
		num: 100,
		v:   "test",
	}

	payload, needRecycle, err := c.encode(ctx, ri, msg)
	test.Assert(t, err == nil, err)
	test.Assert(t, payload != nil)
	test.Assert(t, needRecycle)

	newMsg := &mockFastCodecMsg{}
	err = c.decode(ctx, ri, payload, newMsg)
	test.Assert(t, err == nil, err)
	test.Assert(t, newMsg.num == msg.num)
	test.Assert(t, newMsg.v == msg.v)
}

func Test_protobufCodec_Marshaler(t *testing.T) {
	c := protobufCodec{}
	ctx := context.Background()
	ri := newMockRPCInfo("TestMethod")

	msg := &mockMarshaler{data: "test"}
	payload, needRecycle, err := c.encode(ctx, ri, msg)
	test.Assert(t, err == nil, err)
	test.Assert(t, payload != nil)
	test.Assert(t, needRecycle)
	test.Assert(t, string(payload) == "test")

	msgFail := &mockMarshaler{data: "test", fail: true}
	payload, needRecycle, err = c.encode(ctx, ri, msgFail)
	test.Assert(t, err != nil)
	test.Assert(t, payload == nil)
	test.Assert(t, !needRecycle)
}

func Test_protobufCodec_InvalidMessage(t *testing.T) {
	c := protobufCodec{}
	ctx := context.Background()
	ri := newMockRPCInfo("TestMethod")

	_, _, err := c.encode(ctx, ri, "invalid")
	test.Assert(t, err != nil)
	test.Assert(t, err.Error() == "invalid payload string in EncodeProtobufStruct", err)

	err = c.decode(ctx, ri, []byte("data"), "invalid")
	test.Assert(t, err.Error() == "invalid payload string in DecodeProtobufStruct", err)
}
