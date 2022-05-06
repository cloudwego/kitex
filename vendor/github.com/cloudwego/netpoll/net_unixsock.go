// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file may have been modified by CloudWeGo authors. (“CloudWeGo Modifications”).
// All CloudWeGo Modifications are Copyright 2021 CloudWeGo authors.

// +build aix darwin dragonfly freebsd js,wasm linux nacl netbsd openbsd solaris windows

package netpoll

import (
	"context"
	"errors"
	"net"
	"syscall"
)

// BUG(mikio): On JS, NaCl and Plan 9, methods and functions related
// to UnixConn and UnixListener are not implemented.

// BUG(mikio): On Windows, methods and functions related to UnixConn
// and UnixListener don't work for "unixgram" and "unixpacket".

// UnixAddr represents the address of a Unix domain socket end point.
type UnixAddr struct {
	net.UnixAddr
}

func (a *UnixAddr) isWildcard() bool {
	return a == nil || a.Name == ""
}

func (a *UnixAddr) opAddr() net.Addr {
	if a == nil {
		return nil
	}
	return a
}

func (a *UnixAddr) family() int {
	return syscall.AF_UNIX
}

func (a *UnixAddr) sockaddr(family int) (syscall.Sockaddr, error) {
	if a == nil {
		return nil, nil
	}
	return &syscall.SockaddrUnix{Name: a.Name}, nil
}

func (a *UnixAddr) toLocal(net string) sockaddr {
	return a
}

// ResolveUnixAddr returns an address of Unix domain socket end point.
//
// The network must be a Unix network name.
//
// See func Dial for a description of the network and address
// parameters.
func ResolveUnixAddr(network, address string) (*UnixAddr, error) {
	addr, err := net.ResolveUnixAddr(network, address)
	if err != nil {
		return nil, err
	}
	return &UnixAddr{*addr}, nil
}

// UnixConnection implements Connection.
type UnixConnection struct {
	connection
}

// newUnixConnection wraps UnixConnection.
func newUnixConnection(conn Conn) (connection *UnixConnection, err error) {
	connection = &UnixConnection{}
	err = connection.init(conn, nil)
	if err != nil {
		return nil, err
	}
	return connection, nil
}

// DialUnix acts like Dial for Unix networks.
//
// The network must be a Unix network name; see func Dial for details.
//
// If laddr is non-nil, it is used as the local address for the
// connection.
func DialUnix(network string, laddr, raddr *UnixAddr) (*UnixConnection, error) {
	switch network {
	case "unix", "unixgram", "unixpacket":
	default:
		return nil, &net.OpError{Op: "dial", Net: network, Source: laddr.opAddr(), Addr: raddr.opAddr(), Err: net.UnknownNetworkError(network)}
	}
	sd := &sysDialer{network: network, address: raddr.String()}
	c, err := sd.dialUnix(context.Background(), laddr, raddr)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: laddr.opAddr(), Addr: raddr.opAddr(), Err: err}
	}
	return c, nil
}

func (sd *sysDialer) dialUnix(ctx context.Context, laddr, raddr *UnixAddr) (*UnixConnection, error) {
	conn, err := unixSocket(ctx, sd.network, laddr, raddr, "dial")
	if err != nil {
		return nil, err
	}
	return newUnixConnection(conn)
}

func unixSocket(ctx context.Context, network string, laddr, raddr sockaddr, mode string) (conn *netFD, err error) {
	var sotype int
	switch network {
	case "unix":
		sotype = syscall.SOCK_STREAM
	case "unixgram":
		sotype = syscall.SOCK_DGRAM
	case "unixpacket":
		sotype = syscall.SOCK_SEQPACKET
	default:
		return nil, net.UnknownNetworkError(network)
	}

	switch mode {
	case "dial":
		if laddr != nil && laddr.isWildcard() {
			laddr = nil
		}
		if raddr != nil && raddr.isWildcard() {
			raddr = nil
		}
		if raddr == nil && (sotype != syscall.SOCK_DGRAM || laddr == nil) {
			return nil, errMissingAddress
		}
	case "listen":
	default:
		return nil, errors.New("unknown mode: " + mode)
	}

	return socket(ctx, network, syscall.AF_UNIX, sotype, 0, false, laddr, raddr)
}
