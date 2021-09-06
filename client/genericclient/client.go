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

// Package genericclient ...
package genericclient

import (
	"context"

	"github.com/jackedelic/kitex/internal/client"
	"github.com/jackedelic/kitex/internal/client/callopt"
	"github.com/jackedelic/kitex/internal/pkg/generic"
	"github.com/jackedelic/kitex/internal/pkg/serviceinfo"
)

// NewClient create a generic client
func NewClient(psm string, g generic.Generic, opts ...client.Option) (Client, error) {
	svcInfo := generic.ServiceInfo(g.PayloadCodecType())
	return NewClientWithServiceInfo(psm, g, svcInfo, opts...)
}

// NewClientWithServiceInfo create a generic client with serviceInfo
func NewClientWithServiceInfo(psm string, g generic.Generic, svcInfo *serviceinfo.ServiceInfo, opts ...client.Option) (Client, error) {
	var options []client.Option
	options = append(options, client.WithGeneric(g))
	options = append(options, client.WithDestService(psm))
	options = append(options, opts...)

	kc, err := client.NewClient(svcInfo, options...)
	if err != nil {
		return nil, err
	}
	return &genericServiceClient{
		kClient: kc,
		g:       g,
	}, nil
}

// Client generic client
type Client interface {
	// GenericCall generic call
	GenericCall(ctx context.Context, method string, request interface{}, callOptions ...callopt.Option) (response interface{}, err error)
}

type genericServiceClient struct {
	kClient client.Client
	g       generic.Generic
}

func (gc *genericServiceClient) GenericCall(ctx context.Context, method string, request interface{}, callOptions ...callopt.Option) (response interface{}, err error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOptions)
	var _args generic.Args
	_args.Method = method
	_args.Request = request
	mt, err := gc.g.GetMethod(request, method)
	if err != nil {
		return nil, err
	}
	if mt.Oneway {
		return nil, gc.kClient.Call(ctx, mt.Name, &_args, nil)
	}
	var _result generic.Result
	if err = gc.kClient.Call(ctx, mt.Name, &_args, &_result); err != nil {
		return
	}
	return _result.GetSuccess(), nil
}
