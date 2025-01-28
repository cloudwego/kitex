package streamxclient

import "context"

// StreamCloseCallback define close callback of the stream
type StreamCloseCallback func()

// CallOptions define stream call level options
type CallOptions struct {
	StreamCloseCallback []StreamCloseCallback
}

// CallOption define stream call level option
type CallOption struct {
	f func(o *CallOptions)
}

// WithCallOption add call option
type WithCallOption func(o *CallOption)

type ctxKeyCallOptions struct{}

// NewCtxWithCallOptions register CallOptions into context
func NewCtxWithCallOptions(ctx context.Context) (context.Context, *CallOptions) {
	copts := new(CallOptions)
	return context.WithValue(ctx, ctxKeyCallOptions{}, copts), copts
}

// GetCallOptionsFromCtx get CallOptions from context
func GetCallOptionsFromCtx(ctx context.Context) *CallOptions {
	v := ctx.Value(ctxKeyCallOptions{})
	if v == nil {
		return nil
	}
	copts, ok := v.(*CallOptions)
	if !ok {
		return nil
	}
	return copts
}

// Apply call options
func (copts *CallOptions) Apply(opts []CallOption) {
	for _, opt := range opts {
		opt.f(copts)
	}
}

// WithStreamCloseCallback register StreamCloseCallback
func WithStreamCloseCallback(callback StreamCloseCallback) CallOption {
	return CallOption{f: func(o *CallOptions) {
		o.StreamCloseCallback = append(o.StreamCloseCallback, callback)
	}}
}
