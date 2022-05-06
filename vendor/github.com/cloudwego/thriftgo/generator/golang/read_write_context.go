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

	"github.com/cloudwego/thriftgo/parser"
)

// ReadWriteContext contains information for generating codes in ReadField* and
// WriteField* functions. Each context stands for a struct field, a map key, a
// map value, a list elemement, or a set elemement.
type ReadWriteContext struct {
	Type      *parser.Type
	TypeName  TypeName // The type name in Go code
	TypeID    string   // For `thrift.TProtocol.(Read|Write)${TypeID}` methods
	IsPointer bool     // Whether the target type is a pointer type in Go

	KeyCtx *ReadWriteContext // sub-context if the type is map
	ValCtx *ReadWriteContext // sub-context if the type is container

	Target   string // The target for assignment
	Source   string // The variable for right hand operand in deep-equal
	NeedDecl bool   // Whether a declaration of target is needed

	ids map[string]int // Prefix => local variable index
}

// GenID returns a local variable with the given name as prefix.
func (c *ReadWriteContext) GenID(prefix string) (name string) {
	name = prefix
	if id := c.ids[prefix]; id > 0 {
		name += fmt.Sprint(id)
	}
	c.ids[prefix]++
	return
}

// WithDecl claims that the context needs a variable declaration.
func (c *ReadWriteContext) WithDecl() *ReadWriteContext {
	c.NeedDecl = true
	return c
}

// WithTarget sets the target name.
func (c *ReadWriteContext) WithTarget(t string) *ReadWriteContext {
	c.Target = t
	return c
}

// WithSource sets the source name.
func (c *ReadWriteContext) WithSource(s string) *ReadWriteContext {
	c.Source = s
	return c
}

func (c *ReadWriteContext) asKeyCtx() *ReadWriteContext {
	switch c.TypeID {
	case typeids.Struct:
		c.TypeName = c.TypeName.Deref()
		c.IsPointer = false
	case typeids.Binary:
		c.TypeName = "string"
	}
	return c
}

func mkRWCtx(r *Resolver, s *Scope, t *parser.Type, top *ReadWriteContext) (*ReadWriteContext, error) {
	tn, err := r.GetTypeName(s, t)
	if err != nil {
		return nil, err
	}
	if t.Category.IsStructLike() {
		tn = tn.Pointerize()
	}
	ctx := &ReadWriteContext{
		Type:      t,
		TypeName:  tn,
		TypeID:    GetTypeID(t),
		IsPointer: tn.IsPointer(),
	}
	if top != nil {
		ctx.ids = top.ids // share the namespace for temporary variables
	} else {
		ctx.ids = make(map[string]int)
	}

	// create sub-contexts.
	if t.Category == parser.Category_Map {
		ss, tt, err := r.util.GetKeyType(s, t)
		if err != nil {
			return nil, err
		}
		if ctx.KeyCtx, err = mkRWCtx(r, ss, tt, ctx); err != nil {
			return nil, err
		}
		ctx.KeyCtx.asKeyCtx()
	}

	if t.Category.IsContainerType() {
		ss, tt, err := r.util.GetValType(s, t)
		if err != nil {
			return nil, err
		}
		if ctx.ValCtx, err = mkRWCtx(r, ss, tt, ctx); err != nil {
			return nil, err
		}
	}
	return ctx, nil
}
