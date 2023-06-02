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
	"path/filepath"
	"sync"

	dthrift "github.com/cloudwego/dynamicgo/thrift"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/thriftgo/parser"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
)

var (
	_ Closer = &ThriftContentProvider{}
	_ Closer = &ThriftContentWithAbsIncludePathProvider{}
)

type thriftFileProvider struct {
	closeOnce sync.Once
	svcs      chan *descriptor.ServiceDescriptor
	opts      *ProviderOption
}

// NewThriftFileProvider create a ThriftIDLProvider by given path and include dirs
func NewThriftFileProvider(path string, includeDirs ...string) (DescriptorProvider, error) {
	p := &thriftFileProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts: &ProviderOption{DynamicGoExpected: false},
	}
	svc, err := newServiceDescriptorFromPath(path, includeDirs...)
	if err != nil {
		return nil, err
	}
	p.svcs <- svc
	return p, nil
}

// NewThriftFileProviderWithDynamicGo create a ThriftIDLProvider with dynamicgo by given path and include dirs
func NewThriftFileProviderWithDynamicGo(path string, includeDirs ...string) (DescriptorProvider, error) {
	p := &thriftFileProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts: &ProviderOption{DynamicGoExpected: true},
	}

	svc, err := newServiceDescriptorFromPath(path, includeDirs...)
	if err != nil {
		return nil, err
	}

	// ServiceDescriptor of dynamicgo
	dsvc, err := dthrift.NewDescritorFromPath(context.Background(), path, includeDirs...)
	if err != nil {
		// fall back to the original way (without dynamicgo)
		p.opts.DynamicGoExpected = false
		klog.CtxWarnf(context.Background(), "KITEX: failed to get dynamicgo service descriptor, fall back to the original way, error=%s", err)
		return p, nil
	}
	svc.DynamicGoDsc = dsvc

	p.svcs <- svc
	return p, nil
}

func newServiceDescriptorFromPath(path string, includeDirs ...string) (*descriptor.ServiceDescriptor, error) {
	tree, err := parser.ParseFile(path, includeDirs, true)
	if err != nil {
		return nil, err
	}
	svc, err := thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return nil, err
	}
	return svc, nil
}

// maybe watch the file change and reparse
// p.thrifts <- some change
// func (p *thriftFileProvider) watchAndUpdate() {}

func (p *thriftFileProvider) Provide() <-chan *descriptor.ServiceDescriptor {
	return p.svcs
}

// Close the sending chan.
func (p *thriftFileProvider) Close() error {
	p.closeOnce.Do(func() {
		close(p.svcs)
	})
	return nil
}

func (p *thriftFileProvider) Option() ProviderOption {
	return *p.opts
}

// ThriftContentProvider provide descriptor from contents
type ThriftContentProvider struct {
	closeOnce sync.Once
	svcs      chan *descriptor.ServiceDescriptor
	opts      *ProviderOption
}

var _ DescriptorProvider = (*ThriftContentProvider)(nil)

const defaultMainIDLPath = "main.thrift"

// NewThriftContentProvider builder
func NewThriftContentProvider(main string, includes map[string]string) (*ThriftContentProvider, error) {
	p := &ThriftContentProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts: &ProviderOption{DynamicGoExpected: false},
	}
	svc, err := newServiceDescriptorFromContent(defaultMainIDLPath, main, includes, false)
	if err != nil {
		return nil, err
	}

	p.svcs <- svc
	return p, nil
}

// NewThriftContentProviderWithDynamicGo builder
func NewThriftContentProviderWithDynamicGo(main string, includes map[string]string) (*ThriftContentProvider, error) {
	p := &ThriftContentProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts: &ProviderOption{DynamicGoExpected: true},
	}
	svc, err := newServiceDescriptorFromContent(defaultMainIDLPath, main, includes, false)
	if err != nil {
		return nil, err
	}

	if err = newDynamicgoDscFromContent(svc, defaultMainIDLPath, main, includes, false); err != nil {
		// fall back to the original way (without dynamicgo)
		p.opts.DynamicGoExpected = false
		klog.CtxWarnf(context.Background(), "KITEX: failed to get dynamicgo service descriptor, fall back to the original way, error=%s", err)
		return p, nil
	}

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

	if p.opts.DynamicGoExpected {
		if err = newDynamicgoDscFromContent(svc, defaultMainIDLPath, main, includes, false); err != nil {
			p.opts.DynamicGoExpected = false
			klog.CtxWarnf(context.Background(), "KITEX: failed to get dynamicgo service descriptor, fall back to the original way, error=%s", err)
		}
	}

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

// Provide ...
func (p *ThriftContentProvider) Provide() <-chan *descriptor.ServiceDescriptor {
	return p.svcs
}

// Close the sending chan.
func (p *ThriftContentProvider) Close() error {
	p.closeOnce.Do(func() {
		close(p.svcs)
	})
	return nil
}

// Option ...
func (p *ThriftContentProvider) Option() ProviderOption {
	return *p.opts
}

func parseIncludes(tree *parser.Thrift, parsed map[string]*parser.Thrift, sources map[string]string, isAbsIncludePath bool) (err error) {
	for _, i := range tree.Includes {
		p := i.Path
		if isAbsIncludePath {
			p = absPath(tree.Filename, i.Path)
		}
		ref, ok := parsed[p] // avoid infinite recursion
		if ok {
			i.Reference = ref
			continue
		}
		if src, ok := sources[p]; !ok {
			return fmt.Errorf("miss include path: %s for file: %s", p, tree.Filename)
		} else {
			if ref, err = parser.ParseString(p, src); err != nil {
				return
			}
		}
		parsed[p] = ref
		i.Reference = ref
		if err = parseIncludes(ref, parsed, sources, isAbsIncludePath); err != nil {
			return
		}
	}
	return nil
}

// path := /a/b/c.thrift
// includePath := ../d.thrift
// result := /a/d.thrift
func absPath(path, includePath string) string {
	if filepath.IsAbs(includePath) {
		return includePath
	}
	return filepath.Join(filepath.Dir(path), includePath)
}

// ParseContent parses the IDL from path and content using provided includes
func ParseContent(path, content string, includes map[string]string, isAbsIncludePath bool) (*parser.Thrift, error) {
	if src := includes[path]; src != "" && src != content {
		return nil, fmt.Errorf("provided main IDL content conflicts with includes: %q", path)
	}
	tree, err := parser.ParseString(path, content)
	if err != nil {
		return nil, err
	}
	parsed := make(map[string]*parser.Thrift)
	parsed[path] = tree
	if err := parseIncludes(tree, parsed, includes, isAbsIncludePath); err != nil {
		return nil, err
	}
	if cir := parser.CircleDetect(tree); cir != "" {
		return tree, fmt.Errorf("IDL circular dependency: %s", cir)
	}
	return tree, nil
}

// ThriftContentWithAbsIncludePathProvider ...
type ThriftContentWithAbsIncludePathProvider struct {
	closeOnce sync.Once
	svcs      chan *descriptor.ServiceDescriptor
	opts      *ProviderOption
}

var _ DescriptorProvider = (*ThriftContentWithAbsIncludePathProvider)(nil)

// NewThriftContentWithAbsIncludePathProvider create abs include path DescriptorProvider
func NewThriftContentWithAbsIncludePathProvider(mainIDLPath string, includes map[string]string) (*ThriftContentWithAbsIncludePathProvider, error) {
	p := &ThriftContentWithAbsIncludePathProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts: &ProviderOption{DynamicGoExpected: false},
	}
	mainIDLContent, ok := includes[mainIDLPath]
	if !ok {
		return nil, fmt.Errorf("miss main IDL content for main IDL path: %s", mainIDLPath)
	}
	svc, err := newServiceDescriptorFromContent(mainIDLPath, mainIDLContent, includes, true)
	if err != nil {
		return nil, err
	}

	p.svcs <- svc
	return p, nil
}

// NewThriftContentWithAbsIncludePathProviderWithDynamicGo create abs include path DescriptorProvider with dynamicgo
func NewThriftContentWithAbsIncludePathProviderWithDynamicGo(mainIDLPath string, includes map[string]string) (*ThriftContentWithAbsIncludePathProvider, error) {
	p := &ThriftContentWithAbsIncludePathProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts: &ProviderOption{DynamicGoExpected: true},
	}
	mainIDLContent, ok := includes[mainIDLPath]
	if !ok {
		return nil, fmt.Errorf("miss main IDL content for main IDL path: %s", mainIDLPath)
	}
	svc, err := newServiceDescriptorFromContent(mainIDLPath, mainIDLContent, includes, true)
	if err != nil {
		return nil, err
	}

	if err = newDynamicgoDscFromContent(svc, mainIDLPath, mainIDLContent, includes, true); err != nil {
		// fall back to the original way (without dynamicgo)
		p.opts.DynamicGoExpected = false
		klog.CtxWarnf(context.Background(), "KITEX: failed to get dynamicgo service descriptor, fall back to the original way, error=%s", err)
		return p, nil
	}

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

	if p.opts.DynamicGoExpected {
		if err = newDynamicgoDscFromContent(svc, mainIDLPath, mainIDLContent, includes, true); err != nil {
			p.opts.DynamicGoExpected = false
			klog.CtxWarnf(context.Background(), "KITEX: failed to get dynamicgo service descriptor, fall back to the original way, error=%s", err)
		}
	}

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

// Provide ...
func (p *ThriftContentWithAbsIncludePathProvider) Provide() <-chan *descriptor.ServiceDescriptor {
	return p.svcs
}

// Close the sending chan.
func (p *ThriftContentWithAbsIncludePathProvider) Close() error {
	p.closeOnce.Do(func() {
		close(p.svcs)
	})
	return nil
}

// Option ...
func (p *ThriftContentWithAbsIncludePathProvider) Option() ProviderOption {
	return *p.opts
}

func newServiceDescriptorFromContent(path, content string, includes map[string]string, isAbsIncludePath bool) (*descriptor.ServiceDescriptor, error) {
	tree, err := ParseContent(path, content, includes, isAbsIncludePath)
	if err != nil {
		return nil, err
	}
	svc, err := thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func newDynamicgoDscFromContent(svc *descriptor.ServiceDescriptor, path, content string, includes map[string]string, isAbsIncludePath bool) error {
	// ServiceDescriptor of dynamicgo
	dsvc, err := dthrift.NewDescritorFromContent(context.Background(), path, content, includes, isAbsIncludePath)
	if err != nil {
		return err
	}
	svc.DynamicGoDsc = dsvc
	return nil
}
