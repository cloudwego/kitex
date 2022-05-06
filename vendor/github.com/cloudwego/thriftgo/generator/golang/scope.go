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
	"strings"

	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/pkg/namespace"
)

// Name is the type of identifiers converted from a thrift AST to Go code.
type Name string

func (n Name) String() string {
	return string(n)
}

// Code is the type of go code segments.
type Code string

func (c Code) String() string {
	return string(c)
}

// TypeName is the type for Go symbols converted from a thrift AST.
// It provides serveral methods to manipulate a type name of a symbol.
type TypeName string

func (tn TypeName) String() string {
	return string(tn)
}

// IsForeign reports whether the type name is from another module.
func (tn TypeName) IsForeign() bool {
	return strings.Contains(string(tn), ".")
}

// Pointerize returns the pointer type of the current type name.
func (tn TypeName) Pointerize() TypeName {
	return TypeName("*" + string(tn))
}

// IsPointer reports whether the type name is a pointer type
// by detecting the prefix "*".
func (tn TypeName) IsPointer() bool {
	return strings.HasPrefix(string(tn), "*")
}

// Deref removes the "&" and "*" prefix of the given type name.
func (tn TypeName) Deref() TypeName {
	return TypeName(strings.TrimLeft(string(tn), "&*"))
}

// NewFunc returns the construction function of the given type.
func (tn TypeName) NewFunc() Name {
	n := string(tn)
	if idx := strings.Index(n, "."); idx >= 0 {
		idx++
		return Name(n[:idx] + "New" + n[idx:])
	}
	return Name("New" + n)
}

// BuildScope creates a scope of the AST with its includes processed recursively.
func BuildScope(cu *CodeUtils, ast *parser.Thrift) (*Scope, error) {
	if scope, ok := cu.scopeCache[ast]; ok {
		return scope, nil
	}
	scope := newScope(ast)
	err := scope.init(cu)
	if err != nil {
		return nil, fmt.Errorf("process '%s' failed: %w", ast.Filename, err)
	}
	cu.scopeCache[ast] = scope
	return scope, nil
}

// Scope contains the type symbols defined in a thrift IDL and wraps them to provide
// access to resolved names in go code.
type Scope struct {
	ast       *parser.Thrift
	globals   namespace.Namespace
	imports   *importManager
	namespace string

	includes    []*Include
	constants   []*Constant
	typedefs    []*Typedef
	enums       []*Enum
	structs     []*StructLike
	unions      []*StructLike
	exceptions  []*StructLike
	services    []*Service
	synthesized []*StructLike
}

// AST returns the thrift AST associated with the scope.
func (s *Scope) AST() *parser.Thrift {
	return s.ast
}

// Namespace returns the global namespace of the current scope.
func (s *Scope) Namespace() namespace.Namespace {
	return s.globals
}

// ResolveImports returns a map of import path to alias built from the include list
// of the IDL. An alias may be an empty string to indicate no alias is need for the
// import path.
func (s *Scope) ResolveImports() (map[string]string, error) {
	return s.imports.ResolveImports()
}

// Includes returns the include list of the current scope.
func (s *Scope) Includes() Includes {
	return Includes(s.includes)
}

// Constant returns a constant defined in the current scope
// with the given name. It returns nil if the constant is not found.
func (s *Scope) Constant(name string) *Constant {
	for _, c := range s.constants {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Constants is a list of constants.
type Constants []*Constant

// GoVariables returns the subset of the current constants that
// each one of them results a variable in Go.
func (cs Constants) GoVariables() (gvs Constants) {
	for _, c := range cs {
		if !IsConstantInGo(c.Constant) {
			gvs = append(gvs, c)
		}
	}
	return
}

// GoConstants returns the subset of the current constants that
// each one of them results a constant in Go.
func (cs Constants) GoConstants() (gcs Constants) {
	for _, c := range cs {
		if IsConstantInGo(c.Constant) {
			gcs = append(gcs, c)
		}
	}
	return
}

// Constants returns all thrift constants defined in the current scope.
func (s *Scope) Constants() Constants {
	return Constants(s.constants)
}

// Typedef returns a typedef defined in the current scope with the
// given name. It returns nil if the typedef is not found.
func (s *Scope) Typedef(alias string) *Typedef {
	for _, t := range s.typedefs {
		if t.Alias == alias {
			return t
		}
	}
	return nil
}

// Typedefs returns all typedefs defined in the current scope.
func (s *Scope) Typedefs() []*Typedef {
	return s.typedefs
}

// Enum returns an enum defined in the current scope with the
// given name. It returns nil if the enum is not found.
func (s *Scope) Enum(name string) *Enum {
	for _, enum := range s.enums {
		if enum.Name == name {
			return enum
		}
	}
	return nil
}

// Enums returns all enums defined in the current scope.
func (s *Scope) Enums() []*Enum {
	return s.enums
}

// Struct returns a struct defined in the current scope with
// the given name. It returns nil if the struct is not found.
func (s *Scope) Struct(name string) *StructLike {
	for _, s := range s.structs {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// Structs returns all struct defined in the current scope.
func (s *Scope) Structs() []*StructLike {
	return s.structs
}

// Union returns a union defined in the current scope with the
// given name. It returns nil if the union is not found.
func (s *Scope) Union(name string) *StructLike {
	for _, u := range s.unions {
		if u.Name == name {
			return u
		}
	}
	return nil
}

// Unions returns all union defined in the current scope.
func (s *Scope) Unions() []*StructLike {
	return s.unions
}

// Exception returns an exception defined in the current scope
// with the given name. It returns nil if the exception is not found.
func (s *Scope) Exception(name string) *StructLike {
	for _, e := range s.exceptions {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// Exceptions returns all exception defined in the current scope.
func (s *Scope) Exceptions() []*StructLike {
	return s.exceptions
}

// StructLike returns a struct-like defined in the current scope with
// the given name. It returns nil if the struct-like is not found.
func (s *Scope) StructLike(name string) *StructLike {
	for _, st := range s.StructLikes() {
		if st.Name == name {
			return st
		}
	}
	return nil
}

// StructLikes returns all struct-like defined in the current scope.
func (s *Scope) StructLikes() (ss []*StructLike) {
	ss = make([]*StructLike, 0, len(s.structs)+len(s.unions)+len(s.exceptions))
	ss = append(ss, s.structs...)
	ss = append(ss, s.unions...)
	ss = append(ss, s.exceptions...)
	return
}

// Service returns a service defined in the current scope with the given
// name. It returns nil if the service is not found.
func (s *Scope) Service(name string) *Service {
	for _, svc := range s.services {
		if svc.Name == name {
			return svc
		}
	}
	return nil
}

// Services returns all service defined in the current scope.
func (s *Scope) Services() []*Service {
	return s.services
}

// Include associates an import in golang with the corresponding scope
// that built from an thrift AST in the include list.
type Include struct {
	PackageName string
	ImportPath  string
	*Scope
}

// Includes is a list of Include objects.
type Includes []*Include

// ByIndex returns an Include at the given index. It returns nil if the
// index is invalid.
func (is Includes) ByIndex(i int) *Include {
	if 0 <= i && i < len(is) {
		return is[i]
	}
	return nil
}

// ByAST returns an Include whose scope matches the given AST. It returns
// nil if such an Include is not found.
func (is Includes) ByAST(ast *parser.Thrift) *Include {
	for _, i := range is {
		if i == nil {
			continue
		}
		if i.Scope.ast == ast {
			return i
		}
	}
	return nil
}

// ByPackage returns an Include whose package name matches the given one.
// It returns nil if such an Include is not found.
func (is Includes) ByPackage(pkg string) *Include {
	for _, i := range is {
		if i == nil {
			continue
		}
		if i.PackageName == pkg {
			return i
		}
	}
	return nil
}

// Constant is a wrapper for the parser.Constant.
type Constant struct {
	*parser.Constant
	name     Name
	typeName TypeName
	init     Code
}

// GoName returns the name in go code of the constant.
func (c *Constant) GoName() Name {
	return c.name
}

// GoTypeName returns the type name in go code of the constant.
func (c *Constant) GoTypeName() TypeName {
	return c.typeName
}

// Initialization returns the initialization code of the constant.
func (c *Constant) Initialization() Code {
	return c.init
}

// Typedef is a wrapper for the parser.Typedef.
type Typedef struct {
	*parser.Typedef
	name     Name
	typeName TypeName
}

// GoName returns the name in go code of the typedef.
func (t *Typedef) GoName() Name {
	return t.name
}

// GoTypeName returns the type name in go code of the typedef.
func (t *Typedef) GoTypeName() TypeName {
	return t.typeName
}

// Enum is a wrapper for the parser.Enum.
type Enum struct {
	*parser.Enum
	scope  namespace.Namespace
	name   Name
	values []*EnumValue
}

// GoName returns the name in go code of the enum.
func (e *Enum) GoName() Name {
	return e.name
}

// Value returns a value defined in the enum with the given name.
// It returns nil if the value is not found.
func (e *Enum) Value(name string) *EnumValue {
	for _, ev := range e.values {
		if ev.Name == name {
			return ev
		}
	}
	return nil
}

// Values returns all values defined in the enum.
func (e *Enum) Values() []*EnumValue {
	return e.values
}

// EnumValue is a wrapper for the parser.EnumValue.
type EnumValue struct {
	*parser.EnumValue
	name    Name
	literal Code
}

// GoName returns the name in go code of the enum value.
func (ev *EnumValue) GoName() Name {
	return ev.name
}

// GoLiteral returns the literal in go code of the enum value.
func (ev *EnumValue) GoLiteral() Code {
	return ev.literal
}

// Field is a wrapper for the parser.Field.
type Field struct {
	*parser.Field
	name            Name
	typeName        TypeName
	defaultTypeName TypeName
	defaultValue    Code
	isResponse      bool
	reader          Name
	writer          Name
	getter          Name
	setter          Name
	isset           Name
	deepEqual       Name
}

// GoName returns the name in go code of the field.
func (f *Field) GoName() Name {
	return f.name
}

// GoTypeName returns the type name in go code of the field.
func (f *Field) GoTypeName() TypeName {
	return f.typeName
}

// DefaultTypeName returns the type name in go code of the default value of
// the field. Note that it might be different with the result of GoTypeName.
func (f *Field) DefaultTypeName() TypeName {
	return f.defaultTypeName
}

// DefaultValue returns the go code of the field for its Initialization.
// Note that the code exists only when the field has default value.
func (f *Field) DefaultValue() Code {
	return f.defaultValue
}

// IsResponseFieldOfResult tells if the field is the response of a result type of a method.
func (f *Field) IsResponseFieldOfResult() bool {
	return f.isResponse
}

// Reader returns the name of the method that reads the field.
func (f *Field) Reader() Name {
	return f.reader
}

// Writer returns the name of the method that writes the field.
func (f *Field) Writer() Name {
	return f.writer
}

// Getter returns the getter method's name for the field.
func (f *Field) Getter() Name {
	return f.getter
}

// Setter returns the setter method's name for the field.
func (f *Field) Setter() Name {
	return f.setter
}

// IsSetter returns the isset method's name for the field.
func (f *Field) IsSetter() Name {
	return f.isset
}

// DeepEqual returns the deep compare method's name for the field.
func (f *Field) DeepEqual() Name {
	return f.deepEqual
}

// StructLike is a wrapper for the parser.StructLike.
type StructLike struct {
	*parser.StructLike
	scope  namespace.Namespace
	name   Name
	fields []*Field
}

// GoName returns the name in go code of the struct-like.
func (s *StructLike) GoName() Name {
	return s.name
}

// Field returns a field of the struct-like that has the given name.
// It returns nil if such a field is not found.
func (s *StructLike) Field(name string) *Field {
	for _, f := range s.fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// Fields returns all fields defined in the struct-like.
func (s *StructLike) Fields() []*Field {
	return s.fields
}

// Namespace returns the namescope of the struct-like.
func (s *StructLike) Namespace() namespace.Namespace {
	return s.scope
}

// Service is a wrapper for the parser.Service.
type Service struct {
	*parser.Service
	scope     namespace.Namespace
	from      *Scope
	base      *Service
	name      Name
	functions []*Function
}

// Namespace returns the namespace of the service.
func (s *Service) Namespace() namespace.Namespace {
	return s.scope
}

// From returns the scope that the service is defined in.
func (s *Service) From() *Scope {
	return s.from
}

// Base returns the base service that the current service extends.
// It returns nil if the service has no base service.
func (s *Service) Base() *Service {
	return s.base
}

// GoName returns the name in go code of the service.
func (s *Service) GoName() Name {
	return s.name
}

// Function returns a function defined in the service with the given name.
// It returns nil if the function is not found.
func (s *Service) Function(name string) *Function {
	for _, f := range s.functions {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// Functions returns all functions defined in the service.
func (s *Service) Functions() []*Function {
	return s.functions
}

// Function is a wrapper for the parser.Function.
type Function struct {
	*parser.Function
	scope        namespace.Namespace
	name         Name
	responseType TypeName
	arguments    []*Field
	throws       []*Field
	argType      *StructLike
	resType      *StructLike
}

// GoName returns the go name of the function.
func (f *Function) GoName() Name {
	return f.name
}

// ResponseGoTypeName returns the go type of the response type of the function.
func (f *Function) ResponseGoTypeName() TypeName {
	return f.responseType
}

// Arguments returns the argument list of the function.
func (f *Function) Arguments() []*Field {
	return f.arguments
}

// Throws returns the throw list of the function.
func (f *Function) Throws() []*Field {
	return f.throws
}

// ArgType returns the synthesized structure that wraps of the function.
func (f *Function) ArgType() *StructLike {
	return f.argType
}

// ResType returns the synthesized structure of arguments of the function.
func (f *Function) ResType() *StructLike {
	return f.resType
}

func buildSynthesized(v *parser.Function) (argType, resType *parser.StructLike) {
	argType = &parser.StructLike{
		Category: "struct",
		Name:     v.Name + "_args",
		Fields:   v.Arguments,
	}

	if !v.Oneway {
		resType = &parser.StructLike{
			Category: "struct",
			Name:     v.Name + "_result",
		}
		if !v.Void {
			resType.Fields = append(resType.Fields, &parser.Field{
				ID:           0,
				Name:         "success",
				Requiredness: parser.FieldType_Optional,
				Type:         v.FunctionType,
			})
		}
		resType.Fields = append(resType.Fields, v.Throws...)
	}
	return
}
