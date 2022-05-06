// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file may have been modified by CloudWeGo authors. (“CloudWeGo Modifications”).
// All CloudWeGo Modifications are Copyright 2021 CloudWeGo authors.

// +build aix darwin dragonfly freebsd linux nacl netbsd openbsd solaris windows

package netpoll

import (
	"context"
	"net"
	"runtime"
	"syscall"
)

// A sockaddr represents a TCP, UDP, IP or Unix network endpoint
// address that can be converted into a syscall.Sockaddr.
type sockaddr interface {
	net.Addr

	// family returns the platform-dependent address family
	// identifier.
	family() int

	// isWildcard reports whether the address is a wildcard
	// address.
	isWildcard() bool

	// sockaddr returns the address converted into a syscall
	// sockaddr type that implements syscall.Sockaddr
	// interface. It returns a nil interface when the address is nil.
	sockaddr(family int) (syscall.Sockaddr, error)

	// toLocal maps the zero address to a local system address (127.0.0.1 or ::1)
	toLocal(net string) sockaddr
}

func internetSocket(ctx context.Context, net string, laddr, raddr sockaddr, sotype, proto int, mode string) (conn *netFD, err error) {
	if (runtime.GOOS == "aix" || runtime.GOOS == "windows" || runtime.GOOS == "openbsd" || runtime.GOOS == "nacl") && raddr.isWildcard() {
		raddr = raddr.toLocal(net)
	}
	family, ipv6only := favoriteAddrFamily(net, laddr, raddr)
	return socket(ctx, net, family, sotype, proto, ipv6only, laddr, raddr)
}

// favoriteAddrFamily returns the appropriate address family for the
// given network, laddr, raddr and mode.
//
// If mode indicates "listen" and laddr is a wildcard, we assume that
// the user wants to make a passive-open connection with a wildcard
// address family, both AF_INET and AF_INET6, and a wildcard address
// like the following:
//
// 	- A listen for a wildcard communication domain, "tcp" or
// 	  "udp", with a wildcard address: If the platform supports
// 	  both IPv6 and IPv4-mapped IPv6 communication capabilities,
// 	  or does not support IPv4, we use a dual stack, AF_INET6 and
// 	  IPV6_V6ONLY=0, wildcard address listen. The dual stack
// 	  wildcard address listen may fall back to an IPv6-only,
// 	  AF_INET6 and IPV6_V6ONLY=1, wildcard address listen.
// 	  Otherwise we prefer an IPv4-only, AF_INET, wildcard address
// 	  listen.
//
// 	- A listen for a wildcard communication domain, "tcp" or
// 	  "udp", with an IPv4 wildcard address: same as above.
//
// 	- A listen for a wildcard communication domain, "tcp" or
// 	  "udp", with an IPv6 wildcard address: same as above.
//
// 	- A listen for an IPv4 communication domain, "tcp4" or "udp4",
// 	  with an IPv4 wildcard address: We use an IPv4-only, AF_INET,
// 	  wildcard address listen.
//
// 	- A listen for an IPv6 communication domain, "tcp6" or "udp6",
// 	  with an IPv6 wildcard address: We use an IPv6-only, AF_INET6
// 	  and IPV6_V6ONLY=1, wildcard address listen.
//
// Otherwise guess: If the addresses are IPv4 then returns AF_INET,
// or else returns AF_INET6. It also returns a boolean value what
// designates IPV6_V6ONLY option.
//
// Note that the latest DragonFly BSD and OpenBSD kernels allow
// neither "net.inet6.ip6.v6only=1" change nor IPPROTO_IPV6 level
// IPV6_V6ONLY socket option setting.
func favoriteAddrFamily(network string, laddr, raddr sockaddr) (family int, ipv6only bool) {
	switch network[len(network)-1] {
	case '4':
		return syscall.AF_INET, false
	case '6':
		return syscall.AF_INET6, true
	}
	if (laddr == nil || laddr.family() == syscall.AF_INET) &&
		(raddr == nil || raddr.family() == syscall.AF_INET) {
		return syscall.AF_INET, false
	}
	return syscall.AF_INET6, false
}

// socket returns a network file descriptor that is ready for
// asynchronous I/O using the network poller.
func socket(ctx context.Context, net string, family, sotype, proto int, ipv6only bool, laddr, raddr sockaddr) (netfd *netFD, err error) {
	// syscall.Socket & set socket options
	var fd int
	fd, err = sysSocket(family, sotype, proto)
	if err != nil {
		return nil, err
	}
	err = setDefaultSockopts(fd, family, sotype, ipv6only)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}

	netfd = newNetFD(fd, family, sotype, net)
	err = netfd.dial(ctx, laddr, raddr)
	if err != nil {
		netfd.Close()
		return nil, err
	}
	return netfd, nil
}

// sockaddrToAddr returns a go/net friendly address
func sockaddrToAddr(sa syscall.Sockaddr) net.Addr {
	var a net.Addr
	switch sa := sa.(type) {
	case *syscall.SockaddrInet4:
		a = &net.TCPAddr{
			IP:   sa.Addr[0:],
			Port: sa.Port,
		}
	case *syscall.SockaddrInet6:
		var zone string
		if sa.ZoneId != 0 {
			if ifi, err := net.InterfaceByIndex(int(sa.ZoneId)); err == nil {
				zone = ifi.Name
			}
		}
		// if zone == "" && sa.ZoneId != 0 {
		// }
		a = &net.TCPAddr{
			IP:   sa.Addr[0:],
			Port: sa.Port,
			Zone: zone,
		}
	case *syscall.SockaddrUnix:
		a = &net.UnixAddr{Net: "unix", Name: sa.Name}
	}
	return a
}
