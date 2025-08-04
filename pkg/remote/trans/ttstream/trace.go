/*
 * Copyright 2025 CloudWeGo Authors
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

package ttstream

import (
	"context"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

func (s *stream) setTraceController(ctl *rpcinfo.TraceController) {
	s.traceCtl = ctl
}

func (s *stream) setTraceContext(ctx context.Context) {
	s.traceMu.Lock()
	s.traceCtx = ctx
	s.traceMu.Unlock()
}

func (s *stream) finishTrace() {
	s.traceMu.Lock()
	if len(s.pendingEvents) > 0 {
		s.reportPendingEventsUnlock()
	}
	s.traceFinished = true
	s.traceMu.Unlock()
}

func (s *stream) reportEvent(event stats.Event) {
	if s.traceCtl == nil {
		return
	}
	s.traceMu.Lock()
	defer s.traceMu.Unlock()
	if s.traceFinished {
		return
	}

	if s.traceCtx == nil {
		// traceCtx is not initialized, buffer pending events
		s.pendingEvents = append(s.pendingEvents, rpcinfo.NewEvent(event, stats.StatusInfo, ""))
		return
	}

	// report pending events
	if len(s.pendingEvents) > 0 {
		s.reportPendingEventsUnlock()
	}
	s.traceCtl.ReportStreamEvent(s.traceCtx, event, nil)
}

func (s *stream) reportPendingEventsUnlock() {
	for i, e := range s.pendingEvents {
		s.traceCtl.ReportStreamRawEvent(s.traceCtx, e)
		s.pendingEvents[i] = nil
	}
	s.pendingEvents = s.pendingEvents[:0]
}
