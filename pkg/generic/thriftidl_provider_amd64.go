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

package generic

import (
	"context"
	"fmt"

	dthrift "github.com/cloudwego/dynamicgo/thrift"
	"github.com/cloudwego/thriftgo/parser"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
)

// NewThriftFileProvider create a ThriftIDLProvider by given path and include dirs
func NewThriftFileProvider(path string, includeDirs ...string) (DescriptorProvider, error) {
	p := &thriftFileProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
	}
	tree, err := parser.ParseFile(path, includeDirs, true)
	if err != nil {
		return nil, err
	}
	svc, err := thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return nil, err
	}

	// ServiceDescriptor of dynamicgo
	dsvc, err := dthrift.NewDescritorFromPath(context.Background(), path, includeDirs...)
	if err != nil {
		return nil, err
	}
	svc.DynamicgoDsc = dsvc

	p.svcs <- svc
	return p, nil
}

// NewThriftContentProvider builder
func NewThriftContentProvider(main string, includes map[string]string) (*ThriftContentProvider, error) {
	p := &ThriftContentProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
	}
	tree, err := ParseContent(defaultMainIDLPath, main, includes, false)
	if err != nil {
		return nil, err
	}
	svc, err := thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return nil, err
	}

	// ServiceDescriptor of dynamicgo
	dsvc, err := dthrift.NewDescritorFromContent(context.Background(), defaultMainIDLPath, main, includes, false)
	if err != nil {
		return nil, err
	}
	svc.DynamicgoDsc = dsvc

	p.svcs <- svc
	return p, nil
}

// UpdateIDL ...
func (p *ThriftContentProvider) UpdateIDL(main string, includes map[string]string) error {
	var svc *descriptor.ServiceDescriptor
	tree, err := ParseContent(defaultMainIDLPath, main, includes, false)
	if err != nil {
		return err
	}
	svc, err = thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return err
	}

	dsvc, err := dthrift.NewDescritorFromContent(context.Background(), defaultMainIDLPath, main, includes, false)
	if err != nil {
		return err
	}
	svc.DynamicgoDsc = dsvc

	select {
	case <-p.svcs:
	default:
	}
	select {
	case p.svcs <- svc:
	default:
	}
	return nil
}

// NewThriftContentWithAbsIncludePathProvider create abs include path DescriptorProvider
func NewThriftContentWithAbsIncludePathProvider(mainIDLPath string, includes map[string]string) (*ThriftContentWithAbsIncludePathProvider, error) {
	p := &ThriftContentWithAbsIncludePathProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
	}
	mainIDLContent, ok := includes[mainIDLPath]
	if !ok {
		return nil, fmt.Errorf("miss main IDL content for main IDL path: %s", mainIDLPath)
	}
	tree, err := ParseContent(mainIDLPath, mainIDLContent, includes, true)
	if err != nil {
		return nil, err
	}
	svc, err := thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return nil, err
	}

	// ServiceDescriptor of dynamicgo
	dsvc, err := dthrift.NewDescritorFromContent(context.Background(), mainIDLPath, mainIDLContent, includes, true)
	if err != nil {
		return nil, err
	}
	svc.DynamicgoDsc = dsvc

	p.svcs <- svc

	return p, nil
}

// UpdateIDL update idl by given args
func (p *ThriftContentWithAbsIncludePathProvider) UpdateIDL(mainIDLPath string, includes map[string]string) error {
	mainIDLContent, ok := includes[mainIDLPath]
	if !ok {
		return fmt.Errorf("miss main IDL content for main IDL path: %s", mainIDLPath)
	}
	var svc *descriptor.ServiceDescriptor
	tree, err := ParseContent(mainIDLPath, mainIDLContent, includes, true)
	if err != nil {
		return err
	}
	svc, err = thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return err
	}

	dsvc, err := dthrift.NewDescritorFromContent(context.Background(), mainIDLPath, mainIDLContent, includes, true)
	if err != nil {
		return err
	}
	svc.DynamicgoDsc = dsvc

	// drain the channel
	select {
	case <-p.svcs:
	default:
	}
	select {
	case p.svcs <- svc:
	default:
	}
	return nil
}
