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

// Package generator .
package generator

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/tpl"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

// Constants .
const (
	KitexGenPath    = "kitex_gen"
	KitexImportPath = "github.com/cloudwego/kitex"
	DefaultCodec    = "thrift"

	BuildFileName     = "build.sh"
	BootstrapFileName = "bootstrap.sh"
	HandlerFileName   = "handler.go"
	MainFileName      = "main.go"
	ClientFileName    = "client.go"
	ServerFileName    = "server.go"
	InvokerFileName   = "invoker.go"
	ServiceFileName   = "*service.go"
)

var (
	globalMiddlewares  []Middleware
	globalDependencies = map[string]string{
		"kitex":   KitexImportPath,
		"client":  filepath.Join(KitexImportPath, "client"),
		"server":  filepath.Join(KitexImportPath, "server"),
		"callopt": filepath.Join(KitexImportPath, "client/callopt"),
	}
)

// AddGlobalMiddleware adds middleware for all generators
func AddGlobalMiddleware(mw Middleware) {
	globalMiddlewares = append(globalMiddlewares, mw)
}

// AddGlobalDependency adds dependency for all generators
func AddGlobalDependency(ref, path string) bool {
	if _, ok := globalDependencies[ref]; !ok {
		globalDependencies[ref] = path
		return true
	}
	return false
}

// Generator generates the codes of main package and scripts for building a server based on kitex.
type Generator interface {
	GenerateService(pkg *PackageInfo) ([]*File, error)
	GenerateMainPackage(pkg *PackageInfo) ([]*File, error)
}

// Config .
type Config struct {
	Verbose         bool
	GenerateMain    bool // whether stuff in the main package should be generated
	GenerateInvoker bool // generate main.go with invoker when main package generate
	Version         string
	NoFastAPI       bool
	ModuleName      string
	ServiceName     string
	Use             string
	IDLType         string
	Includes        util.StringSlice
	ThriftOptions   util.StringSlice
	ProtobufOptions util.StringSlice
	IDL             string // the IDL file passed on the command line
	OutputPath      string // the output path for main pkg and kitex_gen
	PackagePrefix   string
	CombineService  bool // combine services to one service
	CopyIDL         bool
	ThriftPlugins   util.StringSlice
}

// Pack packs the Config into a slice of "key=val" strings.
func (c *Config) Pack() (res []string) {
	t := reflect.TypeOf(c).Elem()
	v := reflect.ValueOf(c).Elem()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		x := v.Field(i)
		n := f.Name

		// skip the plugin arguments to avoid the 'strings in strings' trouble
		if f.Name == "ThriftPlugins" {
			continue
		}

		switch x.Kind() {
		case reflect.Bool:
			res = append(res, n+"="+fmt.Sprint(x.Bool()))
		case reflect.String:
			res = append(res, n+"="+x.String())
		case reflect.Slice:
			var ss []string
			if x.Type().Elem().Kind() == reflect.Int {
				for i := 0; i < x.Len(); i++ {
					ss = append(ss, strconv.Itoa(int(x.Index(i).Int())))
				}
			} else {
				for i := 0; i < x.Len(); i++ {
					ss = append(ss, x.Index(i).String())
				}
			}
			res = append(res, n+"="+strings.Join(ss, ";"))
		default:
			panic(fmt.Errorf("unsupported field type: %+v", f))
		}
	}
	return res
}

// Unpack restores the Config from a slice of "key=val" strings.
func (c *Config) Unpack(args []string) error {
	t := reflect.TypeOf(c).Elem()
	v := reflect.ValueOf(c).Elem()
	for _, a := range args {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid argument: '%s'", a)
		}
		name, value := parts[0], parts[1]
		f, ok := t.FieldByName(name)
		if ok && value != "" {
			x := v.FieldByName(name)
			switch x.Kind() {
			case reflect.Bool:
				x.SetBool(value == "true")
			case reflect.String:
				x.SetString(value)
			case reflect.Slice:
				ss := strings.Split(value, ";")
				if x.Type().Elem().Kind() == reflect.Int {
					n := reflect.MakeSlice(x.Type(), len(ss), len(ss))
					for i, s := range ss {
						val, err := strconv.ParseInt(s, 10, 64)
						if err != nil {
							return err
						}
						n.Index(i).SetInt(val)
					}
					x.Set(n)
				} else {
					for _, s := range ss {
						val := reflect.Append(x, reflect.ValueOf(s))
						x.Set(val)
					}
				}
			default:
				return fmt.Errorf("unsupported field type: %+v", f)
			}
		}
	}
	return nil
}

// NewGenerator .
func NewGenerator(config *Config, middlewares []Middleware) Generator {
	mws := append(globalMiddlewares, middlewares...)
	g := &generator{Config: config, middlewares: mws}
	if g.IDLType == "" {
		g.IDLType = DefaultCodec
	}
	return g
}

// Middleware used generator
type Middleware func(HandleFunc) HandleFunc

// HandleFunc used generator
type HandleFunc func(*Task, *PackageInfo) (*File, error)

type generator struct {
	*Config
	middlewares []Middleware
}

func (g *generator) chainMWs(handle HandleFunc) HandleFunc {
	for i := len(g.middlewares) - 1; i > -1; i-- {
		handle = g.middlewares[i](handle)
	}
	return handle
}

func (g *generator) GenerateMainPackage(pkg *PackageInfo) (fs []*File, err error) {
	g.updatePackageInfo(pkg)

	tasks := []*Task{
		{
			Name: BuildFileName,
			Path: filepath.Join(g.OutputPath, BuildFileName),
			Text: tpl.BuildTpl,
		},
		{
			Name: BootstrapFileName,
			Path: filepath.Join(g.OutputPath, "script", BootstrapFileName),
			Text: tpl.BootstrapTpl,
		},
	}
	if !g.Config.GenerateInvoker {
		tasks = append(tasks, &Task{
			Name: MainFileName,
			Path: filepath.Join(g.OutputPath, MainFileName),
			Text: tpl.MainTpl,
		})
	}
	for _, t := range tasks {
		if util.Exists(t.Path) {
			log.Info(t.Path, "exists. Skipped.")
			continue
		}
		g.setImports(t.Name, pkg)
		handle := func(task *Task, pkg *PackageInfo) (*File, error) {
			return task.Render(pkg)
		}
		f, err := g.chainMWs(handle)(t, pkg)
		if err != nil {
			return nil, err
		}
		fs = append(fs, f)
	}

	handlerFilePath := filepath.Join(g.OutputPath, HandlerFileName)
	if util.Exists(handlerFilePath) {
		comp := newCompleter(
			pkg.ServiceInfo.AllMethods(),
			handlerFilePath,
			pkg.ServiceInfo.ServiceName)
		f, err := comp.CompleteMethods()
		if err != nil {
			if err == errNoNewMethod {
				return fs, nil
			}
			return nil, err
		}
		fs = append(fs, f)
	} else {
		task := Task{
			Name: HandlerFileName,
			Path: handlerFilePath,
			Text: tpl.HandlerTpl,
		}
		g.setImports(task.Name, pkg)
		handle := func(task *Task, pkg *PackageInfo) (*File, error) {
			return task.Render(pkg)
		}
		f, err := g.chainMWs(handle)(&task, pkg)
		if err != nil {
			return nil, err
		}
		fs = append(fs, f)
	}
	return
}

func (g *generator) GenerateService(pkg *PackageInfo) ([]*File, error) {
	g.updatePackageInfo(pkg)
	output := filepath.Join(g.OutputPath, KitexGenPath)
	if pkg.Namespace != "" {
		output = filepath.Join(output, strings.ReplaceAll(pkg.Namespace, ".", "/"))
	}
	svcPkg := strings.ToLower(pkg.ServiceName)
	output = filepath.Join(output, svcPkg)

	tasks := []*Task{
		{
			Name: ClientFileName,
			Path: filepath.Join(output, ClientFileName),
			Text: tpl.ClientTpl,
		},
		{
			Name: ServerFileName,
			Path: filepath.Join(output, ServerFileName),
			Text: tpl.ServerTpl,
		},
		{
			Name: InvokerFileName,
			Path: filepath.Join(output, InvokerFileName),
			Text: tpl.InvokerTpl,
		},
		{
			Name: ServiceFileName,
			Path: filepath.Join(output, svcPkg+".go"),
			Text: tpl.ServiceTpl,
		},
	}

	var fs []*File
	for _, t := range tasks {
		if err := t.Build(); err != nil {
			err = fmt.Errorf("build %s failed: %w", t.Name, err)
			return nil, err
		}
		g.setImports(t.Name, pkg)
		handle := func(task *Task, pkg *PackageInfo) (*File, error) {
			return task.Render(pkg)
		}
		f, err := g.chainMWs(handle)(t, pkg)
		if err != nil {
			err = fmt.Errorf("render %s failed: %w", t.Name, err)
			return nil, err
		}
		fs = append(fs, f)
	}
	return fs, nil
}

func (g *generator) updatePackageInfo(pkg *PackageInfo) {
	pkg.Codec = g.IDLType
	pkg.Version = g.Version
	pkg.RealServiceName = g.ServiceName
	pkg.ExternalKitexGen = g.Use
	if pkg.Dependencies == nil {
		pkg.Dependencies = make(map[string]string)
	}

	for ref, path := range globalDependencies {
		if _, ok := pkg.Dependencies[ref]; !ok {
			pkg.Dependencies[ref] = path
		}
	}
}

func (g *generator) setImports(name string, pkg *PackageInfo) {
	pkg.Imports = make(map[string]string)
	switch name {
	case ClientFileName:
		pkg.AddImports("client")
		if pkg.HasStreaming {
			pkg.AddImport("streaming", filepath.Join(KitexImportPath, "pkg/streaming"))
			pkg.AddImport("transport", filepath.Join(KitexImportPath, "transport"))
		}
		if len(pkg.AllMethods()) > 0 {
			pkg.AddImports("callopt")
		}
		fallthrough
	case HandlerFileName:
		if len(pkg.AllMethods()) > 0 {
			pkg.AddImports("context")
		}
		for _, m := range pkg.ServiceInfo.AllMethods() {
			for _, a := range m.Args {
				for _, dep := range a.Deps {
					pkg.AddImport(dep.PkgRefName, dep.ImportPath)
				}
			}
			if !m.Void && m.Resp != nil {
				for _, dep := range m.Resp.Deps {
					pkg.AddImport(dep.PkgRefName, dep.ImportPath)
				}
			}
		}
	case ServerFileName, InvokerFileName:
		if len(pkg.CombineServices) == 0 {
			pkg.AddImport(pkg.ServiceInfo.PkgRefName, pkg.ServiceInfo.ImportPath)
		}
		pkg.AddImports("server")
	case ServiceFileName:
		pkg.AddImports("client")
		pkg.AddImport("kitex", filepath.Join(KitexImportPath, "pkg/serviceinfo"))
		pkg.AddImport(pkg.ServiceInfo.PkgRefName, pkg.ServiceInfo.ImportPath)
		if len(pkg.AllMethods()) > 0 {
			pkg.AddImports("context")
		}
		for _, m := range pkg.ServiceInfo.AllMethods() {
			if m.GenArgResultStruct {
				pkg.AddImports("fmt", "proto")
			} else {
				// for method Arg and Result
				pkg.AddImport(m.PkgRefName, m.ImportPath)
			}
			for _, a := range m.Args {
				for _, dep := range a.Deps {
					pkg.AddImport(dep.PkgRefName, dep.ImportPath)
				}
			}
			if pkg.Codec == "protobuf" {
				pkg.AddImport("streaming", filepath.Join(KitexImportPath, "pkg/streaming"))
			}
			if !m.Void && m.Resp != nil {
				for _, dep := range m.Resp.Deps {
					pkg.AddImport(dep.PkgRefName, dep.ImportPath)
				}
			}
			for _, e := range m.Exceptions {
				for _, dep := range e.Deps {
					pkg.AddImport(dep.PkgRefName, dep.ImportPath)
				}
			}
		}
	case MainFileName:
		pkg.AddImport("log", "log")
		pkg.AddImport(pkg.PkgRefName, filepath.Join(pkg.ImportPath, strings.ToLower(pkg.ServiceName)))
	}
}
