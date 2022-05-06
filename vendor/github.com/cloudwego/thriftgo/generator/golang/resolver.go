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
	"github.com/cloudwego/thriftgo/semantic"
)

// Resolver resolves names for types, names and initialization value for thrift AST
// nodes in a scope (the root scope).
type Resolver struct {
	root *Scope
	util *CodeUtils
}

// NewResolver creates a new Resolver with the given scope.
func NewResolver(root *Scope, cu *CodeUtils) *Resolver {
	return &Resolver{
		root: root,
		util: cu,
	}
}

// GetDefaultValueTypeName returns a type name suitable for the default value of the given field.
func (r *Resolver) GetDefaultValueTypeName(f *parser.Field) (TypeName, error) {
	t, err := r.ResolveFieldTypeName(f)
	if err != nil {
		return "", err
	}
	if IsBaseType(f.Type) {
		t = t.Deref()
	}
	return t, nil
}

// GetFieldInit returns the initialization code for a field.
// The given field must have a default value.
func (r *Resolver) GetFieldInit(f *parser.Field) (Code, error) {
	return r.GetConstInit(f.Name, f.Type, f.Default)
}

// GetConstInit returns the initialization code for a constant.
func (r *Resolver) GetConstInit(name string, t *parser.Type, v *parser.ConstValue) (Code, error) {
	return r.ResolveConst(r.root, name, t, v)
}

// ResolveFieldTypeName returns a legal type name in go for the given field.
func (r *Resolver) ResolveFieldTypeName(f *parser.Field) (TypeName, error) {
	tn, err := r.GetTypeName(r.root, f.Type)
	if err != nil {
		return "", err
	}

	if NeedRedirect(f) {
		return "*" + tn.Deref(), nil
	}
	return tn, nil
}

// ResolveTypeName returns a legal type name in go for the given AST type.
func (r *Resolver) ResolveTypeName(t *parser.Type) (TypeName, error) {
	tn, err := r.GetTypeName(r.root, t)
	if err != nil {
		return "", err
	}
	if t.Category.IsStructLike() {
		return "*" + tn, nil
	}
	return tn, nil
}

// GetTypeName returns a an type name (with selector if necessary) of the
// given type to be used in the root file.
// The type t must be a parser.Type associated with g.
func (r *Resolver) GetTypeName(g *Scope, t *parser.Type) (name TypeName, err error) {
	str, err := r.getTypeName(g, t)
	return TypeName(str), err
}

func (r *Resolver) getTypeName(g *Scope, t *parser.Type) (name string, err error) {
	if ref := t.GetReference(); ref != nil {
		g = g.includes[ref.Index].Scope
		name = g.globals.Get(ref.Name)
	} else {
		if s := baseTypes[t.Name]; s != "" {
			return s, nil
		}
		if isContainerTypes[t.Name] {
			return r.getContainerTypeName(g, t)
		}
		name = g.globals.Get(t.Name)
	}

	if name == "" {
		return "", fmt.Errorf("getTypeName failed: type[%v] file[%s]", t, g.ast.Filename)
	}

	if g.namespace != r.root.namespace {
		pkg := r.root.includeIDL(r.util, g.ast)
		name = pkg + "." + name
	}
	return
}

func (r *Resolver) getContainerTypeName(g *Scope, t *parser.Type) (name string, err error) {
	if t.Name == "map" {
		var k string
		if t.KeyType.Category == parser.Category_Binary {
			k = "string" // 'binary => string' for key type in map
		} else {
			k, err = r.getTypeName(g, t.KeyType)
			if err != nil {
				return "", fmt.Errorf("resolve key type of '%s' failed: %w", t, err)
			}
			if t.KeyType.Category.IsStructLike() {
				// when a struct-like is used as key of a map, it must
				// generte a pointer type instead of the struct itself
				k = "*" + k
			}
		}
		name = fmt.Sprintf("map[%s]", k)
	} else {
		name = "[]" // sets and lists compile into slices
	}

	v, err := r.getTypeName(g, t.ValueType)
	if err != nil {
		return "", fmt.Errorf("resolve value type of '%s' failed: %w", t, err)
	}

	if t.ValueType.Category.IsStructLike() && !r.util.Features().ValueTypeForSIC {
		v = "*" + v // generate pointer type for struct-like by default
	}
	return name + v, nil // map[k]v or []v
}

// getIDValue returns the literal representation of a const value.
// The extra must be associated with g and from a const value that has
// type parser.ConstType_ConstIdentifier.
func (r *Resolver) getIDValue(g *Scope, extra *parser.ConstValueExtra) (v string, ok bool) {
	if extra.Index == -1 {
		if extra.IsEnum {
			enum, ok := g.ast.GetEnum(extra.Sel)
			if !ok {
				return "", false
			}
			if en := g.Enum(enum.Name); en != nil {
				if ev := en.Value(extra.Name); ev != nil {
					v = ev.GoName().String()
				}
			}
		} else {
			v = g.globals.Get(extra.Name)
		}
	} else {
		g = g.includes[extra.Index].Scope
		extra = &parser.ConstValueExtra{
			Index:  -1,
			IsEnum: extra.IsEnum,
			Name:   extra.Name,
			Sel:    extra.Sel,
		}
		return r.getIDValue(g, extra)
	}
	if v != "" && g != r.root {
		pkg := r.root.includeIDL(r.util, g.ast)
		v = pkg + "." + v
	}
	return v, v != ""
}

// ResolveConst returns the initialization code for a constant or a default value.
// The type t must be a parser.Type associated with g.
func (r *Resolver) ResolveConst(g *Scope, name string, t *parser.Type, v *parser.ConstValue) (Code, error) {
	str, err := r.resolveConst(g, name, t, v)
	return Code(str), err
}

func (r *Resolver) resolveConst(g *Scope, name string, t *parser.Type, v *parser.ConstValue) (string, error) {
	goType, err := r.getTypeName(g, t)
	if err != nil {
		return "", err
	}
	switch t.Category {
	case parser.Category_Bool:
		return r.onBool(g, name, t, v)

	case parser.Category_Byte, parser.Category_I16, parser.Category_I32, parser.Category_I64:
		return r.onInt(g, name, t, v)

	case parser.Category_Double:
		return r.onDouble(g, name, t, v)

	case parser.Category_String, parser.Category_Binary:
		return r.onStrBin(g, name, t, v)

	case parser.Category_Enum:
		return r.onEnum(g, name, t, v)

	case parser.Category_Set, parser.Category_List:
		var ss []string
		switch v.Type {
		case parser.ConstType_ConstList:
			elemName := "element of " + name
			for _, elem := range v.TypedValue.GetList() {
				str, err := r.resolveConst(g, elemName, t.ValueType, elem)
				if err != nil {
					return "", err
				}
				ss = append(ss, str+",")
			}
			if len(ss) == 0 {
				return goType + "{}", nil
			}
			return fmt.Sprintf("%s{\n%s\n}", goType, strings.Join(ss, "\n")), nil
		case parser.ConstType_ConstInt, parser.ConstType_ConstDouble,
			parser.ConstType_ConstLiteral, parser.ConstType_ConstMap:
			return goType + "{}", nil
		}

	case parser.Category_Map:
		var kvs []string
		switch v.Type {
		case parser.ConstType_ConstMap:
			for _, mcv := range v.TypedValue.Map {
				keyName := "key of " + name
				key, err := r.resolveConst(g, keyName, r.bin2str(t.KeyType), mcv.Key)
				if err != nil {
					return "", err
				}
				valName := "value of " + name
				val, err := r.resolveConst(g, valName, t.ValueType, mcv.Value)
				if err != nil {
					return "", err
				}
				kvs = append(kvs, fmt.Sprintf("%s: %s,", key, val))
			}
			if len(kvs) == 0 {
				return goType + "{}", nil
			}
			return fmt.Sprintf("%s{\n%s\n}", goType, strings.Join(kvs, "\n")), nil

		case parser.ConstType_ConstInt, parser.ConstType_ConstDouble,
			parser.ConstType_ConstLiteral, parser.ConstType_ConstList:
			return goType + "{}", nil
		}

	case parser.Category_Struct, parser.Category_Union, parser.Category_Exception:
		if v.Type != parser.ConstType_ConstMap {
			// constant value of a struct-like must be a map literal
			break
		}

		// get the target struct-like with typedef dereferenced
		file, st, err := r.getStructLike(g, t)
		if err != nil {
			return "", err
		}

		var kvs []string
		for _, mcv := range v.TypedValue.Map {
			if mcv.Key.Type != parser.ConstType_ConstLiteral {
				return "", fmt.Errorf("expect literals as keys in default value of struct type '%s', got '%s'", name, mcv.Key.Type)
			}
			n := mcv.Key.TypedValue.GetLiteral()

			f, ok := st.GetField(n)
			if !ok {
				return "", fmt.Errorf("field %q not found in %q (%q): %v",
					n, st.Name, file.ast.Filename, v,
				)
			}
			typ, err := r.getTypeName(file, f.Type)
			if err != nil {
				return "", fmt.Errorf("get type name of %q in %q (%q): %w",
					n, st.Name, file.ast.Filename, err,
				)
			}

			key := file.StructLike(st.Name).Field(f.Name).GoName().String()
			val, err := r.resolveConst(file, st.Name+"."+f.Name, f.Type, mcv.Value)
			if err != nil {
				return "", err
			}

			if NeedRedirect(f) {
				if f.Type.Category.IsBaseType() {
					// a trick to create pointers without temporary variables
					val = fmt.Sprintf("(&struct{x %s}{%s}).x", typ, val)
				}
				if !strings.HasPrefix(val, "&") {
					val = "&" + val
				}
			}
			kvs = append(kvs, fmt.Sprintf("%s: %s,", key, val))
		}
		if len(kvs) == 0 {
			return "&" + goType + "{}", nil
		}
		return fmt.Sprintf("&%s{\n%s\n}", goType, strings.Join(kvs, "\n")), nil
	}
	return "", fmt.Errorf("type error: '%s' was declared as type %s(value[%v] file[%s] Category[%s])", name, t, v, g.ast.Filename, t.Category)
}

func (r *Resolver) onBool(g *Scope, name string, t *parser.Type, v *parser.ConstValue) (string, error) {
	switch v.Type {
	case parser.ConstType_ConstInt:
		val := v.TypedValue.GetInt()
		return fmt.Sprint(val > 0), nil
	case parser.ConstType_ConstDouble:
		val := v.TypedValue.GetDouble()
		return fmt.Sprint(val > 0), nil
	case parser.ConstType_ConstIdentifier:
		s := v.TypedValue.GetIdentifier()
		if s == "true" || s == "false" {
			return s, nil
		}

		if val, ok := r.getIDValue(g, v.Extra); ok {
			return val, nil
		}
		return "", fmt.Errorf("undefined value: %q", s)
	}
	return "", fmt.Errorf("type error: '%s' was declared as type %s", name, t)
}

func (r *Resolver) onInt(g *Scope, name string, t *parser.Type, v *parser.ConstValue) (string, error) {
	switch v.Type {
	case parser.ConstType_ConstInt:
		val := v.TypedValue.GetInt()
		return fmt.Sprint(val), nil
	case parser.ConstType_ConstIdentifier:
		s := v.TypedValue.GetIdentifier()
		if s == "true" {
			return "1", nil
		}
		if s == "false" {
			return "0", nil
		}
		if val, ok := r.getIDValue(g, v.Extra); ok {
			goType, _ := r.getTypeName(g, t)
			val = fmt.Sprintf("%s(%s)", goType, val)
			return val, nil
		}
		return "", fmt.Errorf("undefined value: %q", s)
	}
	return "", fmt.Errorf("type error: '%s' was declared as type %s", name, t)
}

func (r *Resolver) onDouble(g *Scope, name string, t *parser.Type, v *parser.ConstValue) (string, error) {
	switch v.Type {
	case parser.ConstType_ConstInt:
		val := v.TypedValue.GetInt()
		return fmt.Sprint(val) + ".0", nil
	case parser.ConstType_ConstDouble:
		val := v.TypedValue.GetDouble()
		return fmt.Sprint(val), nil
	case parser.ConstType_ConstIdentifier:
		s := v.TypedValue.GetIdentifier()
		if s == "true" {
			return "1.0", nil
		}
		if s == "false" {
			return "0.0", nil
		}
		if val, ok := r.getIDValue(g, v.Extra); ok {
			return val, nil
		}
		return "", fmt.Errorf("undefined value: %q", s)
	}
	return "", fmt.Errorf("type error: '%s' was declared as type %s", name, t)
}

func (r *Resolver) onStrBin(g *Scope, name string, t *parser.Type, v *parser.ConstValue) (res string, err error) {
	defer func() {
		if err == nil && t.Category == parser.Category_Binary {
			res = "[]byte(" + res + ")"
		}
	}()
	switch v.Type {
	case parser.ConstType_ConstLiteral:
		return fmt.Sprintf("%q", v.TypedValue.GetLiteral()), nil
	case parser.ConstType_ConstIdentifier:
		s := v.TypedValue.GetIdentifier()
		if s == "true" || s == "false" {
			break
		}

		if val, ok := r.getIDValue(g, v.Extra); ok {
			return val, nil
		}
		return "", fmt.Errorf("undefined value: %q", s)
	default:
	}
	return "", fmt.Errorf("type error: '%s' was declared as type %s", name, t)
}

func (r *Resolver) onEnum(g *Scope, name string, t *parser.Type, v *parser.ConstValue) (string, error) {
	switch v.Type {
	case parser.ConstType_ConstInt:
		return fmt.Sprintf("%d", v.TypedValue.GetInt()), nil
	case parser.ConstType_ConstIdentifier:
		val, ok := r.getIDValue(g, v.Extra)
		if ok {
			return val, nil
		}
	}
	return "", fmt.Errorf("expect const value for %q is a int or enum, got %+v", name, v)
}

func (r *Resolver) getStructLike(g *Scope, t *parser.Type) (f *Scope, s *parser.StructLike, err error) {
	ast, x, err := semantic.Deref(g.ast, t)
	if err != nil {
		err = fmt.Errorf("expect %q a typedef or struct-like in %q: %w",
			t.Name, g.ast.Filename, err)
		return nil, nil, err
	}
	if ast == g.ast {
		f = g
	} else {
		if f = r.util.scopeCache[ast]; f == nil {
			panic(fmt.Errorf("%q not build", ast.Filename))
		}
	}
	for _, y := range ast.GetStructLikes() {
		if x.Name == y.Name {
			s = y
		}
	}
	if s == nil {
		err = fmt.Errorf("expect %q a struct-like in %q: not found: %v",
			x.Name, ast.Filename, x == t)
		return nil, nil, err
	}
	return
}

func (r *Resolver) bin2str(t *parser.Type) *parser.Type {
	if t.Category == parser.Category_Binary {
		r := *t
		r.Category = parser.Category_String
		return &r
	}
	return t
}
