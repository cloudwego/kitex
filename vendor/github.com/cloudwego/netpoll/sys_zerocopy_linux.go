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
	"syscall"
)

const (
	SO_ZEROCOPY       = 60
	SO_ZEROBLOCKTIMEO = 69
	MSG_ZEROCOPY      = 0x4000000
)

func setZeroCopy(fd int) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, SO_ZEROCOPY, 1)
}

func setBlockZeroCopySend(fd int, sec, usec int64) error {
	return syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, SO_ZEROBLOCKTIMEO, &syscall.Timeval{
		Sec:  sec,
		Usec: usec,
	})
}
