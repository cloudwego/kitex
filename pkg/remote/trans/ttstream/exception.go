/*
 * Copyright 2024 CloudWeGo Authors
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
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/kitex/pkg/kerrors"
)

type tException interface {
	Error() string
	TypeId() int32
}

var (
	errApplicationException = newExceptionType("application exception", nil, 12001)
	errUnexpectedHeader     = newExceptionType("unexpected header frame", kerrors.ErrStreamingProtocol, 12002)
	errIllegalBizErr        = newExceptionType("illegal bizErr", kerrors.ErrStreamingProtocol, 12003)
	errIllegalFrame         = newExceptionType("illegal frame", kerrors.ErrStreamingProtocol, 12004)
	errIllegalOperation     = newExceptionType("illegal operation", kerrors.ErrStreamingProtocol, 12005)
	errTransport            = newExceptionType("transport is closing", kerrors.ErrStreamingProtocol, 12006)

	errBizCancel = newCanceledExceptionType("user code invoking stream RPC with context processed by context.WithCancel or context.WithTimeout, then invoking cancel() actively",
		kerrors.ErrStreamingCanceled, 12007)
	errBizCancelWithCause = newCanceledExceptionType("user code canceled with cancelCause(error)", kerrors.ErrStreamingCanceled, 12008)
	errDownstreamCancel   = newCanceledExceptionType("canceled by downstream", kerrors.ErrStreamingCanceled, 12010)
	errUpstreamCancel     = newCanceledExceptionType("canceled by upstream", kerrors.ErrStreamingCanceled, 12009)
	errInternalCancel     = newCanceledExceptionType("internal canceled", kerrors.ErrStreamingCanceled, 12011)

	errBizTimeout                  = newCanceledExceptionType("user code sets timeout with context.WithTimeout or context.WithDeadline", kerrors.ErrStreamingTimeout, 12012)
	errUpstreamTransmissionTimeout = newExceptionType("timeout due to upstream configuring timeout in context", kerrors.ErrStreamingTimeout, 12013)
)

type exceptionType struct {
	side        sideType
	triggeredBy string
	message     string
	format      string

	// parent exceptionType
	basic error
	// detailed err
	cause  error
	typeId int32
}

func newExceptionType(message string, parent error, typeId int32) *exceptionType {
	return &exceptionType{message: message, basic: parent, typeId: typeId}
}

const (
	setSide = 1 << iota
	setTriggeredBy
	setCause
)

type canceledExceptionType struct {
	side sideType

	triggeredBy string
	message     string

	parent error
	cause  error

	typeId int32

	bitSet uint8
}

func newCanceledExceptionType(message string, parent error, typeId int32) *canceledExceptionType {
	return &canceledExceptionType{message: message, parent: parent, typeId: typeId}
}

func (e *canceledExceptionType) NewBuilder() *canceledExceptionType {
	newEx := *e
	return &newEx
}

func (e *canceledExceptionType) Error() string {
	var strBuilder strings.Builder
	strBuilder.WriteString(fmt.Sprintf("[ttstream error, code=%d] ", e.typeId))

	if e.isSideSet() {
		switch e.side {
		case clientTransport:
			strBuilder.WriteString("[client-side stream] ")
		case serverTransport:
			strBuilder.WriteString("[server-side stream] ")
		}
	}

	if e.isTriggeredBySet() {
		strBuilder.WriteString("[")
		strBuilder.WriteString(e.message)
		strBuilder.WriteString(" ")
		strBuilder.WriteString(e.triggeredBy)
		strBuilder.WriteString("] ")
	}

	if e.isCauseSet() {
		strBuilder.WriteString(e.cause.Error())
	} else if !e.isTriggeredBySet() {
		strBuilder.WriteString(e.message)
	}

	return strBuilder.String()
}

func (e *canceledExceptionType) WithCause(cause error) *canceledExceptionType {
	e.cause = cause
	e.bitSet |= setCause
	return e
}

func (e *canceledExceptionType) WithCauseAndTypeId(cause error, typeId int32) *canceledExceptionType {
	e.cause = cause
	e.typeId = typeId
	e.bitSet |= setCause
	return e
}

func (e *canceledExceptionType) isCauseSet() bool {
	return e.bitSet&setCause != 0
}

func (e *canceledExceptionType) WithSide(side sideType) *canceledExceptionType {
	e.side = side
	e.bitSet |= setSide
	return e
}

func (e *canceledExceptionType) isSideSet() bool {
	return e.bitSet&setSide != 0
}

func (e *canceledExceptionType) WithTriggeredBy(by string) *canceledExceptionType {
	e.triggeredBy = by
	e.bitSet |= setTriggeredBy
	return e
}

func (e *canceledExceptionType) isTriggeredBySet() bool {
	return e.bitSet&setTriggeredBy != 0
}

func (e *canceledExceptionType) Is(target error) bool {
	if rawEx, ok := target.(*canceledExceptionType); ok {
		return rawEx.message == e.message
	}
	return target == e || errors.Is(e.parent, target) || errors.Is(e.cause, target)
}

func (e *exceptionType) WithCause(err error) error {
	return &exceptionType{side: e.side, basic: e, cause: err, typeId: e.typeId}
}

func (e *exceptionType) WithSide(side sideType) *exceptionType {
	e.side = side
	return e
}

func (e *exceptionType) Error() string {
	if e.cause == nil {
		return e.message
	}
	return "[" + e.basic.Error() + "] " + e.cause.Error()
}

func (e *exceptionType) Is(target error) bool {
	return target == e || errors.Is(e.basic, target) || errors.Is(e.cause, target)
}

// TypeId is used for aligning with ApplicationException interface
func (e *exceptionType) TypeId() int32 {
	if errWithTypeId, ok := e.cause.(interface{ TypeId() int32 }); ok {
		return errWithTypeId.TypeId()
	}
	return e.typeId
}

// Code is used for uniform code retrieving by Kitex in the future
func (e *exceptionType) Code() int32 {
	return e.TypeId()
}
