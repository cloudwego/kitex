// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package netpoll

import (
	"fmt"
	"syscall"
)

// extends syscall.Errno, the range is set to 0x100-0x1FF
const (
	// The connection closed when in use.
	ErrConnClosed = syscall.Errno(0x101)
	// read I/O buffer timeout, calling by Connection.Reader
	ErrReadTimeout = syscall.Errno(0x102)
	// dial timeout
	ErrDialTimeout = syscall.Errno(0x103)
	// Calling dialer without timeout.
	ErrDialNoDeadline = syscall.Errno(0x104) // TODO: no-deadline support in future
	// The calling function not support.
	ErrUnsupported = syscall.Errno(0x105)
	// Same as io.EOF
	ErrEOF = syscall.Errno(0x106)
)

const ErrnoMask = 0xFF

// wrap Errno, implement xerrors.Wrapper
func Exception(err error, suffix string) error {
	var no, ok = err.(syscall.Errno)
	if !ok {
		if suffix == "" {
			return err
		}
		return fmt.Errorf("%s %s", err.Error(), suffix)
	}
	return &exception{no: no, suffix: suffix}
}

type exception struct {
	no     syscall.Errno
	suffix string
}

func (e *exception) Error() string {
	var s string
	if int(e.no)&0x100 != 0 {
		s = errnos[int(e.no)&ErrnoMask]
	}
	if s == "" {
		s = e.no.Error()
	}
	if e.suffix != "" {
		s += " " + e.suffix
	}
	return s
}

func (e *exception) Is(target error) bool {
	if e == target {
		return true
	}
	if e.no == target {
		return true
	}
	// TODO: ErrConnClosed contains ErrEOF
	if e.no == ErrEOF && target == ErrConnClosed {
		return true
	}
	return e.no.Is(target)
}

func (e *exception) Unwrap() error {
	return e.no
}

// Errors defined in netpoll
var errnos = [...]string{
	ErrnoMask & ErrConnClosed:     "connection has been closed",
	ErrnoMask & ErrReadTimeout:    "connection read timeout",
	ErrnoMask & ErrDialTimeout:    "dial wait timeout",
	ErrnoMask & ErrDialNoDeadline: "dial no deadline",
	ErrnoMask & ErrUnsupported:    "netpoll dose not support",
	ErrnoMask & ErrEOF:            "EOF",
}
