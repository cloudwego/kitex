/*
 *
 * Copyright 2020 gRPC authors.
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
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2021 CloudWeGo Authors.
 */

// Package status implements errors returned by gRPC. These errors are
// serialized and transmitted on the wire between server and client, and allow
// for additional data to be transmitted via the Details field in the status
// proto. gRPC service handlers should return an error created by this
// package, and gRPC clients should expect a corresponding error to be
// returned from the RPC call.
//
// This package upholds the invariants that a non-nil error may not
// contain an OK code, and an OK code must result in a nil error.
package status

import (
	"context"
	"errors"
	"fmt"

	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
)

type Iface interface {
	GRPCStatus() *Status
}

// Source identifies, from the current process's point of view, the boundary
// through which a terminal status originated: produced by the process itself
// (SourceLocal), received on a connection the process accepted (SourceInbound),
// or received on a connection the process initiated (SourceOutbound).
//
// Source is process-local observability metadata: it is never transmitted on
// the wire, so each process derives its own attribution at the point where a
// terminal signal first enters it (or is produced locally). Within the
// process, the attribution is deliberately preserved while the signal
// cascades across RPC legs. E.g. when a proxy cancels its outbound call
// because the caller of the inbound RPC driving it terminated, the outbound
// stream surfaces a SourceInbound status, telling the process that the
// termination originated from its inbound side rather than from the callee.
//
// Source reflects neither the cross-process root cause of a failure nor a
// trusted identity, so it must not be used for authorization decisions.
type Source uint8

const (
	// SourceLocal means the current process produced the terminal status, e.g. a
	// local context cancel/deadline, admission check, or framework-local error.
	SourceLocal Source = iota
	// SourceInbound means the terminal signal was explicitly sent by the peer of
	// an RPC accepted by the current process, e.g. the caller sent RST_STREAM.
	SourceInbound
	// SourceOutbound means the termination was returned by the peer of an RPC
	// initiated by the current process, e.g. a gRPC status carried by
	// Headers/Trailers or an RST_STREAM sent by the callee.
	SourceOutbound
)

// String returns a stable low-cardinality label suitable for metrics.
func (s Source) String() string {
	switch s {
	case SourceInbound:
		return "inbound"
	case SourceOutbound:
		return "outbound"
	default:
		return "local"
	}
}

// Status represents an RPC status code, message, and details.  It is immutable
// and should be created with New, Newf, NewWithSource, FromProto, or
// FromProtoWithSource.
type Status struct {
	s *spb.Status
	// source is intentionally excluded from Proto() and never reaches the wire.
	source Source
}

// New returns a Status representing c and msg.
func New(c codes.Code, msg string) *Status {
	return NewWithSource(c, msg, SourceLocal)
}

// NewWithSource returns a Status representing c and msg with source as its
// process-local source attribution.
func NewWithSource(c codes.Code, msg string, source Source) *Status {
	return &Status{
		s:      &spb.Status{Code: int32(c), Message: msg},
		source: normalizeSource(source),
	}
}

// Newf returns New(c, fmt.Sprintf(format, a...)).
func Newf(c codes.Code, format string, a ...interface{}) *Status {
	return New(c, fmt.Sprintf(format, a...))
}

// ErrorProto returns an error representing s.  If s.Code is OK, returns nil.
func ErrorProto(s *spb.Status) error {
	return FromProto(s).Err()
}

// FromProto returns a Status representing s.
func FromProto(s *spb.Status) *Status {
	return FromProtoWithSource(s, SourceLocal)
}

// FromProtoWithSource returns a Status representing s with source as its
// process-local source attribution.
func FromProtoWithSource(s *spb.Status, source Source) *Status {
	return newStatusFromProto(s, source)
}

func newStatusFromProto(s *spb.Status, source Source) *Status {
	return &Status{
		s:      proto.Clone(s).(*spb.Status),
		source: normalizeSource(source),
	}
}

// Err returns an error representing c and msg.  If c is OK, returns nil.
func Err(c codes.Code, msg string) error {
	return New(c, msg).Err()
}

// Errorf returns Error(c, fmt.Sprintf(format, a...)).
func Errorf(c codes.Code, format string, a ...interface{}) error {
	return Err(c, fmt.Sprintf(format, a...))
}

// Code returns the status code contained in s.
func (s *Status) Code() codes.Code {
	if s == nil || s.s == nil {
		return codes.OK
	}
	return codes.Code(s.s.Code)
}

// Message returns the message contained in s.
func (s *Status) Message() string {
	if s == nil || s.s == nil {
		return ""
	}
	return s.s.Message
}

// Source returns the process-local source attribution of s. It returns
// SourceLocal when s is nil or its source is invalid.
func (s *Status) Source() Source {
	if s == nil {
		return SourceLocal
	}
	return normalizeSource(s.source)
}

// WithSource returns a copy of s with the process-local source
// attribution set to source. The returned Source is excluded from Proto() and
// is therefore never transmitted on the wire.
func (s *Status) WithSource(source Source) *Status {
	if s == nil {
		return nil
	}
	ns := s.Copy()
	ns.source = normalizeSource(source)
	return ns
}

func normalizeSource(source Source) Source {
	switch source {
	case SourceInbound, SourceOutbound:
		return source
	default:
		return SourceLocal
	}
}

// Copy returns a deep copy of s, preserving Source.
// Note that copying via Proto()/FromProto() drops Source by design.
func (s *Status) Copy() *Status {
	if s == nil {
		return nil
	}
	return newStatusFromProto(s.s, s.Source())
}

// AppendMessage append extra msg for Status
func (s *Status) AppendMessage(extraMsg string) *Status {
	if s == nil || s.s == nil || extraMsg == "" {
		return s
	}
	s.s.Message = fmt.Sprintf("%s %s", s.s.Message, extraMsg)
	return s
}

// Proto returns s's status as an spb.Status proto message.
func (s *Status) Proto() *spb.Status {
	if s == nil {
		return nil
	}
	return proto.Clone(s.s).(*spb.Status)
}

// Err returns an immutable error representing s; returns nil if s.Code() is OK.
func (s *Status) Err() error {
	if s.Code() == codes.OK {
		return nil
	}
	return &Error{e: s.Proto(), source: s.Source()}
}

// WithDetails returns a new status with the provided details messages appended to the status.
// If any errors are encountered, it returns nil and the first error encountered.
func (s *Status) WithDetails(details ...proto.Message) (*Status, error) {
	if s.Code() == codes.OK {
		return nil, errors.New("no error details for status with code OK")
	}
	// s.Code() != OK implies that s.Proto() != nil.
	p := s.Proto()
	for _, detail := range details {
		any, err := anypb.New(detail)
		if err != nil {
			return nil, err
		}
		p.Details = append(p.Details, any)
	}
	// p is already a private deep copy returned by Proto(), so constructing the
	// result directly avoids cloning the entire status a second time.
	return &Status{s: p, source: s.Source()}, nil
}

// Details returns a slice of details messages attached to the status.
// If a detail cannot be decoded, the error is returned in place of the detail.
func (s *Status) Details() []interface{} {
	if s == nil || s.s == nil {
		return nil
	}
	details := make([]interface{}, 0, len(s.s.Details))
	for _, any := range s.s.Details {
		detail, err := any.UnmarshalNew()
		if err != nil {
			details = append(details, err)
			continue
		}
		details = append(details, detail)
	}
	return details
}

// Error wraps a pointer of a status proto. It implements error and Status,
// and a nil *Error should never be returned by this package.
type Error struct {
	e      *spb.Status
	source Source
}

func (e *Error) Error() string {
	return fmt.Sprintf("rpc error: code = %d desc = %s", codes.Code(e.e.GetCode()), e.e.GetMessage())
}

// GRPCStatus returns the Status represented by se.
func (e *Error) GRPCStatus() *Status {
	return newStatusFromProto(e.e, e.source)
}

// Is implements future error.Is functionality.
// A Error is equivalent if the code and message are identical.
func (e *Error) Is(target error) bool {
	tse, ok := target.(*Error)
	if !ok {
		return false
	}
	return proto.Equal(e.e, tse.e)
}

// FromError returns a Status representing err if it was produced from this
// package or has a method `GRPCStatus() *Status`. Otherwise, ok is false and a
// Status is returned with codes.Unknown and the original error message.
func FromError(err error) (s *Status, ok bool) {
	if err == nil {
		return nil, true
	}
	var se Iface
	if errors.As(err, &se) {
		return se.GRPCStatus(), true
	}
	return New(codes.Unknown, err.Error()), false
}

// Convert is a convenience function which removes the need to handle the
// boolean return value from FromError.
func Convert(err error) *Status {
	s, _ := FromError(err)
	return s
}

// Code returns the Code of the error if it is a Status error, codes.OK if err
// is nil, or codes.Unknown otherwise.
func Code(err error) codes.Code {
	// Don't use FromError to avoid allocation of OK status.
	if err == nil {
		return codes.OK
	}
	var se Iface
	if errors.As(err, &se) {
		return se.GRPCStatus().Code()
	}
	return codes.Unknown
}

// FromContextError converts a context error into a Status.  It returns a
// Status with codes.OK if err is nil, or a Status with codes.Unknown if err is
// non-nil and not a context error.
func FromContextError(err error) *Status {
	switch err {
	case nil:
		return nil
	case context.DeadlineExceeded:
		return New(codes.DeadlineExceeded, err.Error())
	case context.Canceled:
		return New(codes.Canceled, err.Error())
	default:
		return New(codes.Unknown, err.Error())
	}
}
