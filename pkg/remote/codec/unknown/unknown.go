package unknown

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	thrif "github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/dynamicgo/thrift"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	unknowns "github.com/cloudwego/kitex/pkg/remote/codec/unknown/service"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// UnknownCodec implements PayloadCodec
type unknownCodec struct {
	Codec remote.PayloadCodec
}

func NewUnknownCodec(code remote.PayloadCodec) remote.PayloadCodec {
	return &unknownCodec{code}
}

func (c unknownCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	nw, _ := out.(remote.NocopyWrite)
	ink := msg.RPCInfo().Invocation()
	data := msg.Data()
	msgType := msg.MessageType()
	if data, ok := data.(*unknowns.Result); ok {
		//svcInfo := message.ServiceInfo()
		//unknowns.SetServiceInfo(svcInfo, message.ProtocolInfo().CodecType, data.Method, data.Method)
		if len(data.Success) == 0 {
			return errors.New("unknown messages cannot be empty")
		}
		if msg.MessageType() == remote.Exception {
			return c.Codec.Marshal(ctx, msg, out)
		}
		if ink, ok := ink.(rpcinfo.InvocationSetter); ok {
			ink.SetMethodName(data.Method)
			ink.SetServiceName(data.ServiceName)
		} else {
			return errors.New("the interface Invocation doesn't implement InvocationSetter")
		}
		if msg.ProtocolInfo().CodecType == serviceinfo.Thrift {
			msgBeginLen := bthrift.Binary.MessageBeginLength(data.Method, thrif.TMessageType(msgType), ink.SeqID())
			msgEndLen := bthrift.Binary.MessageEndLength()

			//var buf []byte
			buf, err := out.Malloc(msgBeginLen + len(data.Success) + msgEndLen)
			if err != nil {
				return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, Malloc failed: %s", err.Error()))
			}
			offset := bthrift.Binary.WriteMessageBegin(buf, data.Method, thrif.TMessageType(msgType), ink.SeqID())
			//offset += bthrift.Binary.WriteBinaryNocopy(buf[offset:], nw, []byte(data.Success))
			c.Write(buf[offset:], data.Success)
			bthrift.Binary.WriteMessageEnd(buf[offset:])
			//offset += msg.FastWriteNocopy(buff[offset:], nil)
			if nw == nil {
				// if nw is nil, FastWrite will act in Copy mode.
				return nil
			}
			return nw.MallocAck(out.MallocLen())
		} else if msg.ProtocolInfo().CodecType == serviceinfo.Protobuf {

			// 3.1 magic && msgType
			if err := codec.WriteUint32(codec.ProtobufV1Magic+uint32(msg.MessageType()), out); err != nil {
				return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("protobuf marshal, write meta info failed: %s", err.Error()))
			}
			// 3.2 methodName
			if _, err := codec.WriteString(data.Method, out); err != nil {
				return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("protobuf marshal, write method name failed: %s", err.Error()))
			}
			// 3.3 seqID
			if err := codec.WriteUint32(uint32(ink.SeqID()), out); err != nil {
				return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("protobuf marshal, write seqID failed: %s", err.Error()))
			}
			buf, err := out.Malloc(len(data.Success))
			if err != nil {
				return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("protobuf malloc size %d failed: %s", len(data.Success), err.Error()))
			}
			c.Write(buf, data.Success)
			return nil
		}

	}

	return c.Codec.Marshal(ctx, msg, out)
}

func (c unknownCodec) Unmarshal(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	ink := message.RPCInfo().Invocation()
	service, method, size, err := parse(message, in)
	if err != nil {
		return c.Codec.Unmarshal(ctx, message, in)
	}
	err = codec.SetOrCheckMethodName(method, message)
	if te, ok := err.(*remote.TransError); ok && te.TypeID() == remote.UnknownMethod {
		svcInfo, err := message.SpecifyServiceInfo(unknowns.UnknownService, unknowns.UnknownMethod)
		if err != nil {
			return err
		}

		if ink, ok := ink.(rpcinfo.InvocationSetter); ok {
			ink.SetMethodName(unknowns.UnknownMethod)
			ink.SetPackageName(svcInfo.GetPackageName())
			ink.SetServiceName(unknowns.UnknownService)
		} else {
			return errors.New("the interface Invocation doesn't implement InvocationSetter")
		}
		if err = codec.NewDataIfNeeded(unknowns.UnknownMethod, message); err != nil {
			return err
		}

		data := message.Data()

		if data, ok := data.(*unknowns.Args); ok {
			data.Method = method
			data.ServiceName = service
			err := in.Skip(int(size))
			if err != nil {
				return err
			}
			buf, err := in.Next(in.ReadableLen())
			if err != nil {
				return err
			}
			data.Request = buf
		}
		return nil
	}

	return c.Codec.Unmarshal(ctx, message, in)
}

func (c unknownCodec) Name() string {
	return "unknownMethodCodec"
}

func (c *unknownCodec) Write(dst, src []byte) {
	copy(dst, src)
}

func parse(message remote.Message, in remote.ByteBuffer) (string, string, int32, error) {
	code := message.ProtocolInfo().CodecType
	if code == serviceinfo.Thrift {
		return parseThrift(message, in)
	} else if code == serviceinfo.Protobuf {
		return parseProtobuf(message, in)
	}
	return "", "", 0, nil
}

func parseThrift(message remote.Message, in remote.ByteBuffer) (string, string, int32, error) {
	buf, err := in.Peek(4)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	size := int32(binary.BigEndian.Uint32(buf))
	if size > 0 {
		return "", "", 0, perrors.NewProtocolErrorWithType(perrors.BadVersion, "Missing version in ReadMessageBegin")
	}
	msgType := thrift.TMessageType(size & 0x0ff)
	if err = codec.UpdateMsgType(uint32(msgType), message); err != nil {
		return "", "", 0, err
	}
	version := int64(int64(size) & thrift.VERSION_MASK)
	if version != thrift.VERSION_1 {
		return "", "", 0, perrors.NewProtocolErrorWithType(perrors.BadVersion, "Bad version in ReadMessageBegin")
	}
	// exception message
	if message.MessageType() == remote.Exception {
		return "", "", 0, perrors.NewProtocolErrorWithMsg("thrift unmarshal")
	}
	// 获取method
	method, size, err := peekMethod(in)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	seqID, err := peekSeqID(in, size)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	if err = codec.SetOrCheckSeqID(seqID, message); err != nil {
		return "", "", 0, err
	}
	return message.RPCInfo().Invocation().ServiceName(), method, size + 4, nil
}

func parseProtobuf(message remote.Message, in remote.ByteBuffer) (string, string, int32, error) {
	magicAndMsgType, err := codec.PeekUint32(in)
	if err != nil {
		return "", "", 0, err
	}
	if magicAndMsgType&codec.MagicMask != codec.ProtobufV1Magic {
		return "", "", 0, perrors.NewProtocolErrorWithType(perrors.BadVersion, "Bad version in protobuf Unmarshal")
	}
	msgType := magicAndMsgType & codec.FrontMask
	if err = codec.UpdateMsgType(msgType, message); err != nil {
		return "", "", 0, err
	}

	method, size, err := peekMethod(in)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	seqID, err := peekSeqID(in, size)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	if err = codec.SetOrCheckSeqID(seqID, message); err != nil && msgType != uint32(remote.Exception) {
		return "", "", 0, err
	}
	// exception message
	if message.MessageType() == remote.Exception {
		return "", "", 0, perrors.NewProtocolErrorWithMsg("protobuf unmarshal")
	}
	return message.RPCInfo().Invocation().ServiceName(), method, size + 4, nil
}

func peekMethod(in remote.ByteBuffer) (string, int32, error) {
	buf, err := in.Peek(8)
	if err != nil {
		return "", 0, err
	}
	buf = buf[4:]
	size := int32(binary.BigEndian.Uint32(buf))
	buf, err = in.Peek(int(size + 8))
	if err != nil {
		return "", 0, perrors.NewProtocolError(err)
	}
	buf = buf[8:]
	method := string(buf)
	return method, size + 8, nil
}

func peekSeqID(in remote.ByteBuffer, size int32) (int32, error) {
	buf, err := in.Peek(int(size + 4))
	if err != nil {
		return 0, perrors.NewProtocolError(err)
	}
	buf = buf[size:]
	seqID := int32(binary.BigEndian.Uint32(buf))
	return seqID, nil
}
