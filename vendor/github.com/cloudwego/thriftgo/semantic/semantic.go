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

package semantic

import (
	"fmt"
	"strings"

	"github.com/cloudwego/thriftgo/parser"
)

// ResolveSymbols checks the existence of each symbol in the current AST
// and build a name-to-category mapping for all locally-defined symbols.
// If a type, a value or a service refers to another one from an external IDL, the
// index of that IDL in the include list will be recorded.
// ResolveSymbols stops when it encounters any error.
func ResolveSymbols(ast *parser.Thrift) error {
	if ast.Name2Category != nil {
		return nil
	}
	ast.Name2Category = make(map[string]parser.Category)
	r := &resolver{ast: ast}
	return r.ResolveAST()
}

// typedefPair contains a type and the AST it belongs to. This struct is used
// to record types that resolved to a typedef instead of a concrete type so
// a post process is required to do further resolution.
type typedefPair struct {
	Type *parser.Type
	AST  *parser.Thrift
	Name string
}

type resolver struct {
	ast      *parser.Thrift
	typedefs []*typedefPair
}

// guard is used to simplify error-checking.
func guard(err error) (ok bool) {
	if err != nil {
		panic(err)
	}
	return true
}

func (r *resolver) AddName(name string, category parser.Category) error {
	if _, exist := r.ast.Name2Category[name]; exist {
		return fmt.Errorf("%s: multiple definition of %q", r.ast.Filename, name)
	}
	r.ast.Name2Category[name] = category
	return nil
}

// RegisterNames adds all locally defined names into Name2Category.
// It panics when encounters any error.
func (r *resolver) RegisterNames() {
	r.ast.ForEachTypedef(func(v *parser.Typedef) bool {
		return guard(r.AddName(v.Alias, parser.Category_Typedef))
	})

	r.ast.ForEachConstant(func(v *parser.Constant) bool {
		return guard(r.AddName(v.Name, parser.Category_Constant))
	})

	r.ast.ForEachEnum(func(v *parser.Enum) bool {
		return guard(r.AddName(v.Name, parser.Category_Enum))
	})

	r.ast.ForEachStructLike(func(v *parser.StructLike) bool {
		switch v.Category {
		case "struct":
			return guard(r.AddName(v.Name, parser.Category_Struct))
		case "union":
			return guard(r.AddName(v.Name, parser.Category_Union))
		case "exception":
			return guard(r.AddName(v.Name, parser.Category_Exception))
		}
		return false
	})

	r.ast.ForEachService(func(v *parser.Service) bool {
		return guard(r.AddName(v.Name, parser.Category_Service))
	})
}

// ResolveAST iterates the current AST and checks the legitimacy of each symbol.
func (r *resolver) ResolveAST() (err error) {
	defer func() {
		if x := recover(); x != nil {
			if e, ok := x.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%+v", x)
			}
		}
	}()

	r.ast.ForEachInclude(func(v *parser.Include) bool {
		if v.Reference == nil {
			panic(fmt.Errorf("reference %q of %q is not parsed", v.Path, r.ast.Filename))
		}
		if err = ResolveSymbols(v.Reference); err != nil {
			panic(fmt.Errorf("resolve include %q: %w", v.Path, err))
		}
		return true
	})

	// register all names defined in the current IDL to make type resolution
	// irrelevant with the order of definitions.
	r.RegisterNames()

	r.ast.ForEachTypedef(func(v *parser.Typedef) bool {
		return guard(r.ResolveType(v.Type))
	})

	r.ast.ForEachConstant(func(v *parser.Constant) bool {
		return guard(r.ResolveType(v.Type)) && guard(r.ResolveConstValue(v.Value))
	})

	r.ast.ForEachStructLike(func(v *parser.StructLike) bool {
		if v.Category == "union" {
			v.ForEachField(func(f *parser.Field) bool {
				f.Requiredness = parser.FieldType_Optional
				return true
			})
		}
		v.ForEachField(func(f *parser.Field) bool {
			return guard(r.ResolveStructField(v.Name, f))
		})
		return true
	})

	r.ast.ForEachService(func(v *parser.Service) bool {
		v.ForEachFunction(func(f *parser.Function) bool {
			guard(r.ResolveFunction(v.Name, f))
			return true
		})

		return guard(r.ResolveBaseService(v))
	})

	guard(r.ResolveTypedefs())
	return
}

func (r *resolver) ResolveBaseService(v *parser.Service) error {
	switch tmp := SplitType(v.Extends); len(tmp) {
	case 1:
		if c, exist := r.ast.Name2Category[tmp[0]]; exist && c == parser.Category_Service {
			break
		}
		return fmt.Errorf("base service %q not found for %q", v.Extends, v.Name)
	case 2:
		for idx, inc := range r.ast.Includes {
			if IDLPrefix(inc.Path) == tmp[0] {
				if c, exist := inc.Reference.Name2Category[tmp[1]]; exist && c == parser.Category_Service {
					v.Reference = &parser.Reference{
						Name:  tmp[1],
						Index: int32(idx),
					}
					r.ast.Includes[idx].Used = &yes
					break
				}
			}
		}
		if v.Reference == nil {
			return fmt.Errorf("base service %q not found for %q", v.Extends, v.Name)
		}
	}
	return nil
}

func (r *resolver) ResolveStructField(s string, f *parser.Field) (err error) {
	if err = r.ResolveType(f.Type); err != nil {
		return fmt.Errorf("resolve field %q of %q: %w", f.Name, s, err)
	}
	if f.IsSetDefault() {
		if err = r.ResolveConstValue(f.Default); err != nil {
			return fmt.Errorf("resolve default value of %q of %q: %w", f.Name, s, err)
		}
	}
	return
}

func (r *resolver) ResolveFunction(s string, f *parser.Function) (err error) {
	if !f.Void {
		if err = r.ResolveType(f.FunctionType); err != nil {
			return fmt.Errorf("function %q of service %q: %w", f.Name, s, err)
		}
	}
	for _, v := range f.Arguments {
		if err := r.ResolveType(v.Type); err != nil {
			return fmt.Errorf("resolve argument %q of %q of %q: %w", v.Name, f.Name, s, err)
		}
	}
	for _, v := range f.Throws {
		if err := r.ResolveType(v.Type); err != nil {
			return fmt.Errorf("resolve exception %q of %q of %q: %w", v.Name, f.Name, s, err)
		}
	}
	return
}

var categoryMap = map[string]parser.Category{
	"bool":   parser.Category_Bool,
	"byte":   parser.Category_Byte,
	"i8":     parser.Category_Byte,
	"i16":    parser.Category_I16,
	"i32":    parser.Category_I32,
	"i64":    parser.Category_I64,
	"double": parser.Category_Double,
	"string": parser.Category_String,
	"binary": parser.Category_Binary,
	"map":    parser.Category_Map,
	"list":   parser.Category_List,
	"set":    parser.Category_Set,
}

var yes bool = true

func (r *resolver) ResolveType(t *parser.Type) (err error) {
	switch t.Name {
	case "bool", "byte", "i8", "i16", "i32", "i64", "double", "string", "binary":
		t.Category = categoryMap[t.Name]
	case "map", "list", "set":
		t.Category = categoryMap[t.Name]
		if t.Name == "map" {
			if err = r.ResolveType(t.KeyType); err != nil {
				return
			}
		}
		return r.ResolveType(t.ValueType)
	default:
		tmp := SplitType(t.Name)
		switch len(tmp) {
		case 1: // typedef, enum, struct, union, exception
			if c, exist := r.ast.Name2Category[tmp[0]]; exist {
				if c >= parser.Category_Enum && c <= parser.Category_Typedef {
					if c == parser.Category_Typedef {
						r.typedefs = append(r.typedefs, &typedefPair{
							Type: t,
							AST:  r.ast,
							Name: tmp[0],
						})
						t.IsTypedef = &yes
					}
					t.Category = c
					break
				}
				return fmt.Errorf("unexpected type category '%s' of type '%s'", c, t.Name)
			}
			return fmt.Errorf("undefined type: %q", t.Name)
		case 2: // an external type
			for i, inc := range r.ast.Includes {
				if IDLPrefix(inc.Path) != tmp[0] {
					continue
				}
				if c, exist := inc.Reference.Name2Category[tmp[1]]; exist {
					if c >= parser.Category_Enum && c <= parser.Category_Typedef {
						if c == parser.Category_Typedef {
							r.typedefs = append(r.typedefs, &typedefPair{
								Type: t,
								AST:  inc.Reference,
								Name: tmp[1],
							})
							t.IsTypedef = &yes
						}
						t.Category = c
						t.Reference = &parser.Reference{
							Name:  tmp[1],
							Index: int32(i),
						}
						r.ast.Includes[i].Used = &yes
						break
					}
				}
			}
			if t.Reference == nil {
				return fmt.Errorf("undefined type: %q", t.Name)
			}
		default:
			return fmt.Errorf("invalid type name %q", t.Name)
		}
	}
	return
}

// getEnum searches in the given AST and its includes an enum definition that matches the
// given name. If the name refers to a typedef, then its original type will be considered.
// The return value contains the target enum definition and the index of the **first**
// included IDL or -1 if the enum is defined in the given AST.
// When such an enum is not found, getEnum returns (nil, -1).
func getEnum(ast *parser.Thrift, name string) (enum *parser.Enum, includeIndex int32) {
	c, exist := ast.Name2Category[name]
	if !exist {
		return nil, -1
	}
	if c == parser.Category_Enum {
		x, ok := ast.GetEnum(name)
		if !ok {
			panic(fmt.Errorf("expect %q to be an enum in %q, not found", name, ast.Filename))
		}
		return x, -1
	}
	if c == parser.Category_Typedef {
		if x, ok := ast.GetTypedef(name); !ok {
			panic(fmt.Errorf("expect %q to be an typedef in %q, not found", name, ast.Filename))
		} else {
			if r := x.Type.Reference; r != nil {
				e, _ := getEnum(ast.Includes[r.Index].Reference, r.Name)
				if e != nil {
					return e, r.Index
				}
			}
			return getEnum(ast, x.Type.Name)
		}
	}
	return nil, -1
}

func (r *resolver) ResolveConstValue(t *parser.ConstValue) (err error) {
	switch t.Type {
	case parser.ConstType_ConstIdentifier:
		id := t.TypedValue.GetIdentifier()
		if id == "true" || id == "false" {
			return
		}
		sss := SplitValue(id)
		var ref []*parser.ConstValueExtra
		for _, ss := range sss {
			switch len(ss) {
			case 1: // constant
				if c, exist := r.ast.Name2Category[ss[0]]; exist {
					if c == parser.Category_Constant {
						ref = append(ref, &parser.ConstValueExtra{
							IsEnum: false, Index: -1, Name: ss[0],
						})
					}
				}
				continue
			case 2: // enum.value or someinclude.constant
				// TODO: if enum.value is written in typedef.value?

				// enum.value
				if enum, idx := getEnum(r.ast, ss[0]); enum != nil {
					for _, v := range enum.Values {
						if v.Name == ss[1] {
							ref = append(ref, &parser.ConstValueExtra{
								IsEnum: true, Index: idx, Name: ss[1], Sel: ss[0],
							})
						}
					}
				}
				for idx, inc := range r.ast.Includes {
					if IDLPrefix(inc.Path) != ss[0] {
						continue
					}
					if c, exist := inc.Reference.Name2Category[ss[1]]; exist {
						if c == parser.Category_Constant {
							ref = append(ref, &parser.ConstValueExtra{
								IsEnum: false, Index: int32(idx), Name: ss[1], Sel: ss[0],
							})
							r.ast.Includes[idx].Used = &yes
						}
					}
				}
			case 3: // someinclude.enum.value
				for idx, inc := range r.ast.Includes {
					if IDLPrefix(inc.Path) != ss[0] {
						continue
					}
					if enum, _ := getEnum(inc.Reference, ss[1]); enum != nil {
						for _, v := range enum.Values {
							if v.Name == ss[2] {
								ref = append(ref, &parser.ConstValueExtra{
									IsEnum: true, Index: int32(idx), Name: ss[2], Sel: ss[1],
								})
								r.ast.Includes[idx].Used = &yes
							}
						}
					}
				}
			}
		}
		switch len(ref) {
		case 0:
			return fmt.Errorf("undefined value: %q", t.TypedValue.GetIdentifier())
		case 1:
			t.Extra = ref[0]
		default:
			return fmt.Errorf("ambiguous const value %q (%d possible explainations)", t.TypedValue.GetIdentifier(), len(ref))
		}
	case parser.ConstType_ConstList:
		for _, v := range t.TypedValue.List {
			if err = r.ResolveConstValue(v); err != nil {
				return
			}
		}
	case parser.ConstType_ConstMap:
		for _, m := range t.TypedValue.Map {
			if err = r.ResolveConstValue(m.Key); err != nil {
				return
			}
			if err = r.ResolveConstValue(m.Value); err != nil {
				return
			}
		}
	default: // parser.ConstType_ConstDouble, parser.ConstType_ConstInt, parser.ConstType_ConstLiteral
		return
	}
	return
}

func (r *resolver) ResolveTypedefs() error {
	tds := r.typedefs
	cnt := len(tds)

	// a typedef may depend on another one, so we use an infinite
	// loop to process them and check if the unresolved tasks is
	// lessen by each iteration
	for len(tds) > 0 {
		var tmp []*typedefPair
		for _, t := range tds {
			if err := r.ResolveTypedef(t); err != nil {
				return err
			}
			if t.Type.Category == parser.Category_Typedef {
				// unchanged
				tmp = append(tmp, t)
			}
		}
		if len(tmp) == cnt {
			var ss []string
			for _, t := range tmp {
				ss = append(ss, fmt.Sprintf(
					"%q in %q", t.Type.Name, t.AST.Filename,
				))
			}
			return fmt.Errorf("typedefs can not be resolved: %s", strings.Join(ss, ", "))
		}
		tds = tmp
	}
	return nil
}

func (r *resolver) ResolveTypedef(t *typedefPair) error {
	td, ok := t.AST.GetTypedef(t.Name)
	if !ok {
		return fmt.Errorf("expected %q in %q a typedef, not found", t.Name, t.AST.Filename)
	}

	if td.Type.Category != parser.Category_Typedef {
		t.Type.Category = td.Type.Category
	}
	return nil
}

// Deref returns the target type that t refers to and the AST it belongs to.
// Typedef will be dereferenced.
func Deref(ast *parser.Thrift, t *parser.Type) (*parser.Thrift, *parser.Type, error) {
	if !t.IsSetReference() && !t.GetIsTypedef() {
		return ast, t, nil
	}

	if !t.IsSetReference() { // is a typedef
		if td, ok := ast.GetTypedef(t.Name); ok {
			return Deref(ast, td.Type)
		}
		return nil, nil, fmt.Errorf("expect %q a typedef in %q", t.Name, ast.Filename)
	}

	// refer to an external type
	ref := t.GetReference()
	if ref.Index < 0 || ref.Index >= int32(len(ast.Includes)) {
		return nil, nil, fmt.Errorf("(%+v) invalid ref.Index for %q: %d", t, ast.Filename, ref.Index)
	}
	ast = ast.Includes[ref.Index].Reference
	if ast.Name2Category == nil {
		return nil, nil, fmt.Errorf("AST %q is not semantically resolved", ast.Filename)
	}

	cat := ast.Name2Category[ref.Name]
	switch {
	case cat == parser.Category_Typedef:
		if td, ok := ast.GetTypedef(ref.Name); ok {
			return Deref(ast, td.Type)
		}
		return nil, nil, fmt.Errorf("expect %q a typedef in %q", ref.Name, ast.Filename)
	case parser.Category_Enum <= cat && cat <= parser.Category_Exception:
		t = &parser.Type{
			Name:     ref.Name,
			Category: cat,
		}
		return ast, t, nil
	default:
		return nil, nil, fmt.Errorf("(%+v) unexpected category for %q: %v", t, ast.Filename, cat)
	}
}
