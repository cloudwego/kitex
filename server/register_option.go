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

package server

import (
	"github.com/cloudwego/kitex/pkg/endpoint"
)

// RegisterOption is the only way to config service registration.
type RegisterOption struct {
	F func(o *RegisterOptions)
}

// RegisterOptions is used to config service registration.
type RegisterOptions struct {
	IsFallbackService bool
	UnaryMiddlewares  []UnaryMiddleware
	StreamMiddlewares []StreamMiddleware
	Middlewares       []endpoint.Middleware
}

// NewRegisterOptions creates a register options.
func NewRegisterOptions(opts []RegisterOption) *RegisterOptions {
	o := &RegisterOptions{}
	ApplyRegisterOptions(opts, o)
	return o
}

// ApplyRegisterOptions applies the given register options.
func ApplyRegisterOptions(opts []RegisterOption, o *RegisterOptions) {
	for _, op := range opts {
		op.F(o)
	}
}

func WithServiceUnaryMiddleware(mw UnaryMiddleware) RegisterOption {
	return RegisterOption{F: func(o *RegisterOptions) {
		o.UnaryMiddlewares = append(o.UnaryMiddlewares, mw)
	}}
}

func WithServiceStreamMiddleware(mw StreamMiddleware) RegisterOption {
	return RegisterOption{F: func(o *RegisterOptions) {
		o.StreamMiddlewares = append(o.StreamMiddlewares, mw)
	}}
}

// WithServiceMiddleware add middleware for a single service
// The execution order of middlewares follows:
// - server middlewares
// - service middlewares
// - service handler
// Deprecated: use WithServiceUnaryMiddleware or WithServiceStreamMiddleware instead.
func WithServiceMiddleware(mw endpoint.Middleware) RegisterOption {
	return RegisterOption{F: func(o *RegisterOptions) {
		o.Middlewares = append(o.Middlewares, mw)
	}}
}

func WithFallbackService() RegisterOption {
	return RegisterOption{F: func(o *RegisterOptions) {
		o.IsFallbackService = true
	}}
}
