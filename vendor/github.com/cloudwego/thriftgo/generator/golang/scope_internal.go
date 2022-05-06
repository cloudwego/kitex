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
	"strconv"
	"strings"

	"github.com/cloudwego/thriftgo/generator/golang/common"
	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/pkg/namespace"
)

// A prefix to denote synthesized identifiers.
const prefix = "$"

func _p(id string) string {
	return prefix + id
}

// newScope creates an uninitialized scope from the given IDL.
func newScope(ast *parser.Thrift) *Scope {
	return &Scope{
		ast:       ast,
		imports:   newImportManager(),
		globals:   namespace.NewNamespace(namespace.UnderscoreSuffix),
		namespace: ast.GetNamespaceOrReferenceName("go"),
	}
}

func (s *Scope) init(cu *CodeUtils) (err error) {
	defer func() {
		if x, ok := recover().(error); ok && x != nil {
			err = x
		}
	}()
	if cu.Features().ReorderFields {
		for _, x := range s.ast.GetStructLikes() {
			diff := reorderFields(x)
			if diff != nil && diff.original != diff.arranged {
				cu.Info(fmt.Sprintf("<reorder>(%s) %s: %d -> %d: %.2f%%",
					s.ast.Filename, x.Name, diff.original, diff.arranged, diff.percent()))
			}
		}
	}
	s.imports.init(cu, s.ast)
	s.buildIncludes(cu)
	s.installNames(cu)
	s.resolveTypesAndValues(cu)
	return nil
}

func (s *Scope) buildIncludes(cu *CodeUtils) {
	// the indices of includes must be kept because parser.Reference.Index counts the unused IDLs.
	cnt := len(s.ast.Includes)
	s.includes = make([]*Include, cnt)

	for idx, inc := range s.ast.Includes {
		if !inc.GetUsed() {
			continue
		}
		s.includes[idx] = s.include(cu, inc.Reference)
	}
}

func (s *Scope) include(cu *CodeUtils, t *parser.Thrift) *Include {
	scope, err := BuildScope(cu, t)
	if err != nil {
		panic(err)
	}
	pkg, pth := cu.Import(t)
	if s.namespace != scope.namespace {
		pkg = s.imports.Add(pkg, pth)
	}

	return &Include{
		PackageName: pkg,
		ImportPath:  pth,
		Scope:       scope,
	}
}

// includeIDL adds an probably new IDL to the include list.
func (s *Scope) includeIDL(cu *CodeUtils, t *parser.Thrift) (pkgName string) {
	_, pth := cu.Import(t)
	if pkgName = s.imports.Get(pth); pkgName != "" {
		return
	}
	inc := s.include(cu, t)
	s.includes = append(s.includes, inc)
	return inc.PackageName
}

func (s *Scope) installNames(cu *CodeUtils) {
	for _, v := range s.ast.Services {
		s.buildService(cu, v)
	}
	for _, v := range s.ast.GetStructLikes() {
		s.buildStructLike(cu, v)
	}
	for _, v := range s.ast.Enums {
		s.buildEnum(cu, v)
	}
	for _, v := range s.ast.Typedefs {
		s.buildTypedef(cu, v)
	}
	for _, v := range s.ast.Constants {
		s.buildConstant(cu, v)
	}
}

func (s *Scope) identify(cu *CodeUtils, raw string) string {
	name, err := cu.Identify(raw)
	if err != nil {
		panic(err)
	}
	if !strings.HasPrefix(raw, prefix) && cu.Features().CompatibleNames {
		if strings.HasPrefix(name, "New") || strings.HasSuffix(name, "Args") || strings.HasSuffix(name, "Result") {
			name += "_"
		}
	}
	return name
}

func (s *Scope) buildService(cu *CodeUtils, v *parser.Service) {
	// service name
	sn := s.identify(cu, v.Name)
	sn = s.globals.Add(sn, v.Name)

	svc := &Service{
		Service: v,
		scope:   namespace.NewNamespace(namespace.UnderscoreSuffix),
		from:    s,
		name:    Name(sn),
	}
	s.services = append(s.services, svc)

	// function names
	for _, f := range v.Functions {
		fn := s.identify(cu, f.Name)
		fn = svc.scope.Add(fn, f.Name)
		svc.functions = append(svc.functions, &Function{
			Function: f,
			scope:    namespace.NewNamespace(namespace.UnderscoreSuffix),
			name:     Name(fn),
		})
	}

	// install names for argument types and response types
	for idx, f := range v.Functions {
		argType, resType := buildSynthesized(f)
		an := v.Name + s.identify(cu, _p(f.Name+"_args"))
		rn := v.Name + s.identify(cu, _p(f.Name+"_result"))

		fun := svc.functions[idx]
		fun.argType = s.buildStructLike(cu, argType, _p(an))
		if !f.Oneway {
			fun.resType = s.buildStructLike(cu, resType, _p(rn))
			if !f.Void {
				fun.resType.fields[0].isResponse = true
			}
		}

		s.buildFunction(cu, fun, f)
	}

	// install names for client and processor
	cn := sn + "Client"
	pn := sn + "Processor"
	s.globals.MustReserve(cn, _p("client:"+v.Name))
	s.globals.MustReserve(pn, _p("processor:"+v.Name))
}

// buildFunction builds a namespace for parameters of a Function.
// This function is used to resolve conflicts between parameter, receiver and local variables in generated method.
// Template 'Service' and 'FunctionSignature' depend on this function.
func (s *Scope) buildFunction(cu *CodeUtils, f *Function, v *parser.Function) {
	ns := f.scope

	ns.MustReserve("p", _p("p"))     // the receiver of method
	ns.MustReserve("err", _p("err")) // error
	ns.MustReserve("ctx", _p("ctx")) // first parameter

	if !v.Void {
		ns.MustReserve("r", _p("r"))             // response
		ns.MustReserve("_result", _p("_result")) // a local variable
	}

	for _, a := range v.Arguments {
		name := common.LowerFirstRune(s.identify(cu, a.Name))
		if isKeywords[name] {
			name = "_" + name
		}
		ns.Add(name, a.Name)
	}

	for _, t := range v.Throws {
		name := common.LowerFirstRune(s.identify(cu, t.Name))
		if isKeywords[name] {
			name = "_" + name
		}
		ns.Add(name, t.Name)
	}
}

func (s *Scope) buildTypedef(cu *CodeUtils, t *parser.Typedef) {
	tn := s.identify(cu, t.Alias)
	tn = s.globals.Add(tn, t.Alias)
	if t.Type.Category.IsStructLike() {
		fn := "New" + tn
		s.globals.MustReserve(fn, _p("new:"+t.Alias))
	}
	s.typedefs = append(s.typedefs, &Typedef{
		Typedef: t,
		name:    Name(tn),
	})
}

func (s *Scope) buildEnum(cu *CodeUtils, e *parser.Enum) {
	en := s.identify(cu, e.Name)
	en = s.globals.Add(en, e.Name)

	enum := &Enum{
		Enum:  e,
		scope: namespace.NewNamespace(namespace.UnderscoreSuffix),
		name:  Name(en),
	}
	for _, v := range e.Values {
		vn := enum.scope.Add(en+"_"+v.Name, v.Name)
		ev := &EnumValue{
			EnumValue: v,
			name:      Name(vn),
		}
		if cu.Features().TypedEnumString {
			ev.literal = Code(ev.name)
		} else {
			ev.literal = Code(v.Name)
		}
		enum.values = append(enum.values, ev)
	}
	s.enums = append(s.enums, enum)
}

func (s *Scope) buildConstant(cu *CodeUtils, v *parser.Constant) {
	cn := s.identify(cu, v.Name)
	cn = s.globals.Add(cn, v.Name)
	s.constants = append(s.constants, &Constant{
		Constant: v,
		name:     Name(cn),
	})
}

func (s *Scope) buildStructLike(cu *CodeUtils, v *parser.StructLike, usedName ...string) *StructLike {
	nn := v.Name
	if len(usedName) != 0 {
		nn = usedName[0]
	}
	sn := s.identify(cu, nn)
	sn = s.globals.Add(sn, v.Name)
	s.globals.MustReserve("New"+sn, _p("new:"+nn))

	fids := "fieldIDToName_" + sn
	s.globals.MustReserve(fids, _p("ids:"+nn))

	// built-in methods
	funcs := []string{"Read", "Write", "String"}
	if !strings.HasPrefix(v.Name, prefix) {
		if v.Category == "union" {
			funcs = append(funcs, "CountSetFields")
		}
		if v.Category == "exception" {
			funcs = append(funcs, "Error")
		}
		if cu.Features().KeepUnknownFields {
			funcs = append(funcs, "CarryingUnknownFields")
		}
		if cu.Features().GenDeepEqual {
			funcs = append(funcs, "DeepEqual")
		}
	}

	st := &StructLike{
		StructLike: v,
		scope:      namespace.NewNamespace(namespace.UnderscoreSuffix),
		name:       Name(sn),
	}

	for _, fn := range funcs {
		st.scope.MustReserve(fn, _p(fn))
	}

	id2str := func(id int32) string {
		i := int(id)
		if i < 0 {
			return "_" + strconv.Itoa(-i)
		}
		return strconv.Itoa(i)
	}
	// reserve method names
	for _, f := range v.Fields {
		fn := s.identify(cu, f.Name)
		st.scope.Add("Get"+fn, _p("get:"+f.Name))
		if cu.Features().GenerateSetter {
			st.scope.Add("Set"+fn, _p("set:"+f.Name))
		}
		if SupportIsSet(f) {
			st.scope.Add("IsSet"+fn, _p("isset:"+f.Name))
		}
		id := id2str(f.ID)
		st.scope.Add("ReadField"+id, _p("read:"+id))
		st.scope.Add("writeField"+id, _p("write:"+id))
		if cu.Features().GenDeepEqual {
			st.scope.Add("Field"+id+"DeepEqual", _p("deepequal:"+id))
		}
	}

	// field names
	for _, f := range v.Fields {
		fn := s.identify(cu, f.Name)
		fn = st.scope.Add(fn, f.Name)
		id := id2str(f.ID)
		st.fields = append(st.fields, &Field{
			Field:     f,
			name:      Name(fn),
			reader:    Name(st.scope.Get(_p("read:" + id))),
			writer:    Name(st.scope.Get(_p("write:" + id))),
			getter:    Name(st.scope.Get(_p("get:" + f.Name))),
			setter:    Name(st.scope.Get(_p("set:" + f.Name))),
			isset:     Name(st.scope.Get(_p("isset:" + f.Name))),
			deepEqual: Name(st.scope.Get(_p("deepequal:" + id))),
		})
	}

	if len(usedName) > 0 {
		s.synthesized = append(s.synthesized, st)
	} else {
		ss := map[string]*[]*StructLike{
			"struct":    &s.structs,
			"union":     &s.unions,
			"exception": &s.exceptions,
		}[v.Category]
		if ss == nil {
			cu.Warn(fmt.Sprintf("struct[%s].category[%s]", st.Name, st.Category))
		} else {
			*ss = append(*ss, st)
		}
	}
	return st
}

func (s *Scope) resolveTypesAndValues(cu *CodeUtils) {
	resolver := NewResolver(s, cu)

	ff := make(chan *Field)

	go func() {
		ss := append(s.StructLikes(), s.synthesized...)
		for _, st := range ss {
			for _, f := range st.fields {
				ff <- f
			}
		}
		close(ff)
	}()

	ensureType := func(t TypeName, e error) TypeName {
		if e != nil {
			panic(e)
		}
		return t
	}
	ensureCode := func(c Code, e error) Code {
		if e != nil {
			panic(e)
		}
		return c
	}

	for f := range ff {
		v := f.Field
		f.typeName = ensureType(resolver.ResolveFieldTypeName(v))
		f.defaultTypeName = ensureType(resolver.GetDefaultValueTypeName(v))
		if f.IsSetDefault() {
			f.defaultValue = ensureCode(resolver.GetFieldInit(v))
		}
	}
	for _, t := range s.typedefs {
		t.typeName = ensureType(resolver.ResolveTypeName(t.Type)).Deref()
	}
	for _, v := range s.constants {
		v.typeName = ensureType(resolver.ResolveTypeName(v.Type))
		v.init = ensureCode(resolver.GetConstInit(v.Name, v.Type, v.Value))
	}

	for _, svc := range s.services {
		if svc.Extends == "" {
			continue
		}
		ref := svc.GetReference()
		if ref == nil {
			svc.base = s.Service(svc.Extends)
		} else {
			idx := ref.GetIndex()
			svc.base = s.includes[idx].Scope.Service(ref.GetName())
		}
	}

	for _, svc := range s.services {
		for _, fun := range svc.functions {
			for _, f := range fun.argType.fields {
				a := *f
				a.name = Name(fun.scope.Get(f.Name))
				fun.arguments = append(fun.arguments, &a)
			}
			if !fun.Oneway {
				fs := fun.resType.fields
				if !fun.Void {
					fun.responseType = ensureType(resolver.ResolveTypeName(fs[0].Type))
					fs = fs[1:]
				}
				for _, f := range fs {
					t := *f
					t.name = Name(fun.scope.Get(f.Name))
					fun.throws = append(fun.throws, &t)
				}
			}
		}
	}
}
