/*
 * Copyright 2021 CloudWeGo Authors
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
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"sync"
	"sync/atomic"

	netpoll2 "github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

// The byte count of 32 and 16 integer values.
const (
	Size32 = 4
	Size16 = 2
)

const (
	// ThriftV1Magic is the magic code for thrift.VERSION_1
	ThriftV1Magic = 0x80010000
	// ProtobufV1Magic is the magic code for kitex protobuf
	ProtobufV1Magic = 0x90010000

	// MagicMask is bit mask for checking version.
	MagicMask = 0xffff0000
)

var (
	ttHeaderCodec   = ttHeader{}
	meshHeaderCodec = meshHeader{}

	_ remote.Codec       = (*defaultCodec)(nil)
	_ remote.MetaDecoder = (*defaultCodec)(nil)
)

// crc32cTable is used for crc32c check
var (
	crc32cTable    *crc32.Table
	crc32TableOnce sync.Once
)

// NewDefaultCodec creates the default protocol sniffing codec supporting thrift and protobuf.
func NewDefaultCodec() remote.Codec {
	// No size limit by default
	return &defaultCodec{
		maxSize: 0,
	}
}

// NewDefaultCodecWithSizeLimit creates the default protocol sniffing codec supporting thrift and protobuf but with size limit.
// maxSize is in bytes
func NewDefaultCodecWithSizeLimit(maxSize int) remote.Codec {
	return &defaultCodec{
		maxSize: maxSize,
	}
}

// NewDefaultCodecWithConfig creates the default protocol sniffing codec supporting thrift and protobuf with the input config.
func NewDefaultCodecWithConfig(cfg CodecConfig) remote.Codec {
	if cfg.CRC32Check {
		crc32TableOnce.Do(func() {
			crc32cTable = crc32.MakeTable(crc32.Castagnoli)
		})
	}
	return &defaultCodec{
		maxSize:    cfg.MaxSize,
		crc32Check: cfg.CRC32Check,
	}
}

// CodecConfig is the config of defaultCodec
type CodecConfig struct {
	MaxSize    int
	CRC32Check bool
}

type defaultCodec struct {
	// maxSize limits the max size of the payload
	maxSize int
	// If crc32Check is true, the codec will validate the payload using crc32c.
	// Only effective when transport is TTHeader.
	// Payload is all the data after TTHeader.
	crc32Check bool
}

// EncodePayload encode payload
func (c *defaultCodec) EncodePayload(ctx context.Context, message remote.Message, out remote.ByteBuffer) error {
	defer func() {
		// notice: mallocLen() must exec before flush, or it will be reset
		if ri := message.RPCInfo(); ri != nil {
			if ms := rpcinfo.AsMutableRPCStats(ri.Stats()); ms != nil {
				ms.SetSendSize(uint64(out.MallocLen()))
			}
		}
	}()
	var err error
	var framedLenField []byte
	headerLen := out.MallocLen()
	tp := message.ProtocolInfo().TransProto

	// 1. malloc framed field if needed
	if tp&transport.Framed == transport.Framed {
		if framedLenField, err = out.Malloc(Size32); err != nil {
			return err
		}
		headerLen += Size32
	}

	// 2. encode payload
	if err = c.encodePayload(ctx, message, out); err != nil {
		return err
	}

	// 3. fill framed field if needed
	var payloadLen int
	if tp&transport.Framed == transport.Framed {
		if framedLenField == nil {
			return perrors.NewProtocolErrorWithMsg("no buffer allocated for the framed length field")
		}
		payloadLen = out.MallocLen() - headerLen
		binary.BigEndian.PutUint32(framedLenField, uint32(payloadLen))
	} else if message.ProtocolInfo().CodecType == serviceinfo.Protobuf {
		return perrors.NewProtocolErrorWithMsg("protobuf just support 'framed' trans proto")
	}
	if tp&transport.TTHeader == transport.TTHeader {
		payloadLen = out.MallocLen() - Size32
	}
	err = checkPayloadSize(payloadLen, c.maxSize)
	return err
}

// EncodeMetaAndPayload encode meta and payload
func (c *defaultCodec) EncodeMetaAndPayload(ctx context.Context, message remote.Message, out remote.ByteBuffer, me remote.MetaEncoder) error {
	tp := message.ProtocolInfo().TransProto
	if c.crc32Check && tp&transport.TTHeader == transport.TTHeader {
		return c.encodeMetaAndPayloadWithCRC32C(ctx, message, out, me)
	}

	var err error
	var totalLenField []byte
	// 1. encode header and return totalLenField if needed
	// totalLenField will be filled after payload encoded
	if tp&transport.TTHeader == transport.TTHeader {
		if totalLenField, err = ttHeaderCodec.encode(ctx, message, out); err != nil {
			return err
		}
	}
	// 2. encode payload
	if err = me.EncodePayload(ctx, message, out); err != nil {
		return err
	}
	// 3. fill totalLen field for header if needed
	if tp&transport.TTHeader == transport.TTHeader {
		if totalLenField == nil {
			return perrors.NewProtocolErrorWithMsg("no buffer allocated for the header length field")
		}
		payloadLen := out.MallocLen() - Size32
		binary.BigEndian.PutUint32(totalLenField, uint32(payloadLen))
	}
	return nil
}

// encodeMetaAndPayloadWithCRC32C encodes payload and meta with crc32c checksum of the payload.
func (c *defaultCodec) encodeMetaAndPayloadWithCRC32C(ctx context.Context, message remote.Message, out remote.ByteBuffer, me remote.MetaEncoder) error {
	var err error

	// 1. encode payload and calculate crc32c checksum
	payloadOut := netpoll.NewWriterByteBuffer(netpoll2.NewLinkBuffer())
	if err = me.EncodePayload(ctx, message, payloadOut); err != nil {
		return err
	}
	// get the payload from buffer
	payload, payloadLen, err := payloadOut.(remote.NocopyRead).GetBytes()
	if err != nil {
		// release if err
		payloadOut.Release(err)
		return err
	}
	crc32c := getCRC32C(payload)
	strInfo := message.TransInfo().TransStrInfo()
	if crc32c != "" && strInfo != nil {
		strInfo[transmeta.HeaderCRC32C] = crc32c
	}
	// set payload length before encode TTHeader.
	message.SetPayloadLen(payloadLen)

	// 2. encode header and return totalLenField if needed
	// In this case, set total length during TTHeader encode
	if _, err = ttHeaderCodec.encode(ctx, message, out); err != nil {
		return err
	}

	// 3. write payload to the buffer after TTHeader
	if netpoll.IsNetpollByteBuffer(out) {
		// append buffer only if the input buffer is a netpollByteBuffer
		// release will be executed in AppendBuffer
		err = out.AppendBuffer(payloadOut)
	} else {
		// convert [][]byte to []byte
		p := flatten2DSlice(payload, payloadLen)
		if ncWriter, ok := out.(remote.NocopyWrite); ok {
			err = ncWriter.WriteDirect(p, 0)
		} else {
			_, err = out.WriteBinary(p)
		}
		payloadOut.Release(err)
	}
	return err
}

// Encode implements the remote.Codec interface, it does complete message encode include header and payload.
func (c *defaultCodec) Encode(ctx context.Context, message remote.Message, out remote.ByteBuffer) (err error) {
	return c.EncodeMetaAndPayload(ctx, message, out, c)
}

// DecodeMeta decode header
func (c *defaultCodec) DecodeMeta(ctx context.Context, message remote.Message, in remote.ByteBuffer) (err error) {
	var flagBuf []byte
	if flagBuf, err = in.Peek(2 * Size32); err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("default codec read failed: %s", err.Error()))
	}

	if err = checkRPCState(ctx, message); err != nil {
		// there is one call has finished in retry task, it doesn't need to do decode for this call
		return err
	}
	isTTHeader := IsTTHeader(flagBuf)
	// 1. decode header
	if isTTHeader {
		// TTHeader
		if err = ttHeaderCodec.decode(ctx, message, in); err != nil {
			return err
		}
		if flagBuf, err = in.Peek(2 * Size32); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("ttheader read payload first 8 byte failed: %s", err.Error()))
		}
		if c.crc32Check && crc32cTable != nil {
			if err = checkCRC32C(message, in); err != nil {
				return err
			}
		}
	} else if isMeshHeader(flagBuf) {
		message.Tags()[remote.MeshHeader] = true
		// MeshHeader
		if err = meshHeaderCodec.decode(ctx, message, in); err != nil {
			return err
		}
		if flagBuf, err = in.Peek(2 * Size32); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("meshHeader read payload first 8 byte failed: %s", err.Error()))
		}
	}
	return checkPayload(flagBuf, message, in, isTTHeader, c.maxSize)
}

// DecodePayload decode payload
func (c *defaultCodec) DecodePayload(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	defer func() {
		if ri := message.RPCInfo(); ri != nil {
			if ms := rpcinfo.AsMutableRPCStats(ri.Stats()); ms != nil {
				ms.SetRecvSize(uint64(in.ReadLen()))
			}
		}
	}()

	hasRead := in.ReadLen()
	pCodec, err := remote.GetPayloadCodec(message)
	if err != nil {
		return err
	}
	if err = pCodec.Unmarshal(ctx, message, in); err != nil {
		return err
	}
	if message.PayloadLen() == 0 {
		// if protocol is PurePayload, should set payload length after decoded
		message.SetPayloadLen(in.ReadLen() - hasRead)
	}
	return nil
}

// Decode implements the remote.Codec interface, it does complete message decode include header and payload.
func (c *defaultCodec) Decode(ctx context.Context, message remote.Message, in remote.ByteBuffer) (err error) {
	// 1. decode meta
	if err = c.DecodeMeta(ctx, message, in); err != nil {
		return err
	}

	// 2. decode payload
	return c.DecodePayload(ctx, message, in)
}

func (c *defaultCodec) Name() string {
	return "default"
}

// Select to use thrift or protobuf according to the protocol.
func (c *defaultCodec) encodePayload(ctx context.Context, message remote.Message, out remote.ByteBuffer) error {
	pCodec, err := remote.GetPayloadCodec(message)
	if err != nil {
		return err
	}
	return pCodec.Marshal(ctx, message, out)
}

/**
 * +------------------------------------------------------------+
 * |                  4Byte                 |       2Byte       |
 * +------------------------------------------------------------+
 * |   			     Length			    	|   HEADER MAGIC    |
 * +------------------------------------------------------------+
 */
func IsTTHeader(flagBuf []byte) bool {
	return binary.BigEndian.Uint32(flagBuf[Size32:])&MagicMask == TTHeaderMagic
}

/**
 * +----------------------------------------+
 * |       2Byte        |       2Byte       |
 * +----------------------------------------+
 * |    HEADER MAGIC    |   HEADER SIZE     |
 * +----------------------------------------+
 */
func isMeshHeader(flagBuf []byte) bool {
	return binary.BigEndian.Uint32(flagBuf[:Size32])&MagicMask == MeshHeaderMagic
}

/**
 * Kitex protobuf has framed field
 * +------------------------------------------------------------+
 * |                  4Byte                 |       2Byte       |
 * +------------------------------------------------------------+
 * |   			     Length			    	|   HEADER MAGIC    |
 * +------------------------------------------------------------+
 */
func isProtobufKitex(flagBuf []byte) bool {
	return binary.BigEndian.Uint32(flagBuf[Size32:])&MagicMask == ProtobufV1Magic
}

/**
 * +-------------------+
 * |       2Byte       |
 * +-------------------+
 * |   HEADER MAGIC    |
 * +-------------------
 */
func isThriftBinary(flagBuf []byte) bool {
	return binary.BigEndian.Uint32(flagBuf[:Size32])&MagicMask == ThriftV1Magic
}

/**
 * +------------------------------------------------------------+
 * |                  4Byte                 |       2Byte       |
 * +------------------------------------------------------------+
 * |   			     Length			    	|   HEADER MAGIC    |
 * +------------------------------------------------------------+
 */
func isThriftFramedBinary(flagBuf []byte) bool {
	return binary.BigEndian.Uint32(flagBuf[Size32:])&MagicMask == ThriftV1Magic
}

func checkRPCState(ctx context.Context, message remote.Message) error {
	if message.RPCRole() == remote.Server {
		return nil
	}
	if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
		return kerrors.ErrRPCFinish
	}
	if respOp, ok := ctx.Value(retry.CtxRespOp).(*int32); ok {
		if !atomic.CompareAndSwapInt32(respOp, retry.OpNo, retry.OpDoing) {
			// previous call is being handling or done
			// this flag is used to check request status in retry(backup request) scene
			return kerrors.ErrRPCFinish
		}
	}
	return nil
}

func checkPayload(flagBuf []byte, message remote.Message, in remote.ByteBuffer, isTTHeader bool, maxPayloadSize int) error {
	var transProto transport.Protocol
	var codecType serviceinfo.PayloadCodec
	if isThriftBinary(flagBuf) {
		codecType = serviceinfo.Thrift
		if isTTHeader {
			transProto = transport.TTHeader
		} else {
			transProto = transport.PurePayload
		}
	} else if isThriftFramedBinary(flagBuf) {
		codecType = serviceinfo.Thrift
		if isTTHeader {
			transProto = transport.TTHeaderFramed
		} else {
			transProto = transport.Framed
		}
		payloadLen := binary.BigEndian.Uint32(flagBuf[:Size32])
		message.SetPayloadLen(int(payloadLen))
		if err := in.Skip(Size32); err != nil {
			return err
		}
	} else if isProtobufKitex(flagBuf) {
		codecType = serviceinfo.Protobuf
		if isTTHeader {
			transProto = transport.TTHeaderFramed
		} else {
			transProto = transport.Framed
		}
		payloadLen := binary.BigEndian.Uint32(flagBuf[:Size32])
		message.SetPayloadLen(int(payloadLen))
		if err := in.Skip(Size32); err != nil {
			return err
		}
	} else {
		first4Bytes := binary.BigEndian.Uint32(flagBuf[:Size32])
		second4Bytes := binary.BigEndian.Uint32(flagBuf[Size32:])
		// 0xfff4fffd is the interrupt message of telnet
		err := perrors.NewProtocolErrorWithMsg(fmt.Sprintf("invalid payload (first4Bytes=%#x, second4Bytes=%#x)", first4Bytes, second4Bytes))
		return err
	}
	if err := checkPayloadSize(message.PayloadLen(), maxPayloadSize); err != nil {
		return err
	}
	message.SetProtocolInfo(remote.NewProtocolInfo(transProto, codecType))
	cfg := rpcinfo.AsMutableRPCConfig(message.RPCInfo().Config())
	if cfg != nil {
		tp := message.ProtocolInfo().TransProto
		cfg.SetTransportProtocol(tp)
	}
	return nil
}

func checkPayloadSize(payloadLen, maxSize int) error {
	if maxSize > 0 && payloadLen > 0 && payloadLen > maxSize {
		return perrors.NewProtocolErrorWithType(
			perrors.InvalidData,
			fmt.Sprintf("invalid data: payload size(%d) larger than the limit(%d)", payloadLen, maxSize),
		)
	}
	return nil
}

// getCRC32C calculates the crc32c checksum of the input bytes.
// the checksum will be converted into big-endian format and encoded into hex string.
func getCRC32C(payload [][]byte) string {
	if crc32cTable == nil {
		return ""
	}
	csb := make([]byte, Size32)
	var checksum uint32
	for i := 0; i < len(payload); i++ {
		checksum = crc32.Update(checksum, crc32cTable, payload[i])
	}
	binary.BigEndian.PutUint32(csb, checksum)
	return hex.EncodeToString(csb)
}

// checkCRC32C validates the crc32c checksum in the header.
func checkCRC32C(message remote.Message, in remote.ByteBuffer) error {
	strInfo := message.TransInfo().TransStrInfo()
	if strInfo == nil {
		return nil
	}
	crc32HexString := strInfo[transmeta.HeaderCRC32C]
	if len(crc32HexString) != 0 {
		crc32Byte, err := hex.DecodeString(crc32HexString)
		if err != nil {
			klog.Warnf("KITEX: crc32c key found in TTHeader, value is not a valid hex string")
			return nil
		}
		expectedChecksum := binary.BigEndian.Uint32(crc32Byte)
		payloadLen := message.PayloadLen() // total length
		payload, err := in.Peek(payloadLen)
		if err != nil {
			return err
		}
		realChecksum := crc32.Checksum(payload, crc32cTable)
		if realChecksum != expectedChecksum {
			return perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("crc32c payload check failed, expected=%d, actual=%d", expectedChecksum, realChecksum))
		}
	}
	return nil
}

// flatten2DSlice converts 2d slice to 1d.
// total length should be provided.
func flatten2DSlice(b2 [][]byte, length int) []byte {
	b1 := make([]byte, length)
	off := 0
	for i := 0; i < len(b2); i++ {
		off += copy(b1[off:], b2[i])
	}
	return b1
}
