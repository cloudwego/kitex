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

package generic

import "github.com/cloudwego/dynamicgo/conv"

type Options struct {
	// options for dynamicgo conversion
	dynamicgoConvOpts conv.Options
	// flag to set whether to get response for http generic call using dynamicgo
	enableDynamicgoHTTPResp bool
}

type Option struct {
	F func(opt *Options)
}

// NewOptions creates a new option
func NewOptions(opts []Option) *Options {
	o := &Options{}
	o.apply(opts)
	return o
}

// apply applies all options
func (o *Options) apply(opts []Option) {
	for _, op := range opts {
		op.F(o)
	}
}

// WithDefaultJSONDynamicgoConvOpts sets the default conv.Options for json generic call
func WithDefaultJSONDynamicgoConvOpts() Option {
	return Option{F: func(opt *Options) {
		opt.dynamicgoConvOpts = conv.Options{
			WriteRequireField: true,
			WriteDefaultField: true,
		}
	}}
}

// WithDefaultHTTPDynamicgoConvOpts sets the default conv.Options for http generic call
func WithDefaultHTTPDynamicgoConvOpts() Option {
	return Option{F: func(opt *Options) {
		opt.dynamicgoConvOpts = conv.Options{
			EnableHttpMapping:     true,
			EnableValueMapping:    true,
			WriteRequireField:     true,
			WriteDefaultField:     true,
			OmitHttpMappingErrors: true,
			NoBase64Binary:        true,
		}
	}}
}

// WithCustomDynamicgoConvOpts sets custom conv.Options
func WithCustomDynamicgoConvOpts(opts conv.Options) Option {
	return Option{F: func(opt *Options) {
		opt.dynamicgoConvOpts = opts
	}}
}

// EnableDynamicgoHTTPResp set whether to get response for http generic call using dynamicgo
func EnableDynamicgoHTTPResp(enable bool) Option {
	return Option{F: func(opt *Options) {
		opt.enableDynamicgoHTTPResp = enable
	}}
}
