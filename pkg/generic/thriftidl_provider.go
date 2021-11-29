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
	"fmt"
	"path/filepath"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/thriftgo/parser"
)

type thriftFileProvider struct {
	svcs chan *descriptor.ServiceDescriptor
}

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
	p.svcs <- svc
	return p, nil
}

// maybe watch the file change and reparse
// p.thrifts <- some change
// func (p *thriftFileProvider) watchAndUpdate() {}

func (p *thriftFileProvider) Provide() <-chan *descriptor.ServiceDescriptor {
	return p.svcs
}

// ThriftContentProvider provide descriptor from contents
type ThriftContentProvider struct {
	svcs chan *descriptor.ServiceDescriptor
}

var _ DescriptorProvider = (*ThriftContentProvider)(nil)

const defaultMainIDLPath = "main.thrift"

// NewThriftContentProvider builder
func NewThriftContentProvider(main string, includes map[string]string) (*ThriftContentProvider, error) {
	p := &ThriftContentProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
	}
	tree, err := parseContent(defaultMainIDLPath, main, includes, false)
	if err != nil {
		return nil, err
	}
	svc, err := thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return nil, err
	}
	p.svcs <- svc
	return p, nil
}

// UpdateIDL ...
func (p *ThriftContentProvider) UpdateIDL(main string, includes map[string]string) error {
	tree, err := parseContent(defaultMainIDLPath, main, includes, false)
	if err != nil {
		return err
	}
	svc, err := thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return err
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

func parseIncludes(tree *parser.Thrift, includes map[string]*parser.Thrift, isAbsIncludePath bool) error {
	for _, i := range tree.Includes {
		p := i.Path
		if isAbsIncludePath {
			p = absPath(tree.Filename, i.Path)
		}
		ref, ok := includes[p]
		if !ok {
			return fmt.Errorf("miss include path: %s for file: %s", p, tree.Filename)
		}
		if err := parseIncludes(ref, includes, isAbsIncludePath); err != nil {
			return err
		}
		i.Reference = ref
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

func parseContent(path, content string, includes map[string]string, isAbsIncludePath bool) (*parser.Thrift, error) {
	tree, err := parser.ParseString(path, content)
	if err != nil {
		return nil, err
	}
	_includes := make(map[string]*parser.Thrift, len(includes))
	for k, v := range includes {
		t, err := parser.ParseString(k, v)
		if err != nil {
			return nil, err
		}
		_includes[k] = t
	}
	if err := parseIncludes(tree, _includes, isAbsIncludePath); err != nil {
		return nil, err
	}
	return tree, nil
}

// ThriftContentWithAbsIncludePathProvider ...
type ThriftContentWithAbsIncludePathProvider struct {
	svcs chan *descriptor.ServiceDescriptor
}

var _ DescriptorProvider = (*ThriftContentWithAbsIncludePathProvider)(nil)

// NewThriftContentWithAbsIncludePathProvider create abs include path DescriptorProvider
func NewThriftContentWithAbsIncludePathProvider(mainIDLPath string, includes map[string]string) (*ThriftContentWithAbsIncludePathProvider, error) {
	p := &ThriftContentWithAbsIncludePathProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
	}
	mainIDLContent, ok := includes[mainIDLPath]
	if !ok {
		return nil, fmt.Errorf("miss main IDL content for main IDL path: %s", mainIDLPath)
	}
	tree, err := parseContent(mainIDLPath, mainIDLContent, includes, true)
	if err != nil {
		return nil, err
	}
	svc, err := thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return nil, err
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
	tree, err := parseContent(mainIDLPath, mainIDLContent, includes, true)
	if err != nil {
		return err
	}
	svc, err := thrift.Parse(tree, thrift.DefaultParseMode())
	if err != nil {
		return err
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
