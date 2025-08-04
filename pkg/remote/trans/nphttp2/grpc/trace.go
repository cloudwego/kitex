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

package grpc

import (
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

func (t *http2Client) reportEvent(st *Stream, event stats.Event) {
	if t.traceCtl == nil {
		return
	}
	t.traceCtl.ReportStreamEvent(st.ctx, event, nil)
}

func (t *http2Server) reportEvent(st *Stream, event stats.Event) {
	if t.traceCtl == nil {
		return
	}
	st.traceMu.Lock()
	defer st.traceMu.Unlock()
	if st.traceFinished {
		return
	}

	if st.traceCtx == nil {
		// traceCtx is not initialized, buffer pending events
		st.pendingEvents = append(st.pendingEvents, rpcinfo.NewEvent(event, stats.StatusInfo, ""))
		return
	}

	// report pending events
	if len(st.pendingEvents) > 0 {
		t.reportPendingEventsUnlock(st)
	}
	t.traceCtl.ReportStreamEvent(st.traceCtx, event, nil)
}

func (t *http2Server) reportPendingEventsUnlock(st *Stream) {
	for i, pe := range st.pendingEvents {
		t.traceCtl.ReportStreamRawEvent(st.traceCtx, pe)
		st.pendingEvents[i] = nil
	}
	st.pendingEvents = st.pendingEvents[:0]
}
