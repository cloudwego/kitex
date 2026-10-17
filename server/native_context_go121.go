//go:build go1.21

package server

import "context"

func nativeAfterCancel(ctx context.Context, cancel func()) func() bool {
	return context.AfterFunc(ctx, cancel)
}

func nativeCompleteContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithoutCancel(ctx), nil
}
