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

package thrift

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/netpoll"

	mt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	netpolltrans "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
)

type trackingNocopyByteBuffer struct {
	remote.ByteBuffer
	writeDirectCalls int
	mallocAckCalls   int
}

func (b *trackingNocopyByteBuffer) WriteDirect(p []byte, remainCap int) error {
	b.writeDirectCalls++
	return b.ByteBuffer.(remote.NocopyWrite).WriteDirect(p, remainCap)
}

func (b *trackingNocopyByteBuffer) MallocAck(n int) error {
	b.mallocAckCalls++
	return b.ByteBuffer.(remote.NocopyWrite).MallocAck(n)
}

func newTrackingNocopyByteBuffer() *trackingNocopyByteBuffer {
	return &trackingNocopyByteBuffer{
		ByteBuffer: netpolltrans.NewReaderWriterByteBuffer(netpoll.NewLinkBuffer()),
	}
}

// binaryFastCodec exercises the binary-field writer with mutable input.
type binaryFastCodec struct {
	data []byte
}

func (p *binaryFastCodec) BLength() int {
	return thrift.Binary.FieldBeginLength() + thrift.Binary.BinaryLength(p.data) + thrift.Binary.FieldStopLength()
}

func (p *binaryFastCodec) FastWriteNocopy(buf []byte, w thrift.NocopyWriter) int {
	n := thrift.Binary.WriteFieldBegin(buf, thrift.STRING, 1)
	n += thrift.Binary.WriteBinaryNocopy(buf[n:], w, p.data)
	n += thrift.Binary.WriteFieldStop(buf[n:])
	return n
}

func (p *binaryFastCodec) FastRead(buf []byte) (int, error) {
	typ, id, n, err := thrift.Binary.ReadFieldBegin(buf)
	if err != nil {
		return n, err
	}
	if typ != thrift.STRING || id != 1 {
		return n, fmt.Errorf("unexpected field: type=%d id=%d", typ, id)
	}
	data, l, err := thrift.Binary.ReadBinary(buf[n:])
	n += l
	if err != nil {
		return n, err
	}
	p.data = data
	typ, _, l, err = thrift.Binary.ReadFieldBegin(buf[n:])
	n += l
	if err == nil && typ != thrift.STOP {
		err = fmt.Errorf("expected STOP, got %d", typ)
	}
	return n, err
}

func TestFastWriteCopy(t *testing.T) {
	codecs := []struct {
		name      string
		codec     remote.PayloadCodec
		forceCopy bool
	}{
		{"default", NewThriftCodec(), false},
		{"copy", NewThriftCodecWithFastWriteCopy(), true},
		{"fallback default", NewThriftCodecWithConfig(Basic), false},
		{"fallback copy", NewThriftCodecWithConfig(FastRead | fastWriteCopy), true},
	}
	fields := []struct {
		name    string
		newData func() thrift.FastCodec
		newRecv func() thrift.FastCodec
	}{
		{
			"string",
			func() thrift.FastCodec {
				return &mt.MockTestArgs{Req: &mt.MockReq{
					Msg:     strings.Repeat("s", 8192),
					StrMap:  map[string]string{},
					StrList: []string{},
				}}
			},
			func() thrift.FastCodec { return &mt.MockTestArgs{} },
		},
		{
			"binary",
			func() thrift.FastCodec { return &binaryFastCodec{data: bytes.Repeat([]byte("b"), 8192)} },
			func() thrift.FastCodec { return &binaryFastCodec{} },
		},
	}
	for _, tc := range codecs {
		for _, field := range fields {
			t.Run(tc.name+"/"+field.name, func(t *testing.T) {
				data := field.newData()
				sendMsg := newMsg(data)
				inv := sendMsg.RPCInfo().Invocation()
				want, err := thrift.MarshalFastMsg(inv.MethodName(), thrift.CALL, inv.SeqID(), data)
				test.Assert(t, err == nil, err)

				out := newTrackingNocopyByteBuffer()
				defer out.Release(nil)
				err = tc.codec.Marshal(context.Background(), sendMsg, out)
				test.Assert(t, err == nil, err)
				test.Assert(t, (out.writeDirectCalls > 0) == !tc.forceCopy, out.writeDirectCalls)
				test.Assert(t, (out.mallocAckCalls > 0) == !tc.forceCopy, out.mallocAckCalls)
				test.Assert(t, out.WrittenLen() == len(want), out.WrittenLen(), len(want))

				// Mutating the caller's binary after Marshal must not alter the copied output.
				if binary, ok := data.(*binaryFastCodec); ok && tc.forceCopy {
					for i := range binary.data {
						binary.data[i] = 'x'
					}
				}
				test.Assert(t, out.Flush() == nil)
				got, err := out.Bytes()
				test.Assert(t, err == nil, err)
				test.Assert(t, bytes.Equal(got, want), "encoded bytes differ")

				recvMsg := newMsg(field.newRecv())
				recvMsg.SetPayloadLen(len(want))
				err = tc.codec.Unmarshal(context.Background(), recvMsg, out)
				test.Assert(t, err == nil, err)
				test.DeepEqual(t, recvMsg.Data(), field.newData())
			})
		}
	}
}

func TestFastWriteCopyException(t *testing.T) {
	message := strings.Repeat("e", 8192)
	for _, tc := range []struct {
		name   string
		err    error
		typeID int32
	}{
		{"transport error", remote.NewTransErrorWithMsg(remote.UnknownMethod, message), remote.UnknownMethod},
		{"plain error", errors.New(message), remote.InternalError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			codec := NewThriftCodecWithFastWriteCopy()
			sendMsg := newMsg(tc.err)
			sendMsg.SetMessageType(remote.Exception)
			out := newTrackingNocopyByteBuffer()
			defer out.Release(nil)
			err := codec.Marshal(context.Background(), sendMsg, out)
			test.Assert(t, err == nil, err)
			test.Assert(t, out.writeDirectCalls == 0, out.writeDirectCalls)
			test.Assert(t, out.mallocAckCalls == 0, out.mallocAckCalls)
			writtenLen := out.WrittenLen()
			test.Assert(t, out.Flush() == nil)
			recvMsg := initClientRecvMsg(sendMsg.RPCInfo())
			recvMsg.SetPayloadLen(writtenLen)
			err = codec.Unmarshal(context.Background(), recvMsg, out)
			var transErr *remote.TransError
			test.Assert(t, errors.As(err, &transErr), err)
			test.Assert(t, transErr.TypeID() == tc.typeID, transErr.TypeID())
			test.Assert(t, transErr.Error() == message, transErr.Error())
		})
	}
}
