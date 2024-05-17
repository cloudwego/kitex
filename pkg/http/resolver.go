/*
 * Copyright 2021 CloudWeGo Authors
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

// Package http is used to implement RPC over HTTP.
package http

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
)

const (
	tcp  = "tcp"
	tcp4 = "tcp4"
	tcp6 = "tcp6"
)

// Resolver resolves url to address.
type Resolver interface {
	Resolve(string) (string, error)
}

// IPPolicy specifies the ip version for HTTP Resolver
type IPPolicy struct {
	DisableV4 bool
	DisableV6 bool
}

// if both v4 and v6 are disabled, return false
func (p IPPolicy) legal() bool {
	return !(p.DisableV6 && p.DisableV4)
}

type defaultResolver struct {
	network string
}

// NewDefaultResolver creates a default resolver.
func NewDefaultResolver() Resolver {
	return &defaultResolver{network: tcp}
}

// NewResolverWithIPPolicy creates a resolver with ip policy.
func NewResolverWithIPPolicy(p IPPolicy) Resolver {
	if !p.legal() {
		panic(fmt.Sprintf("Illegal IP policy for resolver, at least one of v4, v6 should be enabled"))
	}
	network := getNetwork(p)
	return &defaultResolver{network}
}

// Resolve implements the Resolver interface.
func (p *defaultResolver) Resolve(URL string) (string, error) {
	pu, err := url.Parse(URL)
	if err != nil {
		return "", err
	}
	host := pu.Hostname()
	port := pu.Port()
	if port == "" {
		port = "443"
		if pu.Scheme == "http" {
			port = "80"
		}
	}
	addr, err := net.ResolveTCPAddr(p.network, net.JoinHostPort(host, port))
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port)), nil
}

func getNetwork(policy IPPolicy) string {
	v4, v6 := !policy.DisableV4, !policy.DisableV6
	if v4 && v6 {
		return tcp
	} else if v4 {
		return tcp4
	} else {
		return tcp6
	}
}
