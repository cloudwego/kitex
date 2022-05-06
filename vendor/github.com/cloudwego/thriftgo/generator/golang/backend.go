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

package golang

import (
	"fmt"
	"go/format"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cloudwego/thriftgo/generator/backend"
	"github.com/cloudwego/thriftgo/generator/golang/templates"
	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/plugin"
)

// GoBackend generates go codes.
// The zero value of GoBackend is ready for use.
type GoBackend struct {
	err error
	tpl *template.Template
	req *plugin.Request
	res *plugin.Response
	log backend.LogFunc

	utils *CodeUtils
	funcs template.FuncMap
}

// Name implements the Backend interface.
func (g *GoBackend) Name() string {
	return "go"
}

// Lang implements the Backend interface.
func (g *GoBackend) Lang() string {
	return "Go"
}

// Options implements the Backend interface.
func (g *GoBackend) Options() (opts []plugin.Option) {
	for _, p := range allParams {
		opts = append(opts, plugin.Option{
			Name: p.name,
			Desc: p.desc,
		})
	}
	return opts
}

// BuiltinPlugins implements the Backend interface.
func (g *GoBackend) BuiltinPlugins() []*plugin.Desc {
	return nil
}

// GetPlugin implements the Backend interface.
func (g *GoBackend) GetPlugin(desc *plugin.Desc) plugin.Plugin {
	return nil
}

// Generate implements the Backend interface.
func (g *GoBackend) Generate(req *plugin.Request, log backend.LogFunc) *plugin.Response {
	g.req = req
	g.res = plugin.NewResponse()
	g.log = log
	g.prepareUtilities()
	g.prepareTemplates()
	g.fillRequisitions()
	g.executeTemplates()
	return g.buildResponse()
}

func (g *GoBackend) prepareUtilities() {
	if g.err != nil {
		return
	}

	g.utils = NewCodeUtils(g.log)

	g.err = g.utils.HandleOptions(g.req.GeneratorParameters)
	if g.err != nil {
		return
	}

	g.funcs = g.utils.BuildFuncMap()
	g.funcs["Version"] = func() string { return g.req.Version }
}

func (g *GoBackend) prepareTemplates() {
	if g.err != nil {
		return
	}

	name := "thrift"
	all := template.New(name).Funcs(g.funcs)
	for _, tpl := range templates.Templates() {
		all = template.Must(all.Parse(tpl))
	}

	// XXX(lushaojie): support substutions by all.AddParseTree
	g.tpl = all
}

func (g *GoBackend) fillRequisitions() {
	if g.err != nil {
		return
	}
}

func (g *GoBackend) executeTemplates() {
	if g.err != nil {
		return
	}

	processed := make(map[*parser.Thrift]bool)

	var trees chan *parser.Thrift
	if g.req.Recursive {
		trees = g.req.AST.DepthFirstSearch()
	} else {
		trees = make(chan *parser.Thrift, 1)
		trees <- g.req.AST
		close(trees)
	}

	for ast := range trees {
		if processed[ast] {
			continue
		}
		processed[ast] = true
		g.log.Info("Processing", ast.Filename)

		if g.err = g.renderOneFile(ast); g.err != nil {
			break
		}
	}
}

func (g *GoBackend) renderOneFile(ast *parser.Thrift) error {
	var buf strings.Builder
	scope, err := BuildScope(g.utils, ast)
	if err != nil {
		return err
	}
	g.utils.SetRootScope(scope)

	path := filepath.Join(g.req.OutputPath, g.utils.GetFilePath(ast))
	err = g.tpl.ExecuteTemplate(&buf, g.tpl.Name(), scope)
	if err != nil {
		return fmt.Errorf("%s: %w", ast.Filename, err)
	}

	g.res.Contents = append(g.res.Contents, &plugin.Generated{
		Content: buf.String(),
		Name:    &path,
	})

	buf.Reset()
	imports, err := scope.ResolveImports()
	if err != nil {
		return err
	}
	err = g.tpl.ExecuteTemplate(&buf, "Imports", imports)
	if err != nil {
		return fmt.Errorf("%s: %w", ast.Filename, err)
	}

	point := "imports"
	g.res.Contents = append(g.res.Contents, &plugin.Generated{
		Content:        buf.String(),
		InsertionPoint: &point,
	})
	return nil
}

func (g *GoBackend) buildResponse() *plugin.Response {
	if g.err != nil {
		return plugin.BuildErrorResponse(g.err.Error())
	}

	return g.res
}

// PostProcess implements the backend.PostProcessor interface to do
// source formatting before writing files out.
func (g *GoBackend) PostProcess(path string, content []byte) ([]byte, error) {
	switch filepath.Ext(path) {
	case ".go":
		if formated, err := format.Source(content); err != nil {
			g.log.Warn(fmt.Sprintf("Failed to format %s: %s", path, err.Error()))
		} else {
			content = formated
		}
	}
	return content, nil
}
