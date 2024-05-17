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

package http

import (
	"net"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestResolve(t *testing.T) {
	r := NewDefaultResolver()
	resolvableURL := "http://www.bytedance.com"
	_, err := r.Resolve(resolvableURL)
	test.Assert(t, err == nil)
	invalidURL := "http://www.)(*&^%$#@!adomainshouldnotexists.com"
	_, err = r.Resolve(invalidURL)
	test.Assert(t, err != nil)
	irresolvableURL := "http://www.adomainshouldnotexists.com"
	_, err = r.Resolve(irresolvableURL)
	test.Assert(t, err != nil)
}

func TestResolverWithIPPolicy(t *testing.T) {
	// panic for invalid policy that disables both version
	var panicErr interface{}
	f := func() {
		defer func() {
			panicErr = recover()
		}()
		NewResolverWithIPPolicy(IPPolicy{DisableV4: true, DisableV6: true})
	}
	f()
	test.Assert(t, panicErr != nil)

	// resolve ipv6 only
	v6Resolver := NewResolverWithIPPolicy(IPPolicy{DisableV4: true})
	test.Assert(t, v6Resolver.(*defaultResolver).network == tcp6)
	resolvableURL := "http://www.google.com"
	ipPort, err := v6Resolver.Resolve(resolvableURL)
	test.Assert(t, err == nil)
	ip, err := parseIPPort(ipPort)
	test.Assert(t, err == nil)
	test.Assert(t, isV6(ip))

	invalidURL := "http://www.)(*&^%$#@!adomainshouldnotexists.com"
	_, err = v6Resolver.Resolve(invalidURL)
	test.Assert(t, err != nil)
	irresolvableURL := "http://www.adomainshouldnotexists.com"
	_, err = v6Resolver.Resolve(irresolvableURL)
	test.Assert(t, err != nil)

	// resolve ipv4 only
	v4Resolver := NewResolverWithIPPolicy(IPPolicy{DisableV6: true})
	test.Assert(t, v4Resolver.(*defaultResolver).network == tcp4)
	ipPort, err = v4Resolver.Resolve(resolvableURL)
	test.Assert(t, err == nil)
	ip, err = parseIPPort(ipPort)
	test.Assert(t, err == nil)
	test.Assert(t, isV4(ip))

	// dual
	dualResolver := NewResolverWithIPPolicy(IPPolicy{})
	test.Assert(t, dualResolver.(*defaultResolver).network == tcp)
	ipPort, err = dualResolver.Resolve(resolvableURL)
	test.Assert(t, err == nil)
	ip, err = parseIPPort(ipPort)
	test.Assert(t, err == nil)
}

func TestGetNetwork(t *testing.T) {
	v4only := IPPolicy{DisableV6: true}
	test.Assert(t, getNetwork(v4only) == tcp4)

	v6only := IPPolicy{DisableV4: true}
	test.Assert(t, getNetwork(v6only) == tcp6)

	dual := IPPolicy{}
	test.Assert(t, getNetwork(dual) == tcp)
}

func parseIPPort(ipPort string) (net.IP, error) {
	host, _, err := net.SplitHostPort(ipPort)
	if err != nil {
		return nil, err
	}

	return net.ParseIP(host), nil
}

func isV6(ip net.IP) bool {
	return ip.To4() == nil && ip.To16() != nil
}

func isV4(ip net.IP) bool {
	return ip.To4() != nil
}
