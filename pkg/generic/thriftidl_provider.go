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
	"sync"

	"github.com/cloudwego/thriftgo/parser"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
)

var (
	_ Closer = &ThriftContentProvider{}
	_ Closer = &ThriftContentWithAbsIncludePathProvider{}
)

type thriftFileProvider struct {
	closeOnce sync.Once
	svcs      chan *descriptor.ServiceDescriptor
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

// ThriftContentProvider provide descriptor from contents
type ThriftContentProvider struct {
	closeOnce sync.Once
	svcs      chan *descriptor.ServiceDescriptor
}

var _ DescriptorProvider = (*ThriftContentProvider)(nil)

const defaultMainIDLPath = "main.thrift"

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
}

var _ DescriptorProvider = (*ThriftContentWithAbsIncludePathProvider)(nil)

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
