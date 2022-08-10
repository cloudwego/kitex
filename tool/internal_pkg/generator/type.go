// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generator

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

// File .
type File struct {
	Name    string
	Content string
}

// PackageInfo contains information to generate a package for a service.
type PackageInfo struct {
	Namespace    string            // a dot-separated string for generating service package under kitex_gen
	Dependencies map[string]string // package name => import path, used for searching imports
	*ServiceInfo                   // the target service

	// the following fields will be filled and used by the generator
	Codec            string
	NoFastAPI        bool
	Version          string
	RealServiceName  string
	Imports          map[string]string // import path => alias
	ExternalKitexGen string
	Features         []feature
}

// AddImport .
func (p *PackageInfo) AddImport(pkg, path string) {
	if pkg != "" {
		if p.ExternalKitexGen != "" && strings.Contains(path, KitexGenPath) {
			parts := strings.Split(path, KitexGenPath)
			path = filepath.Join(p.ExternalKitexGen, parts[len(parts)-1])
		}
		if path == pkg {
			p.Imports[path] = ""
		} else {
			p.Imports[path] = pkg
		}
	}
}

// AddImports .
func (p *PackageInfo) AddImports(pkgs ...string) {
	for _, pkg := range pkgs {
		if path, ok := p.Dependencies[pkg]; ok {
			p.AddImport(pkg, path)
		} else {
			p.AddImport(pkg, pkg)
		}
	}
}

// PkgInfo .
type PkgInfo struct {
	PkgName    string
	PkgRefName string
	ImportPath string
}

// ServiceInfo .
type ServiceInfo struct {
	PkgInfo
	ServiceName     string
	RawServiceName  string
	ServiceTypeName func() string
	Base            *ServiceInfo
	Methods         []*MethodInfo
	CombineServices []*ServiceInfo
	HasStreaming    bool
}

// AllMethods returns all methods that the service have.
func (s *ServiceInfo) AllMethods() (ms []*MethodInfo) {
	ms = s.Methods
	for base := s.Base; base != nil; base = base.Base {
		ms = append(base.Methods, ms...)
	}
	return ms
}

// MethodInfo .
type MethodInfo struct {
	PkgInfo
	ServiceName            string
	Name                   string
	RawName                string
	Oneway                 bool
	Void                   bool
	Args                   []*Parameter
	Resp                   *Parameter
	Exceptions             []*Parameter
	ArgStructName          string
	ResStructName          string
	IsResponseNeedRedirect bool // int -> int*
	GenArgResultStruct     bool
	ClientStreaming        bool
	ServerStreaming        bool
}

// Parameter .
type Parameter struct {
	Deps    []PkgInfo
	Name    string
	RawName string
	Type    string // *PkgA.StructB
}

var funcs = map[string]interface{}{
	"ToLower":    strings.ToLower,
	"LowerFirst": util.LowerFirst,
	"UpperFirst": util.UpperFirst,
	"NotPtr":     util.NotPtr,
	"HasFeature": HasFeature,
}

var templateNames = []string{
	"@client.go-NewClient-option",
	"@client.go-EOF",
	"@invoker.go-NewInvoker-option",
	"@invoker.go-EOF",
	"@server.go-NewServer-option",
	"@server.go-EOF",
}

var templateExtensions = (func() map[string]string {
	m := make(map[string]string)
	for _, name := range templateNames {
		// create dummy templates
		m[name] = fmt.Sprintf(`{{define "%s"}}{{end}}`, name)
	}
	return m
})()

// SetTemplateExtension .
func SetTemplateExtension(name, text string) {
	if _, ok := templateExtensions[name]; ok {
		templateExtensions[name] = text
	}
}

// Task .
type Task struct {
	Name string
	Path string
	Text string
	*template.Template
}

// Build .
func (t *Task) Build() error {
	x, err := template.New(t.Name).Funcs(funcs).Parse(t.Text)
	if err != nil {
		return err
	}
	for _, n := range templateNames {
		x, err = x.Parse(templateExtensions[n])
		if err != nil {
			return fmt.Errorf("failed to parse extension %q for %s: %w (%#q)",
				n, t.Name, err, templateExtensions[n])
		}
	}
	t.Template = x
	return nil
}

// Render .
func (t *Task) Render(data interface{}) (*File, error) {
	if t.Template == nil {
		err := t.Build()
		if err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	err := t.ExecuteTemplate(&buf, t.Name, data)
	if err != nil {
		return nil, err
	}
	return &File{t.Path, buf.String()}, nil
}
