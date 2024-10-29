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

package errors

import (
	"errors"
)

var (
	ErrUnexpectedHeader     = &errType{message: "unexpected header frame"}
	ErrUnexpectedTrailer    = &errType{message: "unexpected trailer frame"}
	ErrApplicationException = &errType{message: "application exception"}
	ErrIllegalBizErr        = &errType{message: "illegal bizErr"}
	ErrIllegalFrame         = &errType{message: "illegal frame"}
	ErrTransport            = &errType{message: "transport is closing"}
)

type errType struct {
	message string
	basic   error
	cause   error
}

func (e *errType) WithCause(err error) error {
	return &errType{message: e.message, basic: e, cause: err}
}

func (e *errType) Error() string {
	if e.cause == nil {
		return e.message
	}
	return "[" + e.message + "] " + e.cause.Error()
}

func (e *errType) Is(target error) bool {
	return target == e || target == e.basic || errors.Is(e.cause, target)
}
