/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
)

const (
	diagnosticActiveLimit = 32
	diagnosticRecentLimit = 8
	diagnosticEventLimit  = 8
	diagnosticStringLimit = 256
)

var (
	diagnosticSequence uint64
	diagnosticPrefix   = func() string {
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err == nil {
			return hex.EncodeToString(nonce[:])
		}
		return fmt.Sprintf("%x-%x", time.Now().UnixNano(), os.Getpid())
	}()
)

// All mutable fields except lastFrameAt are protected by http2Server.mu.
// History contains values only: never retain requests, contexts or pooled RPCInfo.
type serverConnDiagnostics struct {
	id          string
	createdAt   time.Time
	lastFrameAt int64 // atomic; successful reads only
	recent      []streamDiagnostic
	events      []connectionDiagnosticEvent
	close       *connectionCloseDiagnostic
}

type serverStreamDiagnosticKey struct{}

type serverStreamDiagnostics struct {
	mu   sync.Mutex
	info streamDiagnostic
	ctx  context.Context // immutable cancellation context, before traceCtx replaces s.ctx
}

type streamDiagnostic struct {
	StreamID             uint32 `json:"stream_id"`
	SourceService        string `json:"source_service"`
	Method               string `json:"method"`
	LogID                string `json:"logid,omitempty"`
	LogIDSource          string `json:"logid_source,omitempty"`
	IncomingLogIDPresent bool   `json:"incoming_logid_present"`
	MetadataRead         bool   `json:"metadata_read"`
	StartedAtMS          int64  `json:"started_at_ms"`
	ElapsedMS            int64  `json:"elapsed_ms"`
	DeadlineMS           int64  `json:"deadline_ms,omitempty"`
	EndedAtMS            int64  `json:"ended_at_ms,omitempty"`
	Canceled             bool   `json:"canceled"`
	State                uint32 `json:"state"`
}

type connectionDiagnosticEvent struct {
	AtMS         int64  `json:"at_ms"`
	Kind         string `json:"kind"`
	Direction    string `json:"direction,omitempty"`
	Code         string `json:"code,omitempty"`
	StreamID     uint32 `json:"stream_id,omitempty"`
	LastStreamID uint32 `json:"last_stream_id,omitempty"`
	Detail       string `json:"detail,omitempty"`
	DebugDataLen int    `json:"debug_data_len,omitempty"`
}

type connectionCloseDiagnostic struct {
	ConnID               string                      `json:"conn_id"`
	AtMS                 int64                       `json:"at_ms"`
	Stage                string                      `json:"close_stage"`
	Reason               string                      `json:"reason"`
	RawError             string                      `json:"raw_error,omitempty"`
	RawErrorType         string                      `json:"raw_error_type,omitempty"`
	Network              string                      `json:"network"`
	LocalAddr            string                      `json:"local_addr"`
	RemoteAddr           string                      `json:"remote_addr"`
	StateBeforeClose     string                      `json:"state_before_close"`
	LocalDrainStarted    bool                        `json:"local_drain_started"`
	ConnectionAgeMS      int64                       `json:"connection_age_ms"`
	IdleMS               int64                       `json:"idle_ms"`
	LastFrameAgoMS       int64                       `json:"last_successful_frame_ago_ms"`
	ActiveStreamCount    int                         `json:"active_stream_count"`
	ActiveStreamsOmitted int                         `json:"active_streams_omitted"`
	ActiveStreams        []streamDiagnostic          `json:"active_streams"`
	RecentFinished       []streamDiagnostic          `json:"recent_finished_streams"`
	RecentEvents         []connectionDiagnosticEvent `json:"recent_events"`
}

func newServerConnDiagnostics(createdAt time.Time) *serverConnDiagnostics {
	return &serverConnDiagnostics{
		id:          fmt.Sprintf("%s-%x", diagnosticPrefix, atomic.AddUint64(&diagnosticSequence, 1)),
		createdAt:   createdAt,
		lastFrameAt: time.Now().UnixNano(),
	}
}

// RecordServerStreamLogID lets a metadata handler attach the effective request
// log ID to HTTP/2 diagnostics. source describes whether the ID was received,
// generated, or already present in the context. It is a no-op when disabled.
// Only trace metadata belongs here; never pass request bodies or credentials.
func RecordServerStreamLogID(ctx context.Context, logID, source string, incomingPresent bool) {
	d, _ := ctx.Value(serverStreamDiagnosticKey{}).(*serverStreamDiagnostics)
	if d == nil {
		return
	}
	d.mu.Lock()
	d.info.LogID = diagnosticString(logID)
	d.info.LogIDSource = diagnosticString(source)
	d.info.IncomingLogIDPresent = incomingPresent
	d.info.MetadataRead = true
	d.mu.Unlock()
}

func (t *http2Server) initStreamDiagnostics(s *Stream) {
	if t.diagnostics == nil {
		return
	}
	d := &serverStreamDiagnostics{
		ctx: s.ctx,
		info: streamDiagnostic{
			StreamID: s.id, SourceService: diagnosticString(s.sourceService),
			Method: diagnosticString(s.method), StartedAtMS: time.Now().UnixMilli(),
		},
	}
	if deadline, ok := s.ctx.Deadline(); ok {
		d.info.DeadlineMS = deadline.UnixMilli()
	}
	s.diagnostics = d
	s.ctx = context.WithValue(s.ctx, serverStreamDiagnosticKey{}, d)
}

func (s *Stream) diagnosticSnapshot(now time.Time) streamDiagnostic {
	if s.diagnostics == nil {
		return streamDiagnostic{StreamID: s.id}
	}
	d := s.diagnostics
	d.mu.Lock()
	info := d.info
	d.mu.Unlock()
	info.ElapsedMS = now.UnixMilli() - info.StartedAtMS
	info.Canceled = d.ctx.Err() != nil
	info.State = uint32(s.getState())
	return info
}

func diagnosticString(s string) string {
	if len(s) > diagnosticStringLimit {
		s = s[:diagnosticStringLimit]
	}
	// Clone so a short field cannot retain a large decoded header buffer.
	return strings.Clone(s)
}

func diagnosticAddr(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return diagnosticString(addr.String())
}

// Called under t.mu, before clearing activeStreams or canceling its contexts.
// Only the first close observation wins, including writer-initiated closes.
func (t *http2Server) captureDiagnosticCloseLocked(stage string, reason, raw error) *connectionCloseDiagnostic {
	d := t.diagnostics
	if d == nil || d.close != nil {
		return nil
	}
	now := time.Now()
	state := "unknown"
	switch t.state {
	case reachable:
		state = "reachable"
	case draining:
		state = "draining"
	case closing:
		state = "closing"
	}
	v := &connectionCloseDiagnostic{
		ConnID: d.id, AtMS: now.UnixMilli(), Stage: stage,
		LocalAddr: diagnosticAddr(t.localAddr), RemoteAddr: diagnosticAddr(t.remoteAddr),
		StateBeforeClose: state, LocalDrainStarted: t.drainChan != nil,
		ConnectionAgeMS:   now.Sub(d.createdAt).Milliseconds(),
		LastFrameAgoMS:    now.Sub(time.Unix(0, atomic.LoadInt64(&d.lastFrameAt))).Milliseconds(),
		ActiveStreamCount: len(t.activeStreams), ActiveStreams: []streamDiagnostic{},
		RecentFinished: append([]streamDiagnostic{}, d.recent...),
		RecentEvents:   append([]connectionDiagnosticEvent{}, d.events...),
	}
	if t.remoteAddr != nil {
		v.Network = t.remoteAddr.Network()
	}
	if !t.idle.IsZero() {
		v.IdleMS = now.Sub(t.idle).Milliseconds()
	}
	if reason != nil {
		v.Reason = diagnosticString(reason.Error())
	}
	if raw != nil {
		v.RawError, v.RawErrorType = diagnosticString(raw.Error()), fmt.Sprintf("%T", raw)
	}
	for _, s := range t.activeStreams {
		if len(v.ActiveStreams) == diagnosticActiveLimit {
			break
		}
		v.ActiveStreams = append(v.ActiveStreams, s.diagnosticSnapshot(now))
	}
	sort.Slice(v.ActiveStreams, func(i, j int) bool { return v.ActiveStreams[i].StreamID < v.ActiveStreams[j].StreamID })
	v.ActiveStreamsOmitted = v.ActiveStreamCount - len(v.ActiveStreams)
	d.close = v
	return v
}

func (t *http2Server) logDiagnosticClose(v *connectionCloseDiagnostic) {
	if v != nil {
		b, _ := json.Marshal(v) // only primitive fields, no user-defined marshalers
		klog.CtxInfof(t.ctx, "KITEX: grpc server connection diagnostic, diagnostic=%s", b)
	}
}

func (t *http2Server) rememberFinishedStreamLocked(s *Stream) {
	d := t.diagnostics
	if d == nil {
		return
	}
	info := s.diagnosticSnapshot(time.Now())
	info.EndedAtMS = time.Now().UnixMilli()
	if len(d.recent) == diagnosticRecentLimit {
		copy(d.recent, d.recent[1:])
		d.recent = d.recent[:diagnosticRecentLimit-1]
	}
	d.recent = append(d.recent, info)
}

func (t *http2Server) recordDiagnosticEvent(event connectionDiagnosticEvent) {
	if t.diagnostics == nil {
		return
	}
	event.AtMS = time.Now().UnixMilli()
	event.Detail = diagnosticString(event.Detail)
	t.mu.Lock()
	d := t.diagnostics
	if len(d.events) == diagnosticEventLimit {
		copy(d.events, d.events[1:])
		d.events = d.events[:diagnosticEventLimit-1]
	}
	d.events = append(d.events, event)
	var stream *streamDiagnostic
	if s := t.activeStreams[event.StreamID]; s != nil && event.Kind == "rst_stream" {
		info := s.diagnosticSnapshot(time.Now())
		stream = &info
	}
	t.mu.Unlock()
	b, _ := json.Marshal(struct {
		ConnID string `json:"conn_id"`
		connectionDiagnosticEvent
		Stream *streamDiagnostic `json:"stream,omitempty"`
	}{d.id, event, stream})
	klog.CtxInfof(t.ctx, "KITEX: grpc server connection event, diagnostic=%s", b)
}
