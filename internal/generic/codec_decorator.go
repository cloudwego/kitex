package generic

import (
	"context"

	"github.com/cloudwego/gopkg/protocol/thrift/base"
)

// CodecDecorator receives the codec snapshot already selected for a message.
// requestBase is the existing framework Base, if any; inspecting it does not
// allocate one. Returning the original codec leaves the message unchanged.
type CodecDecorator func(codec interface{}, requestBase *base.Base) interface{}

type codecDecoratorKey struct{}

func WithCodecDecorator(ctx context.Context, decorate CodecDecorator) context.Context {
	return context.WithValue(ctx, codecDecoratorKey{}, decorate)
}

func decoratedCodec(ctx context.Context, codec interface{}, requestBase *base.Base) interface{} {
	if decorate, _ := ctx.Value(codecDecoratorKey{}).(CodecDecorator); decorate != nil {
		return decorate(codec, requestBase)
	}
	return codec
}

func DecorateMessageCodec(message interface{}, decorate CodecDecorator) bool {
	if decorate == nil {
		return false
	}
	switch m := message.(type) {
	case *Args:
		if m == nil {
			return false
		}
		m.inner = decorate(m.inner, m.base)
	case *Result:
		if m == nil {
			return false
		}
		m.inner = decorate(m.inner, nil)
	default:
		return false
	}
	return true
}
