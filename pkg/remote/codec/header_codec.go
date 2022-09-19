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
	"fmt"
	"io"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

/**
 *  TTHeader Protocol
 *  +-------------2Byte--------------|-------------2Byte-------------+
 *	+----------------------------------------------------------------+
 *	| 0|                          LENGTH                             |
 *	+----------------------------------------------------------------+
 *	| 0|       HEADER MAGIC          |            FLAGS              |
 *	+----------------------------------------------------------------+
 *	|                         SEQUENCE NUMBER                        |
 *	+----------------------------------------------------------------+
 *	| 0|     Header Size(/32)        | ...
 *	+---------------------------------
 *
 *	Header is of variable size:
 *	(and starts at offset 14)
 *
 *	+----------------------------------------------------------------+
 *	| PROTOCOL ID  |NUM TRANSFORMS . |TRANSFORM 0 ID (uint8)|
 *	+----------------------------------------------------------------+
 *	|  TRANSFORM 0 DATA ...
 *	+----------------------------------------------------------------+
 *	|         ...                              ...                   |
 *	+----------------------------------------------------------------+
 *	|        INFO 0 ID (uint8)      |       INFO 0  DATA ...
 *	+----------------------------------------------------------------+
 *	|         ...                              ...                   |
 *	+----------------------------------------------------------------+
 *	|                                                                |
 *	|                              PAYLOAD                           |
 *	|                                                                |
 *	+----------------------------------------------------------------+
 */

// Header keys
const (
	// Header Magics
	// 0 and 16th bits must be 0 to differentiate from framed & unframed
	TTHeaderMagic     uint32 = 0x10000000
	MeshHeaderMagic   uint32 = 0xFFAF0000
	MeshHeaderLenMask uint32 = 0x0000FFFF

	// HeaderMask        uint32 = 0xFFFF0000
	FlagsMask     uint32 = 0x0000FFFF
	MethodMask    uint32 = 0x41000000 // method first byte [A-Za-z_]
	MaxFrameSize  uint32 = 0x3FFFFFFF
	MaxHeaderSize uint32 = 65536
)

type HeaderFlags uint16

const (
	HeaderFlagSupportOutOfOrder HeaderFlags = 0x01
	HeaderFlagDuplexReverse     HeaderFlags = 0x08
	HeaderFlagSASL              HeaderFlags = 0x10
)

const (
	TTHeaderMetaSize = 14
)

// ProtocolID is the wrapped protocol id used in THeader.
type ProtocolID uint8

// Supported ProtocolID values.
const (
	ProtocolIDThriftBinary    ProtocolID = 0x00
	ProtocolIDThriftCompact   ProtocolID = 0x02 // Kitex not support
	ProtocolIDThriftCompactV2 ProtocolID = 0x03 // Kitex not support
	ProtocolIDKitexProtobuf   ProtocolID = 0x04
	ProtocolIDDefault                    = ProtocolIDThriftBinary
)

type InfoIDType uint8 // uint8

const (
	InfoIDPadding     InfoIDType = 0
	InfoIDKeyValue    InfoIDType = 0x01
	InfoIDIntKeyValue InfoIDType = 0x10
	InfoIDACLToken    InfoIDType = 0x11
)

type ttHeader struct{}

func (t ttHeader) encode(ctx context.Context, message remote.Message, out remote.ByteBuffer) ([]byte, error) {
	tm := message.TransInfo()
	strKV := tm.TransStrInfo()
	intKV := tm.TransIntInfo()
	total, headerInfoSize := t.calcEncodeLength(strKV, intKV)

	buf, err := out.Malloc(total)
	if err != nil {
		return nil, perrors.NewProtocolErrorWithMsg(fmt.Sprintf("ttHeader malloc failed: %s", err.Error()))
	}

	_ = buf[15] // bounds check
	// 1. header meta
	flags := HeaderFlags(message.Header().Flags())
	binary.BigEndian.PutUint32(buf[4:8], TTHeaderMagic+uint32(flags))
	binary.BigEndian.PutUint32(buf[8:12], uint32(message.RPCInfo().Invocation().SeqID()))
	binary.BigEndian.PutUint16(buf[12:14], uint16(headerInfoSize/4))

	// 2.  header info
	// PROTOCOL ID(u8) + NUM TRANSFORMS(always 0)(u8) + TRANSFORM IDs([]u8)
	buf[14] = byte(getProtocolID(message.ProtocolInfo()))
	buf[15] = 0 // no transform IDs

	idx := 16
	// str kv info
	if size := len(strKV); size > 0 {
		// INFO ID TYPE(u8) + NUM HEADERS(u16)
		buf[idx] = byte(InfoIDKeyValue)
		binary.BigEndian.PutUint16(buf[idx+1:], uint16(size))
		idx += 3
		for k, v := range strKV {
			lenK, lenV := len(k), len(v)

			binary.BigEndian.PutUint16(buf[idx:], uint16(lenK))
			copy(buf[idx+2:], utils.StringToSliceByte(k))
			idx += 2 + lenK

			binary.BigEndian.PutUint16(buf[idx:], uint16(lenV))
			copy(buf[idx+2:], utils.StringToSliceByte(v))
			idx += 2 + lenV
		}
	}
	// int kv info
	if size := len(intKV); size > 0 {
		// INFO ID TYPE(u8) + NUM HEADERS(u16)
		buf[idx] = byte(InfoIDIntKeyValue)
		binary.BigEndian.PutUint16(buf[idx+1:], uint16(size))
		idx += 3
		for k, v := range intKV {
			binary.BigEndian.PutUint16(buf[idx:], k)
			idx += 2

			lenV := len(v)
			binary.BigEndian.PutUint16(buf[idx:], uint16(lenV))
			copy(buf[idx+2:], utils.StringToSliceByte(v))
			idx += 2 + lenV
		}
	}
	for idx < total {
		buf[idx] = 0
		idx++
	}

	return buf[:4], err
}

func (t ttHeader) calcEncodeLength(strKV map[string]string, intKV map[uint16]string) (total, infoSize int) {
	// 1. header meta
	total += TTHeaderMetaSize

	// 2. header info
	infoSize += 1 /* Protocol ID (u8) */ + 1 /* number of transform IDs (u8) */ + 0 /* transform IDs u8s */

	// 3. KV infos
	if len(strKV) > 0 {
		infoSize += 1 /* INFO ID TYPE(u8) */ + 2 /* NUM HEADERS(u16) */
		for k, v := range strKV {
			infoSize += 2 /* length (u16) */ + len(k)
			infoSize += 2 /* length (u16) */ + len(v)
		}
	}

	if len(intKV) > 0 {
		infoSize += 1 /* INFO ID TYPE(u8) */ + 2 /* NUM HEADERS(u16) */
		for _, v := range intKV {
			infoSize += 2 /* key (u16) */
			infoSize += 2 /* length (u16) */ + len(v)
		}
	}

	// 4. padding by 4
	infoSize = ((infoSize + 3) >> 2) << 2

	total += infoSize
	return
}

func (t ttHeader) decode(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	headerMeta, err := in.Next(TTHeaderMetaSize)
	if err != nil {
		return perrors.NewProtocolError(err)
	}
	if !IsTTHeader(headerMeta) {
		return perrors.NewProtocolErrorWithMsg("not TTHeader protocol")
	}

	flags := Bytes2Uint16NoCheck(headerMeta[Size16*3:])
	if message.MessageType() == remote.Call {
		message.Header().SetFlags(flags)
	}

	seqID := Bytes2Uint32NoCheck(headerMeta[Size32*2 : Size32*3])
	if err = SetOrCheckSeqID(int32(seqID), message); err != nil {
		klog.Warnf("the seqID in TTHeader check failed, error=%s", err.Error())
		// some framework doesn't write correct seqID in TTheader, to ignore err only check it in payload
		// print log to push the downstream framework to refine it.
	}
	headerInfoSize := Bytes2Uint16NoCheck(headerMeta[Size32*3:TTHeaderMetaSize]) * 4
	if uint32(headerInfoSize) > MaxHeaderSize || headerInfoSize < 2 {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("invalid header length[%d]", headerInfoSize))
	}

	var headerInfo []byte
	if headerInfo, err = in.Next(int(headerInfoSize)); err != nil {
		return perrors.NewProtocolError(err)
	}
	if err = checkProtocolID(headerInfo[0], message); err != nil {
		return err
	}
	hdIdx := 2
	transformIDNum := int(headerInfo[1])
	if transformIDNum > 0 {
		if int(headerInfoSize)-hdIdx < transformIDNum {
			return perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("need read %d transformIDs, but not enough", transformIDNum))
		}
		transformIDs := make([]uint8, transformIDNum)
		for i := 0; i < transformIDNum; i++ {
			transformIDs[i] = headerInfo[hdIdx]
			hdIdx++
		}
	}

	intKV := message.TransInfo().TransIntInfo()
	strKV := message.TransInfo().TransStrInfo()
	if err := readKVInfo(headerInfo[hdIdx:], intKV, strKV); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("ttHeader read kv info failed, %s", err.Error()))
	}
	fillBasicInfoOfTTHeader(message, intKV, strKV)

	totalLen := Bytes2Uint32NoCheck(headerMeta[:Size32])
	message.SetPayloadLen(int(totalLen - uint32(headerInfoSize) + Size32 - TTHeaderMetaSize))
	return err
}

func readKVInfo(bytes []byte, intKV map[uint16]string, strKV map[string]string) (err error) {
	off, end := 0, len(bytes)
loop:
	for off < end {
		infoID := InfoIDType(bytes[off])
		off++
		switch infoID {
		case InfoIDPadding:
			continue
		case InfoIDKeyValue:
			if off+2 > end {
				err = io.EOF
				break loop
			} else {
				cnt := int(bytes[off+1]) | int(bytes[off])<<8
				off += 2
				spans := make([]int, cnt*4)
				for i := 0; i < cnt*2; i++ { // string key-value pairs
					if off+2 > end {
						err = io.EOF
						break loop
					}
					length := int(bytes[off+1]) | int(bytes[off])<<8
					off += 2

					if length < 0 || off+length > end {
						err = io.EOF
						break loop
					}
					spans[i*2] = off
					spans[i*2+1] = off + length
					off += length
				}
				for i := 0; i < cnt*4; i += 4 {
					b, e := spans[i], spans[i+1]
					k := string(bytes[b:e])
					b, e = spans[i+2], spans[i+3]
					v := string(bytes[b:e])
					strKV[k] = v
				}
			}
		case InfoIDIntKeyValue:
			if off+2 > end {
				err = io.EOF
				break loop
			} else {
				cnt := int(bytes[off+1]) | int(bytes[off])<<8
				off += 2
				spans := make([]int, cnt*3)
				for i := 0; i < cnt; i++ { // int key & string value
					if off+4 > end {
						err = io.EOF
						break loop
					}
					key := int(bytes[off+1]) | int(bytes[off])<<8
					length := int(bytes[off+3]) | int(bytes[off+2])<<8
					off += 4

					if length < 0 || off+length > end {
						err = io.EOF
						break loop
					}
					spans[i*3] = key
					spans[i*3+1] = off
					spans[i*3+2] = off + length
					off += length
				}
				for i := 0; i < cnt*3; i += 3 {
					k, b, e := spans[i], spans[i+1], spans[i+2]
					v := string(bytes[b:e])
					intKV[uint16(k)] = v
				}
			}
		case InfoIDACLToken:
			if off+2 > end {
				err = io.EOF
				break loop
			} else {
				// skip ACLToken
				length := int(bytes[off+1]) | int(bytes[off])<<8
				off += 2
				if length < 0 || off+length > end {
					err = io.EOF
					break loop
				}
				off += length
			}
		default:
			err = fmt.Errorf("invalid infoIDType[%#x]", infoID)
			break loop
		}
	}
	return
}

func readStrKVInfo(idx *int, buf []byte, info map[string]string) (has bool, err error) {
	kvSize, err := Bytes2Uint16(buf, *idx)
	*idx += 2
	if err != nil {
		return false, fmt.Errorf("error reading str kv info size: %s", err.Error())
	}
	if kvSize <= 0 {
		return false, nil
	}
	for i := uint16(0); i < kvSize; i++ {
		key, n, err := ReadString2BLen(buf, *idx)
		*idx += n
		if err != nil {
			return false, fmt.Errorf("error reading str kv info: %s", err.Error())
		}
		val, n, err := ReadString2BLen(buf, *idx)
		*idx += n
		if err != nil {
			return false, fmt.Errorf("error reading str kv info: %s", err.Error())
		}
		info[key] = val
	}
	return true, nil
}

// protoID just for ttheader
func getProtocolID(pi remote.ProtocolInfo) ProtocolID {
	switch pi.CodecType {
	case serviceinfo.Protobuf:
		// ProtocolIDKitexProtobuf is 0x03 at old version(<=v1.9.1) , but it conflicts with ThriftCompactV2.
		// Change the ProtocolIDKitexProtobuf to 0x04 from v1.9.2. But notice! that it is an incompatible change of protocol.
		// For keeping compatible, Kitex use ProtocolIDDefault send ttheader+KitexProtobuf request to ignore the old version
		// check failed if use 0x04. It doesn't make sense, but it won't affect the correctness of RPC call because the actual
		// protocol check at checkPayload func which check payload with HEADER MAGIC bytes of payload.
		return ProtocolIDDefault
	}
	return ProtocolIDDefault
}

// protoID just for ttheader
func checkProtocolID(protoID uint8, message remote.Message) error {
	switch protoID {
	case uint8(ProtocolIDThriftBinary):
	case uint8(ProtocolIDKitexProtobuf):
	case uint8(ProtocolIDThriftCompactV2):
		// just for compatibility
	default:
		return fmt.Errorf("unsupported ProtocolID[%d]", protoID)
	}
	return nil
}

/**
 * +-------------2Byte-------------|-------------2Byte--------------+
 * +----------------------------------------------------------------+
 * |       HEADER MAGIC            |      HEADER SIZE               |
 * +----------------------------------------------------------------+
 * |       HEADER MAP SIZE         |    HEADER MAP...               |
 * +----------------------------------------------------------------+
 * |                                                                |
 * |                            PAYLOAD                             |
 * |                                                                |
 * +----------------------------------------------------------------+
 */
type meshHeader struct{}

//lint:ignore U1000 until encode is used
func (m meshHeader) encode(ctx context.Context, message remote.Message, payloadBuf, out remote.ByteBuffer) error {
	// do nothing, kitex just support decode meshHeader, encode protocol depend on the payload
	return nil
}

func (m meshHeader) decode(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	headerMeta, err := in.Next(Size32)
	if err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("meshHeader read header meta failed, %s", err.Error()))
	}
	if !isMeshHeader(headerMeta) {
		return perrors.NewProtocolErrorWithMsg("not MeshHeader protocol")
	}
	headerLen := Bytes2Uint16NoCheck(headerMeta[Size16:])
	var headerInfo []byte
	if headerInfo, err = in.Next(int(headerLen)); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("meshHeader read header buf failed, %s", err.Error()))
	}
	mapInfo := message.TransInfo().TransStrInfo()
	idx := 0
	if _, err = readStrKVInfo(&idx, headerInfo, mapInfo); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("meshHeader read kv info failed, %s", err.Error()))
	}
	return nil
}

// Fill basic from_info(from service, from address) which carried by ttheader to rpcinfo.
// It is better to fill rpcinfo in matahandlers in terms of design,
// but metahandlers are executed after payloadDecode, we don't know from_info when error happen in payloadDecode.
// So 'fillBasicInfoOfTTHeader' is just for getting more info to output log when decode error happen.
func fillBasicInfoOfTTHeader(msg remote.Message, intKV map[uint16]string, strKV map[string]string) {
	if msg.RPCRole() == remote.Server {
		fi := rpcinfo.AsMutableEndpointInfo(msg.RPCInfo().From())
		if fi != nil {
			if v := strKV[transmeta.HeaderTransRemoteAddr]; v != "" {
				fi.SetAddress(utils.NewNetAddr("tcp", v))
			}
			if v := intKV[transmeta.FromService]; v != "" {
				fi.SetServiceName(v)
			}
		}
	} else {
		ti := remoteinfo.AsRemoteInfo(msg.RPCInfo().To())
		if ti != nil {
			if v := strKV[transmeta.HeaderTransRemoteAddr]; v != "" {
				ti.SetRemoteAddr(utils.NewNetAddr("tcp", v))
			}
		}
	}
}
