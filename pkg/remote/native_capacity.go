package remote

import (
	"context"
	"errors"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/transport"
)

type NativeSizedTx interface {
	AcquireTxSized(context.Context, int) (NativeBuffer, error)
}

type nativeHeaderSizer struct{ length int }

func (s *nativeHeaderSizer) Malloc(length int) ([]byte, error) {
	if length < 0 || length > int(^uint(0)>>1)-s.length {
		return nil, errors.New("native header capacity overflow")
	}
	s.length += length
	return make([]byte, length), nil
}

func (s *nativeHeaderSizer) WriteBinary(data []byte) (int, error) {
	if len(data) > int(^uint(0)>>1)-s.length {
		return 0, errors.New("native header capacity overflow")
	}
	s.length += len(data)
	return len(data), nil
}

func (s *nativeHeaderSizer) WrittenLen() int { return s.length }
func (*nativeHeaderSizer) Flush() error      { return nil }

func (a HeaderAccess) nativeTTHeaderSize(ctx context.Context) (int, error) {
	if ti, ok := a.info.(*transInfo); ok && ti.primary {
		s := ti.store
		stringKind, intKind := s.kinds()
		return ttheader.EncodedEntriesSize(ttheader.HeaderEntries{Entries: s.view.entries, IntCount: s.intCount, StrCount: s.strCount, GDPRIndex: s.gdprIndex, StringKind: stringKind, IntKind: intKind})
	}
	out := new(nativeHeaderSizer)
	_, err := a.EncodeTTHeader(ctx, ttheader.EncodeParam{}, out)
	return out.WrittenLen(), err
}

func AcquireNativeTx(ctx context.Context, message Message, source interface {
	AcquireTx(context.Context) (NativeBuffer, error)
}) (NativeBuffer, error) {
	sized, ok := source.(NativeSizedTx)
	if !ok {
		return source.AcquireTx(ctx)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	method := message.RPCInfo().Invocation().MethodName()
	if method == "" {
		return nil, errors.New("native transport has no method for TX sizing")
	}
	if message.Data() == nil && !message.NewData(method) {
		return nil, errors.New("native transport has no data for TX sizing")
	}
	data := message.Data()
	if message.MessageType() == Exception {
		switch value := data.(type) {
		case *TransError:
			data = thrift.NewApplicationException(value.TypeID(), value.Error())
		case error:
			data = thrift.NewApplicationException(InternalError, value.Error())
		default:
			return nil, errors.New("native transport has invalid exception data")
		}
	}
	fast, ok := data.(thrift.FastCodec)
	if !ok {
		return nil, errors.New("native TX sizing requires FastCodec")
	}
	length := fast.BLength()
	begin := thrift.Binary.MessageBeginLength(method)
	maximum := int(^uint(0) >> 1)
	if length < 0 || length > maximum-begin {
		return nil, errors.New("native TX capacity overflow")
	}
	length += begin
	protocol := message.RPCInfo().Config().TransportProtocol()
	if protocol&transport.Framed != 0 {
		if length > maximum-4 {
			return nil, errors.New("native TX capacity overflow")
		}
		length += 4
	}
	if protocol&transport.TTHeader != 0 {
		headerLength, err := AccessHeaders(message.TransInfo()).nativeTTHeaderSize(ctx)
		if err != nil {
			return nil, err
		}
		if length > maximum-headerLength {
			return nil, errors.New("native TX capacity overflow")
		}
		length += headerLength
	}
	if uint64(length) > uint64(^uint32(0)) {
		return nil, errors.New("native TX capacity exceeds wire length")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return sized.AcquireTxSized(ctx, length)
}
