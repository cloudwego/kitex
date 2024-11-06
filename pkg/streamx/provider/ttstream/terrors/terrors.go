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

package terrors

import (
	"errors"

	"github.com/cloudwego/kitex/pkg/kerrors"
)

// terrors define TTHeader Streaming-related protocol errors, they all inherit from ErrStreamingProtocol in kerrors.
var (
	ErrUnexpectedHeader     = newErrType("unexpected header frame")
	ErrApplicationException = newErrType("application exception")
	ErrIllegalBizErr        = newErrType("illegal bizErr")
	ErrIllegalFrame         = newErrType("illegal frame")
	ErrIllegalOperation     = newErrType("illegal operation")
	ErrTransport            = newErrType("transport is closing")
)

type errType struct {
	message string
	// parent errType
	basic error
	// detailed err
	cause error
}

func newErrType(message string) *errType {
	return &errType{message: message, basic: kerrors.ErrStreamingProtocol}
}

func (e *errType) WithCause(err error) error {
	return &errType{basic: e, cause: err}
}

func (e *errType) Error() string {
	if e.cause == nil {
		return e.message
	}
	return "[" + e.basic.Error() + "] " + e.cause.Error()
}

func (e *errType) Is(target error) bool {
	return target == e || errors.Is(e.basic, target) || errors.Is(e.cause, target)
}
