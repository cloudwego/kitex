package generic

import (
	"context"

	igeneric "github.com/cloudwego/kitex/internal/generic"
)

// CodecDecorator decorates the configured message codec without rebuilding its
// descriptor, conversion options, or backend. It must return a codec supporting
// the same reader/writer interfaces. State belongs to the caller.
type CodecDecorator = igeneric.CodecDecorator

// WithCodecDecorator decorates the codec used by generic Args/Result.Read.
// It does not change the stored codec or the message's business representation.
func WithCodecDecorator(ctx context.Context, decorate CodecDecorator) context.Context {
	return igeneric.WithCodecDecorator(ctx, decorate)
}

// DecorateMessageCodec decorates the codec stored by an existing Args or Result.
// It returns false for other message types. Subsequent reads and writes use the
// decorated codec; the original generic client/server and message types remain.
func DecorateMessageCodec(message interface{}, decorate CodecDecorator) bool {
	return igeneric.DecorateMessageCodec(message, decorate)
}
