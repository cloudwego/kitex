/*
 * Copyright 2026 CloudWeGo Authors
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

package rpcinfo

import (
	"context"
	"io"
	"time"

	"github.com/cloudwego/kitex/internal"
	"github.com/cloudwego/kitex/pkg/stats"
)

// StreamEventHandler defines a series handler for detailed Streaming Event tracing.
type StreamEventHandler struct {
	HandleStreamStartEvent      func(ctx context.Context, ri RPCInfo, evt StreamStartEvent)
	HandleStreamRecvEvent       func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent)
	HandleStreamSendEvent       func(ctx context.Context, ri RPCInfo, evt StreamSendEvent)
	HandleStreamRecvHeaderEvent func(ctx context.Context, ri RPCInfo, evt StreamRecvHeaderEvent)
	HandleStreamFinishEvent     func(ctx context.Context, ri RPCInfo, evt StreamFinishEvent)
}

// StreamStartEvent marks the beginning of a stream.
// - client side: writing the Header Frame to the underlying connection.
// - server side, receiving the Header Frame and parses all RPC-related metadata.
type StreamStartEvent struct{}

// StreamRecvEvent corresponds to Stream.Recv.
// - Err is nil: Indicating a successful receive of a deserialized Msg.
// - Err is not nil: Indicating a failure in this receive operation.
type StreamRecvEvent struct {
	Time time.Time
	Err  error
}

// StreamSendEvent corresponds to Stream.Send.
// - Err is nil: indicating successful writing of the serialized Msg to the buffer.
// - Err is not nil: indicating that the Send operation failed.
type StreamSendEvent struct {
	Time time.Time
	Err  error
}

// StreamRecvHeaderEvent indicates the reception of a header frame sent by the peer.
// - When a gRPC header frame is received, GRPCHeader is not nil.
// - when a TTHeader Streaming header frame is received, TTStreamHeader is not nil.
type StreamRecvHeaderEvent struct {
	GRPCHeader     map[string][]string
	TTStreamHeader map[string]string
}

// StreamFinishEvent indicates the end of a stream.
// - When a gRPC Trailer Frame is received, GRPCTrailer is not nil.
// - When a TTHeader Streaming Trailer Frame is received, TTStreamTrailer is not nil.
// All other cases resulting in stream termination (e.g., rst) cause both GRPCTrailer and TTStreamTrailer to be nil.
type StreamFinishEvent struct {
	GRPCTrailer     map[string][]string
	TTStreamTrailer map[string]string
}

// AppendStreamEventHandler appends a new StreamEventHandler to the controller.
// nil handler would be ignored.
func (c *TraceController) AppendStreamEventHandler(hdl StreamEventHandler) {
	if hdl.HandleStreamStartEvent != nil {
		c.streamStartEventHandlers = append(c.streamStartEventHandlers, hdl.HandleStreamStartEvent)
	}
	if hdl.HandleStreamRecvEvent != nil {
		c.streamRecvEventHandlers = append(c.streamRecvEventHandlers, hdl.HandleStreamRecvEvent)
	}
	if hdl.HandleStreamSendEvent != nil {
		c.streamSendEventHandlers = append(c.streamSendEventHandlers, hdl.HandleStreamSendEvent)
	}
	if hdl.HandleStreamRecvHeaderEvent != nil {
		c.streamRecvHeaderEventHandlers = append(c.streamRecvHeaderEventHandlers, hdl.HandleStreamRecvHeaderEvent)
	}
	if hdl.HandleStreamFinishEvent != nil {
		c.streamFinishEventHandlers = append(c.streamFinishEventHandlers, hdl.HandleStreamFinishEvent)
	}
}

func (c *TraceController) HandleStreamStartEvent(ctx context.Context, ri RPCInfo, evt StreamStartEvent) {
	if len(c.streamStartEventHandlers) == 0 {
		return
	}

	defer c.tryRecover(ctx)
	for i := 0; i < len(c.streamStartEventHandlers); i++ {
		c.streamStartEventHandlers[i](ctx, ri, evt)
	}
}

func (c *TraceController) HandleStreamRecvEvent(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
	if len(c.streamRecvEventHandlers) == 0 {
		return
	}
	if evt.Err == io.EOF {
		return
	}

	evt.Time = time.Now()
	defer c.tryRecover(ctx)
	for i := 0; i < len(c.streamRecvEventHandlers); i++ {
		c.streamRecvEventHandlers[i](ctx, ri, evt)
	}
}

func (c *TraceController) HandleStreamSendEvent(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
	if len(c.streamSendEventHandlers) == 0 {
		return
	}

	evt.Time = time.Now()
	defer c.tryRecover(ctx)
	for i := 0; i < len(c.streamSendEventHandlers); i++ {
		c.streamSendEventHandlers[i](ctx, ri, evt)
	}
}

func (c *TraceController) HandleStreamRecvHeaderEvent(ctx context.Context, ri RPCInfo, evt StreamRecvHeaderEvent) {
	if len(c.streamRecvHeaderEventHandlers) == 0 {
		return
	}

	defer c.tryRecover(ctx)
	for i := 0; i < len(c.streamRecvHeaderEventHandlers); i++ {
		c.streamRecvHeaderEventHandlers[i](ctx, ri, evt)
	}
}

func (c *TraceController) HandleStreamFinishEvent(ctx context.Context, ri RPCInfo, evt StreamFinishEvent) {
	if len(c.streamFinishEventHandlers) == 0 {
		return
	}

	defer c.tryRecover(ctx)
	for i := 0; i < len(c.streamFinishEventHandlers); i++ {
		c.streamFinishEventHandlers[i](ctx, ri, evt)
	}
}

// Compatible wrapper func with StreamEventReporter
func (c *TraceController) handleStreamRecvEventWrapper(reporter StreamEventReporter) func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
	return func(ctx context.Context, ri RPCInfo, evt StreamRecvEvent) {
		c.commonStreamIOEventWrapper(ctx, ri, reporter, stats.StreamRecv, evt.Err)
	}
}

// Compatible wrapper func with StreamEventReporter
func (c *TraceController) handleStreamSendEventWrapper(reporter StreamEventReporter) func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
	return func(ctx context.Context, ri RPCInfo, evt StreamSendEvent) {
		c.commonStreamIOEventWrapper(ctx, ri, reporter, stats.StreamSend, evt.Err)
	}
}

func (c *TraceController) commonStreamIOEventWrapper(ctx context.Context, ri RPCInfo,
	reporter StreamEventReporter, evt stats.Event, err error,
) {
	// we should ignore event if stream.RecvMsg return EOF
	// because it means there is no data incoming and stream closed by peer normally
	if err == io.EOF {
		return
	}
	defer c.tryRecover(ctx)
	event := buildStreamingEvent(evt, err)
	defer func() {
		if recyclable, ok := event.(internal.Reusable); ok {
			recyclable.Recycle()
		}
	}()
	reporter.ReportStreamEvent(ctx, ri, event)
}
