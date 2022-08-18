/*
 * Copyright 2014 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 *
 * Code forked from golang v1.17.4
 */

package grpcframe

import (
	"errors"
	"fmt"

	"golang.org/x/net/http2"
)

// connError represents an HTTP/2 ConnectionError error code, along
// with a string (for debugging) explaining why.
//
// Errors of this type are only returned by the frame parser functions
// and converted into ConnectionError(Code), after stashing away
// the Reason into the Framer's errDetail field, accessible via
// the (*Framer).ErrorDetail method.
type connError struct {
	Code   http2.ErrCode // the ConnectionError error code
	Reason string        // additional reason
}

func (e connError) Error() string {
	return fmt.Sprintf("http2: connection error: %v: %v", e.Code, e.Reason)
}

type pseudoHeaderError string

func (e pseudoHeaderError) Error() string {
	return fmt.Sprintf("invalid pseudo-header %q", string(e))
}

type duplicatePseudoHeaderError string

func (e duplicatePseudoHeaderError) Error() string {
	return fmt.Sprintf("duplicate pseudo-header %q", string(e))
}

type headerFieldNameError string

func (e headerFieldNameError) Error() string {
	return fmt.Sprintf("invalid header field name %q", string(e))
}

type headerFieldValueError string

func (e headerFieldValueError) Error() string {
	return fmt.Sprintf("invalid header field value %q", string(e))
}

var (
	errMixPseudoHeaderTypes = errors.New("mix of request and response pseudo headers")
	errPseudoAfterRegular   = errors.New("pseudo header field after regular")
)
