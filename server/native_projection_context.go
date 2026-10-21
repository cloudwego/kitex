package server

import (
	"context"
	"sync"
	"time"
)

type nativeProjectedContext struct {
	context.Context
	deadline    time.Time
	hasDeadline bool
	done        chan struct{}
	mu          sync.Mutex
	err         error
	cancelCause context.CancelCauseFunc
	callbacks   map[*nativeProjectionCallback]struct{}
}

type nativeProjectionCallback struct {
	once sync.Once
	fn   func()
}

func (c *nativeProjectedContext) Deadline() (time.Time, bool) { return c.deadline, c.hasDeadline }
func (c *nativeProjectedContext) Done() <-chan struct{}       { return c.done }
func (c *nativeProjectedContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}
func (c *nativeProjectedContext) cancel(err, cause error) {
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	c.err = err
	c.cancelCause(cause)
	close(c.done)
	callbacks := c.callbacks
	c.callbacks = nil
	c.mu.Unlock()
	for callback := range callbacks {
		callback.once.Do(func() { go callback.fn() })
	}
}
func (c *nativeProjectedContext) AfterFunc(fn func()) func() bool {
	callback := &nativeProjectionCallback{fn: fn}
	c.mu.Lock()
	if c.err == nil {
		if c.callbacks == nil {
			c.callbacks = make(map[*nativeProjectionCallback]struct{})
		}
		c.callbacks[callback] = struct{}{}
		c.mu.Unlock()
	} else {
		c.mu.Unlock()
		callback.once.Do(func() { go callback.fn() })
	}
	return func() bool {
		stopped := false
		callback.once.Do(func() { stopped = true })
		if stopped {
			c.mu.Lock()
			delete(c.callbacks, callback)
			c.mu.Unlock()
		}
		return stopped
	}
}

func nativeProjectCancellation(projected, decoded context.Context) (context.Context, func(), context.CancelFunc) {
	values, releaseValues := nativeCompleteContext(projected)
	causeContext, cancelCause := context.WithCancelCause(values)
	ctx := &nativeProjectedContext{Context: causeContext, done: make(chan struct{}), cancelCause: cancelCause}
	ctx.deadline, ctx.hasDeadline = projected.Deadline()
	if deadline, ok := decoded.Deadline(); ok && (!ctx.hasDeadline || deadline.Before(ctx.deadline)) {
		ctx.deadline, ctx.hasDeadline = deadline, true
	}
	type subscription struct {
		stop func() bool
		done chan struct{}
	}
	var subscriptions [2]subscription
	for i, source := range []context.Context{projected, decoded} {
		if err := source.Err(); err != nil {
			ctx.cancel(err, context.Cause(source))
			continue
		}
		if source.Done() == nil {
			continue
		}
		source := source
		done := make(chan struct{})
		stop := nativeAfterCancel(source, func() {
			defer close(done)
			ctx.cancel(source.Err(), context.Cause(source))
		})
		subscriptions[i] = subscription{stop: stop, done: done}
	}
	return ctx, func() {
			for _, subscription := range subscriptions {
				if subscription.stop != nil && !subscription.stop() {
					<-subscription.done
				}
			}
		}, func() {
			ctx.cancel(context.Canceled, context.Canceled)
			if releaseValues != nil {
				releaseValues()
			}
		}
}
