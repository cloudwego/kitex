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
	"os"
	"path/filepath"
	"sync"

	"github.com/cloudwego/dynamicgo/meta"
	dthrift "github.com/cloudwego/dynamicgo/thrift"
	"github.com/cloudwego/thriftgo/parser"

	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/klog"
)

var (
	_ Closer = &ThriftContentProvider{}
	_ Closer = &ThriftContentWithAbsIncludePathProvider{}

	isGoTagAliasDisabled = os.Getenv("KITEX_GENERIC_GOTAG_ALIAS_DISABLED") == "True"
	goTagMapper          = dthrift.FindAnnotationMapper("go.tag", dthrift.AnnoScopeField)
)

type thriftFileProvider struct {
	closeOnce sync.Once
	svcs      chan *descriptor.ServiceDescriptor
	opts      *ProviderOption
}

// NewThriftFileProvider create a ThriftIDLProvider by given path and include dirs
func NewThriftFileProvider(path string, includeDirs ...string) (DescriptorProvider, error) {
	return NewThriftFileProviderWithOption(path, []ThriftIDLProviderOption{}, includeDirs...)
}

func NewThriftFileProviderWithOption(path string, opts []ThriftIDLProviderOption, includeDirs ...string) (DescriptorProvider, error) {
	p := &thriftFileProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts: &ProviderOption{DynamicGoEnabled: false},
	}
	tOpts := &thriftIDLProviderOptions{}
	tOpts.apply(opts)
	svc, err := newServiceDescriptorFromPath(path, getParseMode(tOpts), tOpts.goTag, includeDirs...)
	if err != nil {
		return nil, err
	}
	p.svcs <- svc
	return p, nil
}

// NewThriftFileProviderWithDynamicGo create a ThriftIDLProvider with dynamicgo by given path and include dirs
func NewThriftFileProviderWithDynamicGo(path string, includeDirs ...string) (DescriptorProvider, error) {
	return NewThriftFileProviderWithDynamicgoWithOption(path, []ThriftIDLProviderOption{}, includeDirs...)
}

func NewThriftFileProviderWithDynamicgoWithOption(path string, opts []ThriftIDLProviderOption, includeDirs ...string) (DescriptorProvider, error) {
	p := &thriftFileProvider{
		svcs: make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts: &ProviderOption{DynamicGoEnabled: true},
	}
	tOpts := &thriftIDLProviderOptions{}
	tOpts.apply(opts)
	parseMode := getParseMode(tOpts)
	svc, err := newServiceDescriptorFromPath(path, parseMode, tOpts.goTag, includeDirs...)
	if err != nil {
		return nil, err
	}

	// ServiceDescriptor of dynamicgo
	dParseMode, err := getDynamicGoParseMode(parseMode)
	if err != nil {
		return nil, err
	}
	handleGoTagForDynamicGo(tOpts.goTag)
	dOpts := dthrift.Options{EnableThriftBase: true, ParseServiceMode: dParseMode, UseDefaultValue: true, SetOptionalBitmap: true}
	dsvc, err := dOpts.NewDescritorFromPath(context.Background(), path, includeDirs...)
	if err != nil {
		// fall back to the original way (without dynamicgo)
		p.opts.DynamicGoEnabled = false
		p.svcs <- svc
		klog.CtxWarnf(context.Background(), "KITEX: failed to get dynamicgo service descriptor, fall back to the original way, error=%s", err)
		return p, nil
	}
	svc.DynamicGoDsc = dsvc

	p.svcs <- svc
	return p, nil
}

func newServiceDescriptorFromPath(path string, parseMode thrift.ParseMode, goTagOpt *goTagOption, includeDirs ...string) (*descriptor.ServiceDescriptor, error) {
	tree, err := parser.ParseFile(path, includeDirs, true)
	if err != nil {
		return nil, err
	}
	var parseOpts []thrift.ParseOption
	if goTagOpt != nil {
		parseOpts = append(parseOpts, thrift.WithGoTagDisabled(goTagOpt.isGoTagAliasDisabled))
	}
	svc, err := thrift.Parse(tree, parseMode, parseOpts...)
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
	parseMode thrift.ParseMode
	goTagOpt  *goTagOption
}

var _ DescriptorProvider = (*ThriftContentProvider)(nil)

const defaultMainIDLPath = "main.thrift"

// NewThriftContentProvider builder
func NewThriftContentProvider(main string, includes map[string]string, opts ...ThriftIDLProviderOption) (*ThriftContentProvider, error) {
	tOpts := &thriftIDLProviderOptions{}
	tOpts.apply(opts)
	parseMode := getParseMode(tOpts)

	p := &ThriftContentProvider{
		svcs:      make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts:      &ProviderOption{DynamicGoEnabled: false},
		parseMode: parseMode,
		goTagOpt:  tOpts.goTag,
	}
	svc, err := newServiceDescriptorFromContent(defaultMainIDLPath, main, includes, false, parseMode, tOpts.goTag)
	if err != nil {
		return nil, err
	}

	p.svcs <- svc
	return p, nil
}

// NewThriftContentProviderWithDynamicGo builder
func NewThriftContentProviderWithDynamicGo(main string, includes map[string]string, opts ...ThriftIDLProviderOption) (*ThriftContentProvider, error) {
	tOpts := &thriftIDLProviderOptions{}
	tOpts.apply(opts)
	parseMode := getParseMode(tOpts)

	p := &ThriftContentProvider{
		svcs:      make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts:      &ProviderOption{DynamicGoEnabled: true},
		parseMode: parseMode,
		goTagOpt:  tOpts.goTag,
	}

	svc, err := newServiceDescriptorFromContent(defaultMainIDLPath, main, includes, false, parseMode, tOpts.goTag)
	if err != nil {
		return nil, err
	}

	p.newDynamicGoDsc(svc, defaultMainIDLPath, main, includes, parseMode, tOpts.goTag)

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
	var parseOpts []thrift.ParseOption
	if p.goTagOpt != nil {
		parseOpts = append(parseOpts, thrift.WithGoTagDisabled(p.goTagOpt.isGoTagAliasDisabled))
	}
	svc, err = thrift.Parse(tree, p.parseMode, parseOpts...)
	if err != nil {
		return err
	}

	if p.opts.DynamicGoEnabled {
		p.newDynamicGoDsc(svc, defaultMainIDLPath, main, includes, p.parseMode, p.goTagOpt)
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

func (p *ThriftContentProvider) newDynamicGoDsc(svc *descriptor.ServiceDescriptor, path, content string, includes map[string]string, parseMode thrift.ParseMode, goTag *goTagOption) {
	if err := newDynamicGoDscFromContent(svc, path, content, includes, false, parseMode, goTag); err != nil {
		p.opts.DynamicGoEnabled = false
	}
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
	parseMode thrift.ParseMode
	goTagOpt  *goTagOption
}

var _ DescriptorProvider = (*ThriftContentWithAbsIncludePathProvider)(nil)

// NewThriftContentWithAbsIncludePathProvider create abs include path DescriptorProvider
func NewThriftContentWithAbsIncludePathProvider(mainIDLPath string, includes map[string]string, opts ...ThriftIDLProviderOption) (*ThriftContentWithAbsIncludePathProvider, error) {
	tOpts := &thriftIDLProviderOptions{}
	tOpts.apply(opts)
	parseMode := getParseMode(tOpts)

	p := &ThriftContentWithAbsIncludePathProvider{
		svcs:      make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts:      &ProviderOption{DynamicGoEnabled: false},
		parseMode: parseMode,
		goTagOpt:  tOpts.goTag,
	}
	mainIDLContent, ok := includes[mainIDLPath]
	if !ok {
		return nil, fmt.Errorf("miss main IDL content for main IDL path: %s", mainIDLPath)
	}

	svc, err := newServiceDescriptorFromContent(mainIDLPath, mainIDLContent, includes, true, parseMode, tOpts.goTag)
	if err != nil {
		return nil, err
	}

	p.svcs <- svc
	return p, nil
}

// NewThriftContentWithAbsIncludePathProviderWithDynamicGo create abs include path DescriptorProvider with dynamicgo
func NewThriftContentWithAbsIncludePathProviderWithDynamicGo(mainIDLPath string, includes map[string]string, opts ...ThriftIDLProviderOption) (*ThriftContentWithAbsIncludePathProvider, error) {
	tOpts := &thriftIDLProviderOptions{}
	tOpts.apply(opts)
	parseMode := getParseMode(tOpts)

	p := &ThriftContentWithAbsIncludePathProvider{
		svcs:      make(chan *descriptor.ServiceDescriptor, 1), // unblock with buffered channel
		opts:      &ProviderOption{DynamicGoEnabled: true},
		parseMode: parseMode,
		goTagOpt:  tOpts.goTag,
	}
	mainIDLContent, ok := includes[mainIDLPath]
	if !ok {
		return nil, fmt.Errorf("miss main IDL content for main IDL path: %s", mainIDLPath)
	}

	svc, err := newServiceDescriptorFromContent(mainIDLPath, mainIDLContent, includes, true, parseMode, tOpts.goTag)
	if err != nil {
		return nil, err
	}

	p.newDynamicGoDsc(svc, mainIDLPath, mainIDLContent, includes, parseMode, tOpts.goTag)

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
	var parseOpts []thrift.ParseOption
	if p.goTagOpt != nil {
		parseOpts = append(parseOpts, thrift.WithGoTagDisabled(p.goTagOpt.isGoTagAliasDisabled))
	}
	svc, err = thrift.Parse(tree, p.parseMode, parseOpts...)
	if err != nil {
		return err
	}

	if p.opts.DynamicGoEnabled {
		p.newDynamicGoDsc(svc, mainIDLPath, mainIDLContent, includes, p.parseMode, p.goTagOpt)
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

func (p *ThriftContentWithAbsIncludePathProvider) newDynamicGoDsc(svc *descriptor.ServiceDescriptor, path, content string, includes map[string]string, parseMode thrift.ParseMode, goTag *goTagOption) {
	if err := newDynamicGoDscFromContent(svc, path, content, includes, true, parseMode, goTag); err != nil {
		p.opts.DynamicGoEnabled = false
	}
}

func getParseMode(opt *thriftIDLProviderOptions) thrift.ParseMode {
	parseMode := thrift.DefaultParseMode()
	if opt.parseMode != nil {
		parseMode = *opt.parseMode
	}
	return parseMode
}

func getDynamicGoParseMode(parseMode thrift.ParseMode) (meta.ParseServiceMode, error) {
	switch parseMode {
	case thrift.FirstServiceOnly:
		return meta.FirstServiceOnly, nil
	case thrift.LastServiceOnly:
		return meta.LastServiceOnly, nil
	case thrift.CombineServices:
		return meta.CombineServices, nil
	default:
		return -1, fmt.Errorf("invalid parse mode: %d", parseMode)
	}
}

func newServiceDescriptorFromContent(path, content string, includes map[string]string, isAbsIncludePath bool, parseMode thrift.ParseMode, goTagOpt *goTagOption) (*descriptor.ServiceDescriptor, error) {
	tree, err := ParseContent(path, content, includes, isAbsIncludePath)
	if err != nil {
		return nil, err
	}
	var parseOpts []thrift.ParseOption
	if goTagOpt != nil {
		parseOpts = append(parseOpts, thrift.WithGoTagDisabled(goTagOpt.isGoTagAliasDisabled))
	}
	svc, err := thrift.Parse(tree, parseMode, parseOpts...)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func newDynamicGoDscFromContent(svc *descriptor.ServiceDescriptor, path, content string, includes map[string]string, isAbsIncludePath bool, parseMode thrift.ParseMode, goTag *goTagOption) error {
	handleGoTagForDynamicGo(goTag)
	// ServiceDescriptor of dynamicgo
	dParseMode, err := getDynamicGoParseMode(parseMode)
	if err != nil {
		return err
	}
	dOpts := dthrift.Options{EnableThriftBase: true, ParseServiceMode: dParseMode, UseDefaultValue: true, SetOptionalBitmap: true}
	dsvc, err := dOpts.NewDescritorFromContent(context.Background(), path, content, includes, isAbsIncludePath)
	if err != nil {
		klog.CtxWarnf(context.Background(), "KITEX: failed to get dynamicgo service descriptor, fall back to the original way, error=%s", err)
		return err
	}
	svc.DynamicGoDsc = dsvc
	return nil
}

func handleGoTagForDynamicGo(goTagOpt *goTagOption) {
	shouldRemove := isGoTagAliasDisabled
	if goTagOpt != nil {
		shouldRemove = goTagOpt.isGoTagAliasDisabled
	}
	if shouldRemove {
		dthrift.RemoveAnnotationMapper(dthrift.AnnoScopeField, "go.tag")
	} else {
		dthrift.RegisterAnnotationMapper(dthrift.AnnoScopeField, goTagMapper, "go.tag")
	}
}
