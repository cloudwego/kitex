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
	Message() string
	TriggeredBy() string
}

var (
	errApplicationException = newException("application exception", nil, 12001)
	errUnexpectedHeader     = newException("unexpected header frame", kerrors.ErrStreamingProtocol, 12002)
	errIllegalBizErr        = newException("illegal bizErr", kerrors.ErrStreamingProtocol, 12003)
	errIllegalFrame         = newException("illegal frame", kerrors.ErrStreamingProtocol, 12004)
	errIllegalOperation     = newException("illegal operation", kerrors.ErrStreamingProtocol, 12005)
	errTransport            = newException("transport is closing", kerrors.ErrStreamingProtocol, 12006)

	errBizCancel = newException("user code invoking stream RPC with context processed by context.WithCancel or context.WithTimeout, then invoking cancel() actively",
		kerrors.ErrStreamingCanceled, 12007)
	// todo: support cancelWithCause
	// errBizCancelWithCause = newCanceledExceptionType("user code canceled with cancelCause(error)", kerrors.ErrStreamingCanceled, 12008)
	errDownstreamCancel = newException("canceled by downstream", kerrors.ErrStreamingCanceled, 12010)
	errUpstreamCancel   = newException("canceled by upstream", kerrors.ErrStreamingCanceled, 12009)
	errInternalCancel   = newException("internal canceled", kerrors.ErrStreamingCanceled, 12011)
)

type exceptionType struct {
	message string
	// parent exceptionType
	basic error
	// detailed err
	cause  error
	typeId int32
}

func newExceptionType(message string, parent error, typeId int32) *exceptionType {
	return &exceptionType{message: message, basic: parent, typeId: typeId}
}

func (e *exceptionType) WithCause(err error) error {
	return &exceptionType{basic: e, cause: err, typeId: e.typeId}
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

func (e *exceptionType) Message() string {
	return e.Error()
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

func (e *exceptionType) TriggeredBy() string {
	return ""
}

const (
	setSide = 1 << iota
	setTriggeredBy
	setCause
)

type Exception struct {
	message string
	typeId  int32

	side sideType
	via  string

	parent error
	cause  error

	bitSet uint8
}

func newException(message string, parent error, typeId int32) *Exception {
	return &Exception{message: message, parent: parent, typeId: typeId}
}

func (e *Exception) newBuilder() *Exception {
	newEx := *e
	return &newEx
}

func (e *Exception) Error() string {
	var strBuilder strings.Builder
	strBuilder.WriteString(fmt.Sprintf("[ttstream error, code=%d] ", e.typeId))

	if e.isSideSet() {
		switch e.side {
		case clientSide:
			strBuilder.WriteString("[client-side stream] ")
		case serverSide:
			strBuilder.WriteString("[server-side stream] ")
		}
	}

	if e.isViaSet() {
		strBuilder.WriteString("[canceled ")
		if e.viaMultipleNodes() {
			strBuilder.WriteString("link: ")
		} else {
			strBuilder.WriteString(" by ")
		}
		strBuilder.WriteString(e.via)
		strBuilder.WriteString("] ")
	}

	if e.isCauseSet() {
		strBuilder.WriteString(e.cause.Error())
	} else {
		strBuilder.WriteString(e.message)
	}

	return strBuilder.String()
}

func (e *Exception) withCause(cause error) *Exception {
	e.cause = cause
	e.bitSet |= setCause
	return e
}

func (e *Exception) withCauseAndTypeId(cause error, typeId int32) *Exception {
	e.cause = cause
	e.typeId = typeId
	e.bitSet |= setCause
	return e
}

func (e *Exception) isCauseSet() bool {
	return e.bitSet&setCause != 0
}

func (e *Exception) withSide(side sideType) *Exception {
	e.side = side
	e.bitSet |= setSide
	return e
}

func (e *Exception) isSideSet() bool {
	return e.bitSet&setSide != 0
}

func (e *Exception) setOrAppendVia(via string) *Exception {
	if len(e.via) > 0 {
		e.via = e.via + "," + via
	} else {
		e.via = via
	}
	e.bitSet |= setTriggeredBy
	return e
}

func (e *Exception) isViaSet() bool {
	return e.bitSet&setTriggeredBy != 0
}

func (e *Exception) viaMultipleNodes() bool {
	if strings.Contains(e.via, ",") {
		return true
	}
	return false
}

func (e *Exception) Is(target error) bool {
	if rawEx, ok := target.(*Exception); ok {
		return rawEx.message == e.message
	}
	return target == e || errors.Is(e.parent, target) || errors.Is(e.cause, target)
}

func (e *Exception) Message() string {
	if e.isCauseSet() {
		return e.cause.Error()
	}
	return e.message
}

func (e *Exception) TypeId() int32 {
	return e.typeId
}
