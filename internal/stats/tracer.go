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

package stats

import (
	"context"
	"runtime/debug"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

// Controller controls tracers.
type Controller struct {
	tracers []stats.Tracer
}

// Append appends a new tracer to the controller.
func (c *Controller) Append(col stats.Tracer) {
	c.tracers = append(c.tracers, col)
}

// DoStart starts the tracers.
func (c *Controller) DoStart(ctx context.Context, ri rpcinfo.RPCInfo) context.Context {
	defer c.tryRecover(ctx)
	Record(ctx, ri, stats.RPCStart, nil)

	for _, col := range c.tracers {
		ctx = col.Start(ctx)
	}
	return ctx
}

// DoFinish calls the tracers in reversed order.
func (c *Controller) DoFinish(ctx context.Context, ri rpcinfo.RPCInfo, err error) {
	defer c.tryRecover(ctx)
	Record(ctx, ri, stats.RPCFinish, err)
	if err != nil {
		rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats())
		rpcStats.SetError(err)
	}

	// reverse the order
	for i := len(c.tracers) - 1; i >= 0; i-- {
		c.tracers[i].Finish(ctx)
	}
}

func (c *Controller) HasTracer() bool {
	return c != nil && len(c.tracers) > 0
}

func (c *Controller) tryRecover(ctx context.Context) {
	if err := recover(); err != nil {
		klog.CtxWarnf(ctx, "Panic happened during tracer call. This doesn't affect the rpc call, but may lead to lack of monitor data such as metrics and logs: %s, %s", err, string(debug.Stack()))
	}
}
