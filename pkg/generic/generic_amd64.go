//go:build amd64
// +build amd64

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

// Package generic ...
package generic

import (
	"sync"

	"github.com/cloudwego/dynamicgo/conv"
)

const defaultBufferSize = 4096

var bytesPool = sync.Pool{
	New: func() interface{} {
		// TODO: need to consider the size
		b := make([]byte, 0, defaultBufferSize)
		return &b
	},
}

type DynamicgoOptions struct {
	ConvOpts                conv.Options
	EnableDynamicgoHTTPResp bool
}

// HTTPThriftGeneric http mapping Generic.
// Base64 codec for binary field is disabled by default. You can change this option with SetBinaryWithBase64.
// eg:
//
//	g, err := generic.HTTPThriftGeneric(p)
//	SetBinaryWithBase64(g, true)
func HTTPThriftGeneric(p DescriptorProvider, opts ...DynamicgoOptions) (Generic, error) {
	var codec *httpThriftCodec
	var err error
	if opts != nil {
		// generic with dynamicgo
		codec, err = newHTTPThriftCodec(p, thriftCodec, opts[0])
		if err != nil {
			return nil, err
		}
	} else {
		codec, err = newHTTPThriftCodec(p, thriftCodec)
		if err != nil {
			return nil, err
		}
	}
	return &httpThriftGeneric{codec: codec}, nil
}

// JSONThriftGeneric json mapping generic.
// Base64 codec for binary field is enabled by default. You can change this option with SetBinaryWithBase64.
// eg:
//
//	g, err := generic.JSONThriftGeneric(p)
//	SetBinaryWithBase64(g, false)
func JSONThriftGeneric(p DescriptorProvider, opts ...DynamicgoOptions) (Generic, error) {
	var codec *jsonThriftCodec
	var err error
	if opts != nil {
		// generic with dynamicgo
		codec, err = newJsonThriftCodec(p, thriftCodec, opts[0])
		if err != nil {
			return nil, err
		}
	} else {
		codec, err = newJsonThriftCodec(p, thriftCodec)
		if err != nil {
			return nil, err
		}
	}
	return &jsonThriftGeneric{codec: codec}, nil
}
