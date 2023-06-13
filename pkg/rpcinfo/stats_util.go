/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rpcinfo

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/stats"
)

// Record records the event to RPCStats.
func Record(ctx context.Context, ri RPCInfo, event stats.Event, err error) {
	if ctx == nil || ri.Stats() == nil {
		return
	}
	if err != nil {
		ri.Stats().Record(ctx, event, stats.StatusError, err.Error())
	} else {
		ri.Stats().Record(ctx, event, stats.StatusInfo, "")
	}
}

// CalcEventCostUs calculates the duration between start and end and returns in microsecond.
func CalcEventCostUs(start, end Event) uint64 {
	if start == nil || end == nil || start.IsNil() || end.IsNil() {
		return 0
	}
	return uint64(end.Time().Sub(start.Time()).Microseconds())
}

// ClientPanicToErr to transform the panic info to error, and output the error if needed.
func ClientPanicToErr(ctx context.Context, panicInfo interface{}, ri RPCInfo, logErr bool) error {
	e := fmt.Errorf("KITEX: client panic, to_service=%s to_method=%s error=%v\nstack=%s",
		ri.To().ServiceName(), ri.To().Method(), panicInfo, debug.Stack())
	rpcStats := AsMutableRPCStats(ri.Stats())
	rpcStats.SetPanicked(e)
	if logErr {
		klog.CtxErrorf(ctx, "%s", e.Error())
	}
	return e
}
