/*
 * Copyright 2024 CloudWeGo Authors
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

import "github.com/cloudwego/kitex/pkg/generic/thrift"

type thriftIDLProviderOptions struct {
	parseMode   *thrift.ParseMode
	goTag       *goTagOption
	serviceName string
}

type goTagOption struct {
	isGoTagAliasDisabled bool
}

type ThriftIDLProviderOption struct {
	F func(opt *thriftIDLProviderOptions)
}

func (o *thriftIDLProviderOptions) apply(opts []ThriftIDLProviderOption) {
	for _, op := range opts {
		op.F(o)
	}
}

// WithParseMode sets the parse mode.
// NOTE: when using WithIDLServiceName at the same time, parse mode will be ignored.
func WithParseMode(parseMode thrift.ParseMode) ThriftIDLProviderOption {
	return ThriftIDLProviderOption{F: func(opt *thriftIDLProviderOptions) {
		opt.parseMode = &parseMode
	}}
}

func WithGoTagDisabled(disable bool) ThriftIDLProviderOption {
	return ThriftIDLProviderOption{F: func(opt *thriftIDLProviderOptions) {
		opt.goTag = &goTagOption{
			isGoTagAliasDisabled: disable,
		}
	}}
}

// WithIDLServiceName specifies the target IDL service to be parsed.
// NOTE: when using this option, the specified service is prioritized, and parse mode will be ignored.
func WithIDLServiceName(serviceName string) ThriftIDLProviderOption {
	return ThriftIDLProviderOption{F: func(opt *thriftIDLProviderOptions) {
		opt.serviceName = serviceName
	}}
}
