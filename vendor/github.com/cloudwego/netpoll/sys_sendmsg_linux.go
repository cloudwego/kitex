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
	"unsafe"
)

//func init() {
//	err := syscall.Setrlimit(8, &syscall.Rlimit{
//		Cur: 0xffffffff,
//		Max: 0xffffffff,
//	})
//	if err != nil {
//		panic(err)
//	}
//}

// sendmsg wraps the sendmsg system call.
// Must len(iovs) >= len(vs)
func sendmsg(fd int, bs [][]byte, ivs []syscall.Iovec, zerocopy bool) (n int, err error) {
	iovLen := iovecs(bs, ivs)
	if iovLen == 0 {
		return 0, nil
	}
	var msghdr = syscall.Msghdr{
		Iov:    &ivs[0],
		Iovlen: uint64(iovLen),
	}
	var flags uintptr
	if zerocopy {
		flags = MSG_ZEROCOPY
	}
	r, _, e := syscall.RawSyscall(syscall.SYS_SENDMSG, uintptr(fd), uintptr(unsafe.Pointer(&msghdr)), flags)
	if e != 0 {
		return int(r), syscall.Errno(e)
	}
	return int(r), nil
}
