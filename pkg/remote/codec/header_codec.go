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
	HeaderFlagsKey              string      = "HeaderFlags"
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

func (t ttHeader) encode(ctx context.Context, message remote.Message, out remote.ByteBuffer) (totalLenField []byte, err error) {
	// 1. header meta
	var headerMeta []byte
	headerMeta, err = out.Malloc(TTHeaderMetaSize)
	if err != nil {
		return nil, perrors.NewProtocolErrorWithMsg(fmt.Sprintf("ttHeader malloc header meta failed, %s", err.Error()))
	}

	totalLenField = headerMeta[0:4]
	headerInfoSizeField := headerMeta[12:14]
	binary.BigEndian.PutUint32(headerMeta[4:8], TTHeaderMagic+uint32(getFlags(message)))
	binary.BigEndian.PutUint32(headerMeta[8:12], uint32(message.RPCInfo().Invocation().SeqID()))

	var transformIDs []uint8 // transformIDs not support TODO compress
	// 2.  header info, malloc and write
	if err = WriteByte(byte(getProtocolID(message.ProtocolInfo())), out); err != nil {
		return nil, perrors.NewProtocolErrorWithMsg(fmt.Sprintf("ttHeader write protocol id failed, %s", err.Error()))
	}
	if err = WriteByte(byte(len(transformIDs)), out); err != nil {
		return nil, perrors.NewProtocolErrorWithMsg(fmt.Sprintf("ttHeader write transformIDs length failed, %s", err.Error()))
	}
	for tid := range transformIDs {
		if err = WriteByte(byte(tid), out); err != nil {
			return nil, perrors.NewProtocolErrorWithMsg(fmt.Sprintf("ttHeader write transformIDs failed, %s", err.Error()))
		}
	}
	// PROTOCOL ID(u8) + NUM TRANSFORMS(always 0)(u8) + TRANSFORM IDs([]u8)
	headerInfoSize := 1 + 1 + len(transformIDs)
	headerInfoSize, err = writeKVInfo(headerInfoSize, message, out)
	if err != nil {
		return nil, perrors.NewProtocolErrorWithMsg(fmt.Sprintf("ttHeader write kv info failed, %s", err.Error()))
	}

	binary.BigEndian.PutUint16(headerInfoSizeField, uint16(headerInfoSize/4))
	return totalLenField, err
}

func (t ttHeader) decode(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	headerMeta, err := in.Next(TTHeaderMetaSize)
	if err != nil {
		return perrors.NewProtocolError(err)
	}
	if !IsTTHeader(headerMeta) {
		return perrors.NewProtocolErrorWithMsg("not TTHeader protocol")
	}
	totalLen := Bytes2Uint32NoCheck(headerMeta[:Size32])

	flags := Bytes2Uint16NoCheck(headerMeta[Size16*3:])
	setFlags(flags, message)

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
	if int(headerInfoSize)-hdIdx < transformIDNum {
		return perrors.NewProtocolErrorWithType(perrors.InvalidData, fmt.Sprintf("need read %d transformIDs, but not enough", transformIDNum))
	}
	transformIDs := make([]uint8, transformIDNum)
	for i := 0; i < transformIDNum; i++ {
		transformIDs[i] = headerInfo[hdIdx]
		hdIdx++
	}

	if err := readKVInfo(hdIdx, headerInfo, message); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("ttHeader read kv info failed, %s, headerInfo=%#x", err.Error(), headerInfo))
	}
	fillBasicInfoOfTTHeader(message)

	message.SetPayloadLen(int(totalLen - uint32(headerInfoSize) + Size32 - TTHeaderMetaSize))
	return err
}

func writeKVInfo(writtenSize int, message remote.Message, out remote.ByteBuffer) (writeSize int, err error) {
	writeSize = writtenSize
	tm := message.TransInfo()
	// str kv info
	strKVMap := tm.TransStrInfo()
	strKVSize := len(strKVMap)
	// write gdpr token into InfoIDACLToken
	// supplementary doc: https://www.cloudwego.io/docs/kitex/reference/transport_protocol_ttheader/
	if gdprToken, ok := strKVMap[transmeta.GDPRToken]; ok {
		strKVSize--
		// INFO ID TYPE(u8)
		if err = WriteByte(byte(InfoIDACLToken), out); err != nil {
			return writeSize, err
		}
		writeSize += 1

		wLen, err := WriteString2BLen(gdprToken, out)
		if err != nil {
			return writeSize, err
		}
		writeSize += wLen
	}

	if strKVSize > 0 {
		// INFO ID TYPE(u8) + NUM HEADERS(u16)
		if err = WriteByte(byte(InfoIDKeyValue), out); err != nil {
			return writeSize, err
		}
		if err = WriteUint16(uint16(strKVSize), out); err != nil {
			return writeSize, err
		}
		writeSize += 3
		for key, val := range strKVMap {
			if key == transmeta.GDPRToken {
				continue
			}
			keyWLen, err := WriteString2BLen(key, out)
			if err != nil {
				return writeSize, err
			}
			valWLen, err := WriteString2BLen(val, out)
			if err != nil {
				return writeSize, err
			}
			writeSize = writeSize + keyWLen + valWLen
		}
	}

	// int kv info
	intKVSize := len(tm.TransIntInfo())
	if intKVSize > 0 {
		// INFO ID TYPE(u8) + NUM HEADERS(u16)
		if err = WriteByte(byte(InfoIDIntKeyValue), out); err != nil {
			return writeSize, err
		}
		if err = WriteUint16(uint16(intKVSize), out); err != nil {
			return writeSize, err
		}
		writeSize += 3
		for key, val := range tm.TransIntInfo() {
			if err = WriteUint16(key, out); err != nil {
				return writeSize, err
			}
			valWLen, err := WriteString2BLen(val, out)
			if err != nil {
				return writeSize, err
			}
			writeSize = writeSize + 2 + valWLen
		}
	}

	// padding = (4 - headerInfoSize%4) % 4
	padding := (4 - writeSize%4) % 4
	paddingBuf, err := out.Malloc(padding)
	if err != nil {
		return writeSize, err
	}
	for i := 0; i < len(paddingBuf); i++ {
		paddingBuf[i] = byte(0)
	}
	writeSize += padding
	return
}

func readKVInfo(idx int, buf []byte, message remote.Message) error {
	intInfo := message.TransInfo().TransIntInfo()
	strInfo := message.TransInfo().TransStrInfo()
	for {
		infoID, err := Bytes2Uint8(buf, idx)
		idx++
		if err != nil {
			// this is the last field, read until there is no more padding
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		switch InfoIDType(infoID) {
		case InfoIDPadding:
			continue
		case InfoIDKeyValue:
			_, err := readStrKVInfo(&idx, buf, strInfo)
			if err != nil {
				return err
			}
		case InfoIDIntKeyValue:
			_, err := readIntKVInfo(&idx, buf, intInfo)
			if err != nil {
				return err
			}
		case InfoIDACLToken:
			if err := readACLToken(&idx, buf, strInfo); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid infoIDType[%#x]", infoID)
		}
	}
	return nil
}

func readIntKVInfo(idx *int, buf []byte, info map[uint16]string) (has bool, err error) {
	kvSize, err := Bytes2Uint16(buf, *idx)
	*idx += 2
	if err != nil {
		return false, fmt.Errorf("error reading int kv info size: %s", err.Error())
	}
	if kvSize <= 0 {
		return false, nil
	}
	for i := uint16(0); i < kvSize; i++ {
		key, err := Bytes2Uint16(buf, *idx)
		*idx += 2
		if err != nil {
			return false, fmt.Errorf("error reading int kv info: %s", err.Error())
		}
		val, n, err := ReadString2BLen(buf, *idx)
		*idx += n
		if err != nil {
			return false, fmt.Errorf("error reading int kv info: %s", err.Error())
		}
		info[key] = val
	}
	return true, nil
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

// readACLToken reads acl token
func readACLToken(idx *int, buf []byte, info map[string]string) error {
	val, n, err := ReadString2BLen(buf, *idx)
	*idx += n
	if err != nil {
		return fmt.Errorf("error reading acl token: %s", err.Error())
	}
	info[transmeta.GDPRToken] = val
	return nil
}

func getFlags(message remote.Message) HeaderFlags {
	var headerFlags HeaderFlags
	if message.Tags() != nil && message.Tags()[HeaderFlagsKey] != nil {
		if hfs, ok := message.Tags()[HeaderFlagsKey].(HeaderFlags); ok {
			headerFlags = hfs
		} else {
			klog.Warnf("KITEX: the type of headerFlags is invalid, %T", message.Tags()[HeaderFlagsKey])
		}
	}
	return headerFlags
}

func setFlags(flags uint16, message remote.Message) {
	if message.MessageType() == remote.Call {
		message.Tags()[HeaderFlagsKey] = HeaderFlags(flags)
	}
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
func fillBasicInfoOfTTHeader(msg remote.Message) {
	if msg.RPCRole() == remote.Server {
		fi := rpcinfo.AsMutableEndpointInfo(msg.RPCInfo().From())
		if fi != nil {
			if v := msg.TransInfo().TransStrInfo()[transmeta.HeaderTransRemoteAddr]; v != "" {
				fi.SetAddress(utils.NewNetAddr("tcp", v))
			}
			if v := msg.TransInfo().TransIntInfo()[transmeta.FromService]; v != "" {
				fi.SetServiceName(v)
			}
		}
	} else {
		ti := remoteinfo.AsRemoteInfo(msg.RPCInfo().To())
		if ti != nil {
			if v := msg.TransInfo().TransStrInfo()[transmeta.HeaderTransRemoteAddr]; v != "" {
				ti.SetRemoteAddr(utils.NewNetAddr("tcp", v))
			}
		}
	}
}
