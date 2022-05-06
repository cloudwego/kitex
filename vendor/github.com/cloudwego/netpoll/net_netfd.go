// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file may have been modified by CloudWeGo authors. (“CloudWeGo Modifications”).
// All CloudWeGo Modifications are Copyright 2021 CloudWeGo authors.

//go:build aix || darwin || dragonfly || freebsd || linux || nacl || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package netpoll

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
	"syscall"
	"time"
)

var (
	// aLongTimeAgo is a non-zero time, far in the past, used for
	// immediate cancelation of dials.
	aLongTimeAgo = time.Unix(1, 0)
	// nonDeadline and noCancel are just zero values for
	// readability with functions taking too many parameters.
	noDeadline = time.Time{}
)

type netFD struct {
	// file descriptor
	fd int
	// When calling netFD.dial(), fd will be registered into poll in some scenarios, such as dialing tcp socket,
	// but not in other scenarios, such as dialing unix socket.
	// This leads to a different behavior in register poller at after, so use this field to mark it.
	pd *pollDesc
	// closed marks whether fd has expired
	closed uint32
	// Whether this is a streaming descriptor, as opposed to a
	// packet-based descriptor like a UDP socket. Immutable.
	isStream bool
	// Whether a zero byte read indicates EOF. This is false for a
	// message based socket connection.
	zeroReadIsEOF bool
	family        int    // AF_INET, AF_INET6, syscall.AF_UNIX
	sotype        int    // syscall.SOCK_STREAM, syscall.SOCK_DGRAM, syscall.SOCK_RAW
	isConnected   bool   // handshake completed or use of association with peer
	network       string // tcp tcp4 tcp6, udp, udp4, udp6, ip, ip4, ip6, unix, unixgram, unixpacket
	localAddr     net.Addr
	remoteAddr    net.Addr
}

func newNetFD(fd, family, sotype int, net string) *netFD {
	var ret = &netFD{}
	ret.fd = fd
	ret.network = net
	ret.family = family
	ret.sotype = sotype
	ret.isStream = sotype == syscall.SOCK_STREAM
	ret.zeroReadIsEOF = sotype != syscall.SOCK_DGRAM && sotype != syscall.SOCK_RAW
	return ret
}

// if dial connection error, you need exec netFD.Close actively
func (c *netFD) dial(ctx context.Context, laddr, raddr sockaddr) (err error) {
	var lsa syscall.Sockaddr
	if laddr != nil {
		if lsa, err = laddr.sockaddr(c.family); err != nil {
			return err
		} else if lsa != nil {
			// bind local address
			if err = syscall.Bind(c.fd, lsa); err != nil {
				return os.NewSyscallError("bind", err)
			}
		}
	}
	var rsa syscall.Sockaddr  // remote address from the user
	var crsa syscall.Sockaddr // remote address we actually connected to
	if raddr != nil {
		if rsa, err = raddr.sockaddr(c.family); err != nil {
			return err
		}
	}
	// remote address we actually connected to
	if crsa, err = c.connect(ctx, lsa, rsa); err != nil {
		return err
	}
	c.isConnected = true

	// Record the local and remote addresses from the actual socket.
	// Get the local address by calling Getsockname.
	// For the remote address, use
	// 1) the one returned by the connect method, if any; or
	// 2) the one from Getpeername, if it succeeds; or
	// 3) the one passed to us as the raddr parameter.
	lsa, _ = syscall.Getsockname(c.fd)
	c.localAddr = sockaddrToAddr(lsa)
	if crsa != nil {
		c.remoteAddr = sockaddrToAddr(crsa)
	} else if crsa, _ = syscall.Getpeername(c.fd); crsa != nil {
		c.remoteAddr = sockaddrToAddr(crsa)
	} else {
		c.remoteAddr = sockaddrToAddr(rsa)
	}
	return nil
}

func (c *netFD) connect(ctx context.Context, la, ra syscall.Sockaddr) (rsa syscall.Sockaddr, ret error) {
	// Do not need to call c.writing here,
	// because c is not yet accessible to user,
	// so no concurrent operations are possible.
	switch err := syscall.Connect(c.fd, ra); err {
	case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
	case nil, syscall.EISCONN:
		select {
		case <-ctx.Done():
			return nil, mapErr(ctx.Err())
		default:
		}
		return nil, nil
	case syscall.EINVAL:
		// On Solaris we can see EINVAL if the socket has
		// already been accepted and closed by the server.
		// Treat this as a successful connection--writes to
		// the socket will see EOF.  For details and a test
		// case in C see https://golang.org/issue/6828.
		if runtime.GOOS == "solaris" {
			return nil, nil
		}
		fallthrough
	default:
		return nil, os.NewSyscallError("connect", err)
	}

	// TODO: can't support interrupter now.
	// Start the "interrupter" goroutine, if this context might be canceled.
	// (The background context cannot)
	//
	// The interrupter goroutine waits for the context to be done and
	// interrupts the dial (by altering the c's write deadline, which
	// wakes up waitWrite).
	if ctx != context.Background() {
		// Wait for the interrupter goroutine to exit before returning
		// from connect.
		done := make(chan struct{})
		interruptRes := make(chan error)
		defer func() {
			close(done)
			if ctxErr := <-interruptRes; ctxErr != nil && ret == nil {
				// The interrupter goroutine called SetWriteDeadline,
				// but the connect code below had returned from
				// waitWrite already and did a successful connect (ret
				// == nil). Because we've now poisoned the connection
				// by making it unwritable, don't return a successful
				// dial. This was issue 16523.
				ret = mapErr(ctxErr)
				c.Close() // prevent a leak
			}
		}()
		go func() {
			select {
			case <-ctx.Done():
				// Force the runtime's poller to immediately give up
				// waiting for writability, unblocking waitWrite
				// below.
				c.SetWriteDeadline(aLongTimeAgo)
				interruptRes <- ctx.Err()
			case <-done:
				interruptRes <- nil
			}
		}()
	}

	c.pd = newPollDesc(c.fd)
	for {
		// Performing multiple connect system calls on a
		// non-blocking socket under Unix variants does not
		// necessarily result in earlier errors being
		// returned. Instead, once runtime-integrated network
		// poller tells us that the socket is ready, get the
		// SO_ERROR socket option to see if the connection
		// succeeded or failed. See issue 7474 for further
		// details.
		if err := c.pd.WaitWrite(ctx); err != nil {
			return nil, err
		}
		nerr, err := syscall.GetsockoptInt(c.fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			return nil, os.NewSyscallError("getsockopt", err)
		}
		switch err := syscall.Errno(nerr); err {
		case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
		case syscall.EISCONN:
			return nil, nil
		case syscall.Errno(0):
			// The runtime poller can wake us up spuriously;
			// see issues 14548 and 19289. Check that we are
			// really connected; if not, wait again.
			if rsa, err := syscall.Getpeername(c.fd); err == nil {
				return rsa, nil
			}
		default:
			return nil, os.NewSyscallError("connect", err)
		}
	}
}

// Various errors contained in OpError.
var (
	errMissingAddress = errors.New("missing address")
	errCanceled       = errors.New("operation was canceled")
	errIOTimeout      = errors.New("i/o timeout")
)

// mapErr maps from the context errors to the historical internal net
// error values.
//
// TODO(bradfitz): get rid of this after adjusting tests and making
// context.DeadlineExceeded implement net.Error?
func mapErr(err error) error {
	switch err {
	case context.Canceled:
		return errCanceled
	case context.DeadlineExceeded:
		return errIOTimeout
	default:
		return err
	}
}
