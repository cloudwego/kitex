// Copyright 2021 ByteDance Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metainfo

import (
	"context"
	"strings"
)

// HTTP header prefixes.
const (
	HTTPPrefixTransient  = "rpc-transit-"
	HTTPPrefixPersistent = "rpc-persist-"
	HTTPPrefixBackward   = "rpc-backward-"

	lenHPT = len(HTTPPrefixTransient)
	lenHPP = len(HTTPPrefixPersistent)
	lenHPB = len(HTTPPrefixBackward)
)

// HTTPHeaderToCGIVariable performs an CGI variable conversion.
// For example, an HTTP header key `abc-def` will result in `ABC_DEF`.
func HTTPHeaderToCGIVariable(key string) string {
	return strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
}

// CGIVariableToHTTPHeader converts a CGI variable into an HTTP header key.
// For example, `ABC_DEF` will be converted to `abc-def`.
func CGIVariableToHTTPHeader(key string) string {
	return strings.ToLower(strings.ReplaceAll(key, "_", "-"))
}

// HTTPHeaderSetter sets a key with a value into a HTTP header.
type HTTPHeaderSetter interface {
	Set(key, value string)
}

// HTTPHeaderCarrier accepts a visitor to access all key value pairs in an HTTP header.
type HTTPHeaderCarrier interface {
	Visit(func(k, v string))
}

// HTTPHeader is provided to wrap an http.Header into an HTTPHeaderCarrier.
type HTTPHeader map[string][]string

// Visit implements the HTTPHeaderCarrier interface.
func (h HTTPHeader) Visit(v func(k, v string)) {
	for k, vs := range h {
		v(k, vs[0])
	}
}

// Set sets the header entries associated with key to the single element value.
// The key will converted into lowercase as the HTTP/2 protocol requires.
func (h HTTPHeader) Set(key, value string) {
	h[strings.ToLower(key)] = []string{value}
}

// FromHTTPHeader reads metainfo from a given HTTP header and sets them into the context.
// Note that this function does not call TransferForward inside.
func FromHTTPHeader(ctx context.Context, h HTTPHeaderCarrier) context.Context {
	if ctx == nil || h == nil {
		return ctx
	}

	var m *mapView
	if x := getNode(ctx); x != nil {
		m = x.mapView()
	} else {
		m = newMapView()
	}
	h.Visit(func(k, v string) {
		if len(v) == 0 {
			return
		}

		kk := strings.ToLower(k)
		ln := len(kk)

		if ln > lenHPT && strings.HasPrefix(kk, HTTPPrefixTransient) {
			kk = HTTPHeaderToCGIVariable(kk[lenHPT:])
			m.transient[kk] = v
		} else if ln > lenHPP && strings.HasPrefix(kk, HTTPPrefixPersistent) {
			kk = HTTPHeaderToCGIVariable(kk[lenHPP:])
			m.persistent[kk] = v
		}
	})

	if m.size() == 0 {
		// TODO: remove this?
		return ctx
	}
	return withNode(ctx, m.toNode())
}

// ToHTTPHeader writes all metainfo into the given HTTP header.
// Note that this function does not call TransferForward inside.
func ToHTTPHeader(ctx context.Context, h HTTPHeaderSetter) {
	if ctx == nil || h == nil {
		return
	}

	for k, v := range GetAllValues(ctx) {
		k := HTTPPrefixTransient + CGIVariableToHTTPHeader(k)
		h.Set(k, v)
	}

	for k, v := range GetAllPersistentValues(ctx) {
		k := HTTPPrefixPersistent + CGIVariableToHTTPHeader(k)
		h.Set(k, v)
	}
}
