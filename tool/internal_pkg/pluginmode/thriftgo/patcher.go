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

package thriftgo

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/cloudwego/kitex/tool/internal_pkg/util"

	"github.com/cloudwego/thriftgo/generator/golang"
	"github.com/cloudwego/thriftgo/generator/golang/templates"
	"github.com/cloudwego/thriftgo/generator/golang/templates/slim"
	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/plugin"

	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
)

var extraTemplates []string

// AppendToTemplate string
func AppendToTemplate(text string) {
	extraTemplates = append(extraTemplates, text)
}

const kitexUnusedProtection = `
// KitexUnusedProtection is used to prevent 'imported and not used' error.
var KitexUnusedProtection = struct{}{}
`

const kitexUnusedProtectionWithStringDeepCopy = `
import "unsafe"

// KitexUnusedProtection is used to prevent 'imported and not used' error.
var KitexUnusedProtection = struct{}{}

func StringDeepCopy(s string) string {
	buf := []byte(s)
	ns := (*string)(unsafe.Pointer(&buf))
	return *ns
}
`

//lint:ignore U1000 until protectionInsertionPoint is used
var protectionInsertionPoint = "KitexUnusedProtection"

type patcher struct {
	noFastAPI      bool
	utils          *golang.CodeUtils
	module         string
	copyIDL        bool
	version        string
	record         bool
	recordCmd      []string
	deepCopyAPI    bool
	genStrDeepCopy bool
	protocol       string

	fileTpl *template.Template
}

func (p *patcher) buildTemplates() (err error) {
	m := p.utils.BuildFuncMap()
	m["ReorderStructFields"] = p.reorderStructFields
	m["TypeIDToGoType"] = func(t string) string { return typeIDToGoType[t] }
	m["IsBinaryOrStringType"] = p.isBinaryOrStringType
	m["Version"] = func() string { return p.version }
	m["GenerateFastAPIs"] = func() bool { return !p.noFastAPI && p.utils.Template() != "slim" }
	m["GenerateDeepCopyAPIs"] = func() bool { return p.deepCopyAPI }
	m["GenStrDeepCopy"] = func() bool { return p.genStrDeepCopy }
	m["GenerateArgsResultTypes"] = func() bool { return p.utils.Template() == "slim" }
	m["ImportPathTo"] = generator.ImportPathTo
	m["ToPackageNames"] = func(imports map[string]string) (res []string) {
		for pth, alias := range imports {
			if alias != "" {
				res = append(res, alias)
			} else {
				res = append(res, strings.ToLower(filepath.Base(pth)))
			}
		}
		sort.Strings(res)
		return
	}
	m["Str"] = func(id int32) string {
		if id < 0 {
			return "_" + strconv.Itoa(-int(id))
		}
		return strconv.Itoa(int(id))
	}
	m["IsNil"] = func(i interface{}) bool {
		return i == nil || reflect.ValueOf(i).IsNil()
	}
	m["SourceTarget"] = func(s string) string {
		// p.XXX
		if strings.HasPrefix(s, "p.") {
			return "src." + s[2:]
		}
		// _key, _val
		return s[1:]
	}
	m["FieldName"] = func(s string) string {
		// p.XXX
		return strings.ToLower(s[2:3]) + s[3:]
	}
	m["IsHessian"] = func() bool {
		return p.IsHessian2()
	}
	m["IsGoStringType"] = func(typeName golang.TypeName) bool {
		return typeName == "string" || typeName == "*string"
	}

	tpl := template.New("kitex").Funcs(m)
	allTemplates := basicTemplates
	if p.utils.Template() == "slim" {
		allTemplates = append(allTemplates, slim.StructLike,
			templates.StructLikeDefault,
			templates.FieldGetOrSet,
			templates.FieldIsSet,
			structLikeDeepCopy,
			fieldDeepCopy,
			fieldDeepCopyStructLike,
			fieldDeepCopyContainer,
			fieldDeepCopyMap,
			fieldDeepCopyList,
			fieldDeepCopySet,
			fieldDeepCopyBaseType,
			structLikeCodec,
			structLikeProtocol,
			javaClassName,
			processor)
	} else {
		allTemplates = append(allTemplates, structLikeCodec,
			structLikeFastRead,
			structLikeFastReadField,
			structLikeDeepCopy,
			structLikeFastWrite,
			structLikeFastWriteNocopy,
			structLikeLength,
			structLikeFastWriteField,
			structLikeFieldLength,
			structLikeProtocol,
			javaClassName,
			fieldFastRead,
			fieldFastReadStructLike,
			fieldFastReadBaseType,
			fieldFastReadContainer,
			fieldFastReadMap,
			fieldFastReadSet,
			fieldFastReadList,
			fieldDeepCopy,
			fieldDeepCopyStructLike,
			fieldDeepCopyContainer,
			fieldDeepCopyMap,
			fieldDeepCopyList,
			fieldDeepCopySet,
			fieldDeepCopyBaseType,
			fieldFastWrite,
			fieldLength,
			fieldFastWriteStructLike,
			fieldStructLikeLength,
			fieldFastWriteBaseType,
			fieldBaseTypeLength,
			fieldFixedLengthTypeLength,
			fieldFastWriteContainer,
			fieldContainerLength,
			fieldFastWriteMap,
			fieldMapLength,
			fieldFastWriteSet,
			fieldSetLength,
			fieldFastWriteList,
			fieldListLength,
			templates.FieldDeepEqual,
			templates.FieldDeepEqualBase,
			templates.FieldDeepEqualStructLike,
			templates.FieldDeepEqualContainer,
			validateSet,
			processor,
		)
	}
	for _, txt := range allTemplates {
		tpl = template.Must(tpl.Parse(txt))
	}

	ext := `{{define "ExtraTemplates"}}{{end}}`
	if len(extraTemplates) > 0 {
		ext = fmt.Sprintf("{{define \"ExtraTemplates\"}}\n%s\n{{end}}",
			strings.Join(extraTemplates, "\n"))
	}
	tpl, err = tpl.Parse(ext)
	if err != nil {
		return fmt.Errorf("failed to parse extra templates: %w: %q", err, ext)
	}

	if p.IsHessian2() {
		tpl, err = tpl.Parse(registerHessian)
		if err != nil {
			return fmt.Errorf("failed to parse hessian2 templates: %w: %q", err, registerHessian)
		}
	}

	p.fileTpl = tpl
	return nil
}

func (p *patcher) patch(req *plugin.Request) (patches []*plugin.Generated, err error) {
	p.buildTemplates()
	var buf strings.Builder

	protection := make(map[string]*plugin.Generated)

	for ast := range req.AST.DepthFirstSearch() {
		// scope, err := golang.BuildScope(p.utils, ast)
		scope, _, err := golang.BuildRefScope(p.utils, ast)
		if err != nil {
			return nil, fmt.Errorf("build scope for ast %q: %w", ast.Filename, err)
		}
		p.utils.SetRootScope(scope)

		pkgName := golang.GetImportPackage(golang.GetImportPath(p.utils, ast))

		path := p.utils.CombineOutputPath(req.OutputPath, ast)
		base := p.utils.GetFilename(ast)
		target := util.JoinPath(path, "k-"+base)

		// Define KitexUnusedProtection in k-consts.go .
		// Add k-consts.go before target to force the k-consts.go generated by consts.thrift to be renamed.
		consts := util.JoinPath(path, "k-consts.go")
		if protection[consts] == nil {
			patch := &plugin.Generated{
				Name: &consts,
			}
			if p.genStrDeepCopy {
				patch.Content = "package " + pkgName + "\n" + kitexUnusedProtectionWithStringDeepCopy
			} else {
				patch.Content = "package " + pkgName + "\n" + kitexUnusedProtection
			}
			patches = append(patches, patch)
			protection[consts] = patch
		}

		buf.Reset()

		// if all scopes are ref, don't generate k-xxx
		if scope == nil {
			continue
		}

		if p.IsHessian2() {
			register := util.JoinPath(path, fmt.Sprintf("hessian2-register-%s", base))
			patch, err := p.patchHessian(path, scope, pkgName, base)
			if err != nil {
				return nil, fmt.Errorf("patch hessian fail for %q: %w", ast.Filename, err)
			}

			patches = append(patches, patch)
			protection[register] = patch
		}

		data := &struct {
			Scope   *golang.Scope
			PkgName string
			Imports map[string]string
		}{Scope: scope, PkgName: pkgName}
		data.Imports, err = scope.ResolveImports()
		if err != nil {
			return nil, fmt.Errorf("resolve imports failed for %q: %w", ast.Filename, err)
		}
		p.filterStdLib(data.Imports)
		if err = p.fileTpl.ExecuteTemplate(&buf, "file", data); err != nil {
			return nil, fmt.Errorf("%q: %w", ast.Filename, err)
		}
		content := buf.String()
		// if kutils is not used, remove the dependency.
		if !strings.Contains(content, "kutils.StringDeepCopy") {
			kutilsImp := `kutils "github.com/cloudwego/kitex/pkg/utils"`
			idx := strings.Index(content, kutilsImp)
			if idx > 0 {
				content = content[:idx-1] + content[idx+len(kutilsImp):]
			}
		}
		patches = append(patches, &plugin.Generated{
			Content: content,
			Name:    &target,
		})

		if p.copyIDL {
			content, err := ioutil.ReadFile(ast.Filename)
			if err != nil {
				return nil, fmt.Errorf("read %q: %w", ast.Filename, err)
			}
			path := util.JoinPath(path, filepath.Base(ast.Filename))
			patches = append(patches, &plugin.Generated{
				Content: string(content),
				Name:    &path,
			})
		}

		if p.record {
			content := doRecord(p.recordCmd)
			bashPath := util.JoinPath(getBashPath())
			patches = append(patches, &plugin.Generated{
				Content: content,
				Name:    &bashPath,
			})
		}

	}
	return
}

func (p *patcher) patchHessian(path string, scope *golang.Scope, pkgName, base string) (patch *plugin.Generated, err error) {
	buf := strings.Builder{}
	resigterIDLName := fmt.Sprintf("hessian2-register-%s", base)
	register := util.JoinPath(path, resigterIDLName)
	data := &struct {
		Scope   *golang.Scope
		PkgName string
		Imports map[string]string
		GoName  string
		IDLName string
	}{Scope: scope, PkgName: pkgName, IDLName: util.UpperFirst(strings.Replace(base, ".go", "", -1))}
	data.Imports, err = scope.ResolveImports()
	if err != nil {
		return nil, err
	}

	if err = p.fileTpl.ExecuteTemplate(&buf, "register", data); err != nil {
		return nil, err
	}
	patch = &plugin.Generated{
		Content: buf.String(),
		Name:    &register,
	}
	return patch, nil
}

func getBashPath() string {
	if runtime.GOOS == "windows" {
		return "kitex-all.bat"
	}
	return "kitex-all.sh"
}

// DoRecord records current cmd into kitex-all.sh
func doRecord(recordCmd []string) string {
	bytes, err := ioutil.ReadFile(getBashPath())
	content := string(bytes)
	if err != nil {
		content = "#! /usr/bin/env bash\n"
	}
	var input, currentIdl string
	for _, s := range recordCmd {
		if s != "-record" {
			input += s + " "
		}
		if strings.HasSuffix(s, ".thrift") || strings.HasSuffix(s, ".proto") {
			currentIdl = s
		}
	}
	if input != "" && currentIdl != "" {
		find := false
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.Contains(input, "-service") && strings.Contains(line, "-service") {
				lines[i] = input
				find = true
				break
			}
			if strings.Contains(line, currentIdl) && !strings.Contains(line, "-service") {
				lines[i] = input
				find = true
				break
			}
		}
		if !find {
			content += "\n" + input
		} else {
			content = strings.Join(lines, "\n")
		}
	}
	return content
}

func (p *patcher) reorderStructFields(fields []*golang.Field) ([]*golang.Field, error) {
	fixedLengthFields := make(map[*golang.Field]bool, len(fields))
	for _, field := range fields {
		fixedLengthFields[field] = golang.IsFixedLengthType(field.Type)
	}

	sortedFields := make([]*golang.Field, 0, len(fields))
	for _, v := range fields {
		if fixedLengthFields[v] {
			sortedFields = append(sortedFields, v)
		}
	}
	for _, v := range fields {
		if !fixedLengthFields[v] {
			sortedFields = append(sortedFields, v)
		}
	}

	return sortedFields, nil
}

func (p *patcher) filterStdLib(imports map[string]string) {
	// remove std libs and thrift to prevent duplicate import.
	prefix := p.module + "/"
	for pth := range imports {
		if strings.HasPrefix(pth, prefix) { // local module
			continue
		}
		if pth == "github.com/apache/thrift/lib/go/thrift" {
			delete(imports, pth)
		}
		if strings.HasPrefix(pth, "github.com/cloudwego/thriftgo") {
			delete(imports, pth)
		}
		if !strings.Contains(pth, ".") { // std lib
			delete(imports, pth)
		}
	}
}

func (p *patcher) isBinaryOrStringType(t *parser.Type) bool {
	return t.Category.IsBinary() || t.Category.IsString()
}

func (p *patcher) IsHessian2() bool {
	return strings.EqualFold(p.protocol, "hessian2")
}

var typeIDToGoType = map[string]string{
	"Bool":   "bool",
	"Byte":   "int8",
	"I16":    "int16",
	"I32":    "int32",
	"I64":    "int64",
	"Double": "float64",
	"String": "string",
	"Binary": "[]byte",
}
