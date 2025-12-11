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

package prutal

import (
	"errors"
	"fmt"
	"go/format"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/tpl/pbtpl"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"

	"github.com/cloudwego/prutal/prutalgen/pkg/prutalgen"
)

type PrutalGen struct {
	g generator.Generator
	c *generator.Config

	pp []*generator.PackageInfo
}

func NewPrutalGen(c generator.Config) *PrutalGen {
	c.NoFastAPI = true
	return &PrutalGen{
		c: &c,
		g: generator.NewGenerator(&c, nil),
	}
}

func (pg *PrutalGen) initPackageInfo(f *prutalgen.Proto) *generator.PackageInfo {
	p := &generator.PackageInfo{}
	c := pg.c
	p.NoFastAPI = true
	p.Namespace = strings.TrimPrefix(f.GoImport, c.PackagePrefix)
	p.IDLName = util.IDLName(c.IDL)
	p.Services = pg.convertTypes(f)
	return p
}

func (pg *PrutalGen) generateClientServerFiles(f *prutalgen.Proto, p *generator.PackageInfo) error {
	log.Debugf("[INFO] Generate %q at %q\n", f.ProtoFile, f.GoImport)
	for _, s := range p.Services {
		p.ServiceInfo = s
		fs, err := pg.g.GenerateService(p)
		if err != nil {
			return err
		}
		for _, f := range fs {
			writeFile(f.Name, []byte(f.Content))
		}
	}
	return nil
}

func (pg *PrutalGen) Process() error {
	c := pg.c
	x := prutalgen.NewLoader(c.Includes, nil)
	x.SetLogger(prutalgen.LoggerFunc(log.DefaultLogger().Printf))

	ff := x.LoadProto(filepath.Clean(c.IDL))

	for _, f := range ff { // fix GoImport
		pkg, ok := getImportPath(f.GoImport, c.ModuleName, c.PackagePrefix)
		if !ok {
			continue
		}
		f.GoImport = pkg
	}

	for i, f := range ff {
		if !strings.HasPrefix(f.GoImport, c.PackagePrefix) {
			if i == 0 {
				log.Warnf("skipped %q, package %q is not under %q. please update go_package option and try again",
					f.ProtoFile, f.GoImport, c.PackagePrefix)
			}
			continue
		}
		srcPath := c.PkgOutputPath(f.GoImport)

		p := pg.initPackageInfo(f)
		pg.pp = append(pg.pp, p)

		if pg.c.Use != "" {
			continue
		}

		// generate the structs and interfaces
		genStructsAndKitexInterfaces(f, c, srcPath)
		if err := pg.generateClientServerFiles(f, p); err != nil {
			return err
		}
	}

	p := pg.pp[0] // the main proto

	// generate main and handler
	if c.GenerateMain {
		if len(p.Services) == 0 {
			return errors.New("no service defined")
		}
		if !c.IsUsingMultipleServicesTpl() {
			// if -tpl multiple_services is not set, specify the last service as the target service
			p.ServiceInfo = p.Services[len(p.Services)-1]
		} else {
			var svcs []*generator.ServiceInfo
			for _, svc := range p.Services {
				if svc.GenerateHandler {
					svc.RefName = "service" + svc.ServiceName
					svcs = append(svcs, svc)
				}
			}
			p.Services = svcs
		}
		fs, err := pg.g.GenerateMainPackage(p)
		if err != nil {
			return err
		}
		for _, f := range fs {
			writeFile(f.Name, []byte(f.Content))
		}
	}

	if c.TemplateDir != "" {
		if len(p.Services) == 0 {
			return errors.New("no service defined")
		}
		p.ServiceInfo = p.Services[len(p.Services)-1]
		fs, err := pg.g.GenerateCustomPackage(p)
		if err != nil {
			return err
		}
		for _, f := range fs {
			writeFile(f.Name, []byte(f.Content))
		}
	}

	// FastPB is deprecated, so truncate all fastpb files
	util.TruncateAllFastPBFiles(filepath.Join(pg.c.OutputPath, pg.c.GenPath))

	return nil
}

func (pg *PrutalGen) convertTypes(f *prutalgen.Proto) (ss []*generator.ServiceInfo) {
	pi := generator.PkgInfo{
		PkgName:    f.Package,
		PkgRefName: f.GoPackage,
		ImportPath: f.GoImport,
	}
	for _, s := range f.Services {
		si := &generator.ServiceInfo{
			PkgInfo:        pi,
			ServiceName:    s.GoName,
			RawServiceName: s.Name,
		}
		si.ServiceTypeName = func() string { return si.PkgRefName + "." + si.ServiceName }
		for _, m := range s.Methods {
			req := pg.convertParameter(f, m.Request, "Req")
			res := pg.convertParameter(f, m.Return, "Resp")

			methodName := m.GoName
			mi := &generator.MethodInfo{
				PkgInfo:            pi,
				ServiceName:        si.ServiceName,
				RawName:            m.Name,
				Name:               methodName,
				Args:               []*generator.Parameter{req},
				Resp:               res,
				ArgStructName:      methodName + "Args",
				ResStructName:      methodName + "Result",
				GenArgResultStruct: true,
				ClientStreaming:    m.RequestStream,
				ServerStreaming:    m.ReturnStream,
			}
			si.Methods = append(si.Methods, mi)
			if mi.ClientStreaming || mi.ServerStreaming {
				mi.IsStreaming = true
				si.HasStreaming = true
			}
		}
		si.GenerateHandler = true
		ss = append(ss, si)
	}

	if pg.c.CombineService && len(ss) > 0 {
		// TODO(liyun.339): this code is redundant. it exists in thrift & protoc
		var svcs []*generator.ServiceInfo
		var methods []*generator.MethodInfo
		for _, s := range ss {
			svcs = append(svcs, s)
			methods = append(methods, s.AllMethods()...)
		}
		// check method name conflict
		mm := make(map[string]*generator.MethodInfo)
		for _, m := range methods {
			if _, ok := mm[m.Name]; ok {
				log.Warnf("combine service method %s in %s conflicts with %s in %s\n",
					m.Name, m.ServiceName, m.Name, mm[m.Name].ServiceName)
				return
			}
			mm[m.Name] = m
		}
		var hasStreaming bool
		for _, m := range methods {
			if m.ClientStreaming || m.ServerStreaming {
				hasStreaming = true
			}
		}
		svcName := getCombineServiceName("CombineService", ss)
		si := &generator.ServiceInfo{
			PkgInfo:         pi,
			ServiceName:     svcName,
			RawServiceName:  svcName,
			CombineServices: svcs,
			Methods:         methods,
			HasStreaming:    hasStreaming,
		}
		si.ServiceTypeName = func() string { return si.ServiceName }
		ss = append(ss, si)
	}
	return
}

// TODO(liyun.339): this code is redundant. it exists in thrift & protoc
func getCombineServiceName(name string, svcs []*generator.ServiceInfo) string {
	for _, svc := range svcs {
		if svc.ServiceName == name {
			return getCombineServiceName(name+"_", svcs)
		}
	}
	return name
}

func (pg *PrutalGen) convertParameter(f *prutalgen.Proto, t *prutalgen.Type, paramName string) *generator.Parameter {
	importPath := t.GoImport()
	pkgRefName := path.Base(importPath)
	typeName := t.GoName()
	if !strings.Contains(typeName, ".") {
		typeName = pkgRefName + "." + t.GoName()
	}
	res := &generator.Parameter{
		Deps: []generator.PkgInfo{
			{
				PkgRefName: pkgRefName,
				ImportPath: importPath,
			},
		},
		Name:    paramName,
		RawName: paramName,
		Type:    "*" + typeName,
	}
	return res
}

func genStructsAndKitexInterfaces(f *prutalgen.Proto, c *generator.Config, dir string) {
	g := prutalgen.NewGoCodeGen()
	g.Getter = true
	g.Marshaler = prutalgen.MarshalerKitexProtobuf

	header := fmt.Sprintf(`// Code generated by Kitex %s. DO NOT EDIT.`, c.Version)
	w := prutalgen.NewCodeWriter(header, f.GoPackage)
	g.ProtoGen(f, w) // structs

	// interfaces used by kitex
	genKitexServiceInterface(f, w, c.IsUsingStreamX())

	baseFilename := filepath.Base(f.ProtoFile)
	fn := strings.TrimSuffix(baseFilename, ".proto") + ".pb.go"

	out := filepath.Join(dir, fn)
	f.Infof("generating %s", out)
	writeFile(out, w.Bytes())
}

func genKitexServiceInterface(f *prutalgen.Proto, w *prutalgen.CodeWriter, streamx bool) {
	in := &pbtpl.Args{}
	in.StreamX = streamx
	for _, s := range f.Services {
		x := &pbtpl.Service{
			Name: s.GoName,
		}
		for _, m := range s.Methods {
			x.Methods = append(x.Methods, &pbtpl.Method{
				Name:    m.GoName,
				ReqType: "*" + m.Request.GoName(),
				ResType: "*" + m.Return.GoName(),

				ClientStream: m.RequestStream,
				ServerStream: m.ReturnStream,
			})
			if m.Request != nil && m.Request.IsExternalType() {
				w.UsePkg(m.Request.GoImport(), "")
			}
			if m.Return != nil && m.Return.IsExternalType() {
				w.UsePkg(m.Return.GoImport(), "")
			}

			if streamx {
				w.UsePkg("context", "")
				w.UsePkg("github.com/cloudwego/kitex/pkg/streaming", "")
			} else if m.RequestStream || m.ReturnStream {
				w.UsePkg("github.com/cloudwego/kitex/pkg/streaming", "")
			} else {
				w.UsePkg("context", "")
			}
		}
		in.Services = append(in.Services, x)
	}
	pbtpl.Render(w, in)
}

func getImportPath(pkg, module, prefix string) (string, bool) {
	parts := strings.Split(pkg, "/")
	if len(parts) == 0 {
		// malformed import path
		return "", false
	}

	if strings.HasPrefix(pkg, prefix) {
		return pkg, true
	}
	if strings.Contains(parts[0], ".") || (module != "" && strings.HasPrefix(pkg, module)) {
		// already a complete import path, but outside the target path
		return "", false
	}
	// incomplete import path
	return path.Join(prefix, pkg), true
}

func writeFile(fn string, data []byte) {
	if strings.HasSuffix(fn, ".go") {
		formatted, err := format.Source(data)
		if err != nil {
			log.Errorf("format file %q err: %s", fn, err)
		} else {
			data = formatted
		}
	}
	err := os.MkdirAll(filepath.Dir(fn), 0o755)
	if err == nil {
		err = os.WriteFile(fn, data, 0o644)
	}
	if err != nil {
		log.Errorf("write file %q err: %s", fn, err)
	}
}
