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
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cloudwego/thriftgo/generator/backend"
	"github.com/cloudwego/thriftgo/generator/golang/common"
	"github.com/cloudwego/thriftgo/generator/golang/styles"
	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/plugin"
	"github.com/cloudwego/thriftgo/semantic"
)

// Default libraries.
const (
	DefaultThriftLib  = "github.com/apache/thrift/lib/go/thrift"
	DefaultUnknownLib = "github.com/cloudwego/thriftgo/generator/golang/extension/unknown"
)

// CodeUtils contains a set of utility functions.
type CodeUtils struct {
	backend.LogFunc
	packagePrefix string            // Package prefix for all generated codes.
	customized    map[string]string // Customized imports, package => import path.
	features      Features          // Available features.
	namingStyle   styles.Naming     // Naming style.
	doInitialisms bool              // Make initialisms setting kept event naming style changes.

	rootScope  *Scope
	scopeCache map[*parser.Thrift]*Scope
}

// NewCodeUtils creates a new CodeUtils.
func NewCodeUtils(log backend.LogFunc) *CodeUtils {
	cu := &CodeUtils{
		LogFunc:     log,
		customized:  make(map[string]string),
		features:    defaultFeatures,
		namingStyle: styles.NewNamingStyle("thriftgo"),
		scopeCache:  make(map[*parser.Thrift]*Scope),
	}
	return cu
}

// SetFeatures sets the feature set.
func (cu *CodeUtils) SetFeatures(fs Features) {
	cu.features = fs
}

// Features returns the current settings of generator features.
func (cu *CodeUtils) Features() Features {
	return cu.features
}

// GetPackagePrefix sets the package prefix in generated codes.
func (cu *CodeUtils) GetPackagePrefix() (pp string) {
	return cu.packagePrefix
}

// SetPackagePrefix sets the package prefix in generated codes.
func (cu *CodeUtils) SetPackagePrefix(pp string) {
	cu.packagePrefix = pp
}

// UsePackage forces the generated codes to use the specific package.
func (cu *CodeUtils) UsePackage(pkg, path string) {
	cu.customized[pkg] = path
}

// NamingStyle returns the current naming style.
func (cu *CodeUtils) NamingStyle() styles.Naming {
	return cu.namingStyle
}

// SetNamingStyle sets the naming style.
func (cu *CodeUtils) SetNamingStyle(style styles.Naming) {
	cu.namingStyle = style
	cu.namingStyle.UseInitialisms(cu.doInitialisms)
}

// UseInitialisms sets the naming style's initialisms option.
func (cu *CodeUtils) UseInitialisms(enable bool) {
	cu.doInitialisms = enable
	cu.namingStyle.UseInitialisms(cu.doInitialisms)
}

// Identify converts an raw name from IDL into an exported identifier in go.
func (cu *CodeUtils) Identify(name string) (s string, err error) {
	s = strings.TrimPrefix(name, prefix)
	s, err = cu.namingStyle.Identify(s)
	if err != nil {
		return "", err
	}
	return s, nil
}

// Debug prints the given values with println.
func (cu *CodeUtils) Debug(vs ...interface{}) string {
	var ss []string
	for _, v := range vs {
		ss = append(ss, fmt.Sprintf("%T(%+v)", v, v))
	}
	println("[DEBUG]", strings.Join(ss, " "))
	return ""
}

// ParseNamespace retrieves informations from the given AST and returns a
// reference name in the IDL, a package name for generated codes and an import path.
func (cu *CodeUtils) ParseNamespace(ast *parser.Thrift) (ref, pkg, pth string) {
	ref = filepath.Base(ast.Filename)
	ref = strings.TrimSuffix(ref, filepath.Ext(ref))
	ns := ast.GetNamespaceOrReferenceName("go")
	pkg = cu.NamespaceToPackage(ns)
	pth = cu.NamespaceToImportPath(ns)
	return
}

// GetFilePath returns a path to the generated file for the given IDL.
// Note that the result is a path relative to the root output path.
func (cu *CodeUtils) GetFilePath(t *parser.Thrift) string {
	ref, _, pth := cu.ParseNamespace(t)
	full := filepath.Join(pth, ref+".go")
	if strings.HasSuffix(full, "_test.go") {
		full = strings.ReplaceAll(full, "_test.go", "_test_.go")
	}
	return full
}

// GetPackageName returns a go package name for the given thrift AST.
func (cu *CodeUtils) GetPackageName(ast *parser.Thrift) string {
	namespace := ast.GetNamespaceOrReferenceName("go")
	return cu.NamespaceToPackage(namespace)
}

// NamespaceToPackage converts a namespace to a package.
func (cu *CodeUtils) NamespaceToPackage(ns string) string {
	parts := strings.Split(ns, ".")
	return strings.ToLower(parts[len(parts)-1])
}

// NamespaceToImportPath returns an import path for the given namespace.
// Note that the result will not have the package prefix set with SetPackagePrefix.
func (cu *CodeUtils) NamespaceToImportPath(ns string) string {
	pkg := strings.ReplaceAll(ns, ".", "/")
	return pkg
}

// NamespaceToFullImportPath returns an import path for the given namespace.
// The result path will contain the package prefix if it is set.
func (cu *CodeUtils) NamespaceToFullImportPath(ns string) string {
	pth := cu.NamespaceToImportPath(ns)
	if cu.packagePrefix != "" {
		pth = filepath.Join(cu.packagePrefix, pth)
	}
	return pth
}

// Import returns the package name and the full import path for the given AST.
func (cu *CodeUtils) Import(t *parser.Thrift) (pkg, pth string) {
	ns := t.GetNamespaceOrReferenceName("go")
	pkg = cu.NamespaceToPackage(ns)
	pth = cu.NamespaceToFullImportPath(ns)
	return
}

// GenTags generates go tags for the given parser.Field.
func (cu *CodeUtils) GenTags(f *parser.Field, insertPoint string) (string, error) {
	var tags []string
	if f.Requiredness == parser.FieldType_Required {
		tags = append(tags, fmt.Sprintf(`thrift:"%s,%d,required"`, f.Name, f.ID))
	} else {
		tags = append(tags, fmt.Sprintf(`thrift:"%s,%d"`, f.Name, f.ID))
	}

	if gotags := f.Annotations.Get("go.tag"); len(gotags) > 0 {
		tags = append(tags, gotags[0])
	} else {
		if cu.Features().GenDatabaseTag {
			tags = append(tags, fmt.Sprintf(`db:"%s"`, f.Name))
		}

		if f.Requiredness.IsOptional() && cu.Features().GenOmitEmptyTag {
			tags = append(tags, fmt.Sprintf(`json:"%s,omitempty"`, f.Name))
		} else {
			tags = append(tags, fmt.Sprintf(`json:"%s"`, f.Name))
		}
	}
	str := fmt.Sprintf("`%s%s`", strings.Join(tags, " "), insertPoint)
	return str, nil
}

// GetKeyType returns the key type of the given type. T must be a map type.
func (cu *CodeUtils) GetKeyType(s *Scope, t *parser.Type) (*Scope, *parser.Type, error) {
	if t.Category != parser.Category_Map {
		return nil, nil, fmt.Errorf("expect map type, got: '%s'", t)
	}
	g, x, err := semantic.Deref(s.ast, t)
	if err != nil {
		return nil, nil, err
	}
	return cu.scopeCache[g], x.KeyType, nil
}

// GetValType returns the value type of the given type. T must be a container type.
func (cu *CodeUtils) GetValType(s *Scope, t *parser.Type) (*Scope, *parser.Type, error) {
	if !t.Category.IsContainerType() {
		return nil, nil, fmt.Errorf("expect container type, got: '%s'", t)
	}
	g, x, err := semantic.Deref(s.ast, t)
	if err != nil {
		return nil, nil, err
	}
	return cu.scopeCache[g], x.ValueType, nil
}

// MkRWCtx = MakeReadWriteContext.
// Check the documents of ReadWriteContext for more informations.
func (cu *CodeUtils) MkRWCtx(root *Scope, f *Field) (*ReadWriteContext, error) {
	r := NewResolver(root, cu)
	ctx, err := mkRWCtx(r, root, f.Type, nil)
	if err != nil {
		return nil, err
	}

	// adjust for fields
	ctx.Target = "p." + f.GoName().String()
	ctx.Source = "src"
	ctx.TypeName = f.GoTypeName()
	ctx.IsPointer = f.GoTypeName().IsPointer()
	return ctx, nil
}

// SetRootScope sets the root scope for rendering templates.
func (cu *CodeUtils) SetRootScope(s *Scope) {
	cu.rootScope = s
}

// RootScope returns the root scope previously set by SetRootScope.
func (cu *CodeUtils) RootScope() *Scope {
	return cu.rootScope
}

// BuildFuncMap builds a function map for templates.
func (cu *CodeUtils) BuildFuncMap() template.FuncMap {
	m := map[string]interface{}{
		"ToUpper":        strings.ToUpper,
		"ToLower":        strings.ToLower,
		"InsertionPoint": plugin.InsertionPoint,
		"Unexport":       common.Unexport,

		"Debug":          cu.Debug,
		"Features":       cu.Features,
		"GetPackageName": cu.GetPackageName,
		"GenTags":        cu.GenTags,
		"MkRWCtx": func(f *Field) (*ReadWriteContext, error) {
			return cu.MkRWCtx(cu.rootScope, f)
		},

		"IsBaseType":        IsBaseType,
		"NeedRedirect":      NeedRedirect,
		"IsFixedLengthType": IsFixedLengthType,
		"SupportIsSet":      SupportIsSet,
		"GetTypeIDConstant": GetTypeIDConstant,
		"UseStdLibrary": func(lib string) string {
			cu.rootScope.imports.UseStdLibrary(lib)
			return ""
		},
		"ServicePrefix": func(svc *Service) (string, error) {
			if svc == nil || svc.From().namespace == cu.rootScope.namespace {
				return "", nil
			}
			ast := svc.From().AST()
			inc := cu.rootScope.Includes().ByAST(ast)
			if inc == nil {
				return "", fmt.Errorf("unexpected service[%s] from scope[%s]", svc.Name, ast.Filename)
			}
			return inc.PackageName + ".", nil
		},
		"ServiceName": func(svc *Service) Name {
			if svc == nil {
				return Name("")
			}
			return svc.GoName()
		},
	}
	return m
}
