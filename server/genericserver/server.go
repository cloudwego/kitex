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

// Package genericserver ...
package genericserver

import (
	"errors"

	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/server"
)

// NewServer creates a generic server with the given handler and options.
func NewServer(handler generic.Service, g generic.Generic, opts ...server.Option) server.Server {
	svcInfo := generic.ServiceInfoWithGeneric(g)
	return NewServerWithServiceInfo(handler, g, svcInfo, opts...)
}

// NewServerWithServiceInfo creates a generic server with the given handler, serviceInfo and options.
func NewServerWithServiceInfo(handler generic.Service, g generic.Generic, svcInfo *serviceinfo.ServiceInfo, opts ...server.Option) server.Server {
	var options []server.Option
	options = append(options, server.WithGeneric(g))
	options = append(options, opts...)

	svr := server.NewServer(options...)
	if err := svr.RegisterService(svcInfo, handler); err != nil {
		panic(err)
	}
	return svr
}

// NewServerV2 creates a generic server with the given handler, generic and options.
// Note: NOT support generic.BinaryThriftGeneric, please use generic.BinaryThriftGenericV2 instead.
func NewServerV2(handler *generic.ServiceV2, g generic.Generic, opts ...server.Option) server.Server {
	svr := server.NewServer(opts...)
	if err := RegisterService(svr, handler, g); err != nil {
		panic(err)
	}
	return svr
}

// RegisterService registers a generic service with the given server.
// Note: NOT support generic.BinaryThriftGeneric, please use generic.BinaryThriftGenericV2 instead.
func RegisterService(svr server.Server, handler *generic.ServiceV2, g generic.Generic, opts ...server.RegisterOption) error {
	if svr == nil {
		return errors.New("svr is nil. please specify non-nil svr")
	}
	if g == nil {
		return errors.New("generic is nil. please specify non-nil generic")
	}
	if _, ok := g.GetExtra(igeneric.BinaryThriftGenericV1PayloadCodecKey).(remote.PayloadCodec); ok {
		return errors.New("KITEX: not support generic.BinaryThriftGeneric, please use generic.BinaryThriftGenericV2 instead")
	}
	return svr.RegisterService(generic.ServiceInfoWithGeneric(g), handler, opts...)
}
