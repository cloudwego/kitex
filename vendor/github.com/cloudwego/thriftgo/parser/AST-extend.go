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

package parser

import (
	"fmt"
	"path/filepath"
	"strings"
)

func indent(lines, pad string) string {
	var ss []string
	for _, s := range strings.Split(lines, "\n") {
		ss = append(ss, pad+s)
	}
	return strings.Join(ss, "\n")
}

// Append append key value pair to Annotation slice.
func (a *Annotations) Append(key string, value string) {
	old := *a
	for _, anno := range old {
		if anno.Key == key {
			anno.Values = append(anno.Values, value)
			return
		}
	}
	*a = append(old, &Annotation{Key: key, Values: []string{value}})
}

// Get return annotations values.
func (a *Annotations) Get(key string) []string {
	for _, anno := range *a {
		if anno.Key == key {
			return anno.Values
		}
	}
	return nil
}

// IsDefault tells whether a field type is default.
func (r FieldType) IsDefault() bool {
	return r == FieldType_Default
}

// IsRequired tells whether a field type is required.
func (r FieldType) IsRequired() bool {
	return r == FieldType_Required
}

// IsOptional tells whether a field type is optional.
func (r FieldType) IsOptional() bool {
	return r == FieldType_Optional
}

func (t *ConstValue) String() string {
	if t == nil {
		return "<nil>"
	}
	var val string
	switch t.Type {
	case ConstType_ConstDouble:
		val = fmt.Sprintf("%f", *t.TypedValue.Double)
	case ConstType_ConstInt:
		val = fmt.Sprintf("%d", *t.TypedValue.Int)
	case ConstType_ConstLiteral:
		val = fmt.Sprintf("\"%s\"", *t.TypedValue.Literal)
	case ConstType_ConstIdentifier:
		val = *t.TypedValue.Identifier
	case ConstType_ConstList:
		if len(t.TypedValue.List) == 0 {
			return "{}"
		}
		var ss []string
		ss = append(ss, "[")
		for _, item := range t.TypedValue.List {
			ss = append(ss, indent(item.String(), "  "))
		}
		ss = append(ss, "]")
		val = strings.Join(ss, "\n")
	case ConstType_ConstMap:
		if len(t.TypedValue.Map) == 0 {
			return "{}"
		}
		var ss []string
		ss = append(ss, "{")
		for _, kv := range t.TypedValue.Map {
			key := kv.Key.String()
			val := kv.Value.String()
			pair := key + ": " + val
			ss = append(ss, indent(pair, "  "))
		}
		ss = append(ss, "}")
		val = strings.Join(ss, "\n")
	default:
		return fmt.Sprintf("ConstValue(%+v)", *t)
	}
	return fmt.Sprintf("%s(%s)", t.Type, val)
}

func (t *Type) String() string {
	switch t.Name {
	case "map":
		return fmt.Sprintf("map<%s,%s>", t.KeyType, t.ValueType)
	case "list":
		return fmt.Sprintf("list<%s>", t.ValueType)
	case "set":
		return fmt.Sprintf("set<%s>", t.ValueType)
	}
	return t.Name
}

// GetField returns a field of the struct-like that matches the name.
func (s *StructLike) GetField(name string) (*Field, bool) {
	for _, fi := range s.Fields {
		if fi.Name == name {
			return fi, true
		}
	}
	return nil, false
}

func dfs(t, root *Thrift, out chan *Thrift, set map[string]bool) {
	if t != nil && !set[t.Filename] {
		set[t.Filename] = true
		for _, inc := range t.Includes {
			dfs(inc.Reference, root, out, set)
		}
		out <- t
		if t == root {
			close(out)
		}
	}
}

// DepthFirstSearch returns a channal providing Thrift ASTs in a depth-first order without duplication.
// Unparsed references will be ignored.
func (t *Thrift) DepthFirstSearch() chan *Thrift {
	res := make(chan *Thrift)
	set := make(map[string]bool)
	go dfs(t, t, res, set)
	return res
}

// GetReference return a AST that matches the given name.
// References should be initialized before calling this method.
func (t *Thrift) GetReference(refname string) (*Thrift, bool) {
	for _, inc := range t.Includes {
		ref := inc.Reference
		if ref != nil && refName(inc.Path) == refname {
			return ref, true
		}
	}
	return nil, false
}

// GetNamespace returns a namespace for the language.
// When both "*" and the language have a namespace defined, the latter is preferred.
func (t *Thrift) GetNamespace(lang string) (ns string, found bool) {
	for _, n := range t.Namespaces {
		if n.Language == lang {
			return n.Name, true
		}
		if n.Language == "*" {
			ns, found = n.Name, true
		}
	}
	return
}

// GetNamespaceOrReferenceName returns a namespace for the language. If the namespace is not defined,
// then the base name without extension of the IDL will be returned.
func (t *Thrift) GetNamespaceOrReferenceName(lang string) string {
	ns, found := t.GetNamespace(lang)
	if found {
		return ns
	}
	return strings.ToLower(strings.TrimSuffix(filepath.Base(t.Filename), filepath.Ext(t.Filename)))
}

// GetService returns a Service node that matches the name.
func (t *Thrift) GetService(name string) (*Service, bool) {
	for _, svc := range t.Services {
		if svc.Name == name {
			return svc, true
		}
	}
	return nil, false
}

// GetStructLikes returns all struct-like definitions in the AST.
func (t *Thrift) GetStructLikes() (ss []*StructLike) {
	ss = append(ss, t.Structs...)
	ss = append(ss, t.Unions...)
	ss = append(ss, t.Exceptions...)
	return
}

// GetStruct returns a Struct node that matches the name.
func (t *Thrift) GetStruct(name string) (*StructLike, bool) {
	for _, st := range t.Structs {
		if st.Name == name {
			return st, true
		}
	}
	return nil, false
}

// GetUnion returns a Union node that matches the name.
func (t *Thrift) GetUnion(name string) (*StructLike, bool) {
	for _, un := range t.Unions {
		if un.Name == name {
			return un, true
		}
	}
	return nil, false
}

// GetException returns an Exception that matches the name.
func (t *Thrift) GetException(name string) (*StructLike, bool) {
	for _, ec := range t.Exceptions {
		if ec.Name == name {
			return ec, true
		}
	}
	return nil, false
}

// GetTypedef returns a Typedef node that matches the alias name.
func (t *Thrift) GetTypedef(alias string) (*Typedef, bool) {
	for _, td := range t.Typedefs {
		if td.Alias == alias {
			return td, true
		}
	}
	return nil, false
}

// GetConstant returns a Constant node that mathces the name.
func (t *Thrift) GetConstant(name string) (*Constant, bool) {
	for _, ct := range t.Constants {
		if ct.Name == name {
			return ct, true
		}
	}
	return nil, false
}

// GetEnum returns an Enum node that matches the name.
func (t *Thrift) GetEnum(name string) (*Enum, bool) {
	for _, e := range t.Enums {
		if e.Name == name {
			return e, true
		}
	}
	return nil, false
}
