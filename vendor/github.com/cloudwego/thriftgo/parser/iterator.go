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

// ForEachInclude calls f on each include of the current AST.
func (t *Thrift) ForEachInclude(f func(v *Include) bool) {
	for _, v := range t.Includes {
		if !f(v) {
			break
		}
	}
}

// ForEachNamepace calls f on each namespace of the current AST.
func (t *Thrift) ForEachNamepace(f func(v *Namespace) bool) {
	for _, v := range t.Namespaces {
		if !f(v) {
			break
		}
	}
}

// ForEachTypedef calls f on each typedef in the current AST.
func (t *Thrift) ForEachTypedef(f func(v *Typedef) bool) {
	for _, v := range t.Typedefs {
		if !f(v) {
			break
		}
	}
}

// ForEachConstant calls f on each constant in the current AST.
func (t *Thrift) ForEachConstant(f func(v *Constant) bool) {
	for _, v := range t.Constants {
		if !f(v) {
			break
		}
	}
}

// ForEachEnum calls f on each enum in the current AST.
func (t *Thrift) ForEachEnum(f func(v *Enum) bool) {
	for _, v := range t.Enums {
		if !f(v) {
			break
		}
	}
}

// ForEachStructLike calls f on each struct-like in the current AST.
func (t *Thrift) ForEachStructLike(f func(v *StructLike) bool) {
	for _, v := range t.GetStructLikes() {
		if !f(v) {
			break
		}
	}
}

// ForEachStruct calls f on each struct in the current AST.
func (t *Thrift) ForEachStruct(f func(v *StructLike) bool) {
	for _, v := range t.Structs {
		if !f(v) {
			break
		}
	}
}

// ForEachUnion calls f on each union in the current AST.
func (t *Thrift) ForEachUnion(f func(v *StructLike) bool) {
	for _, v := range t.Unions {
		if !f(v) {
			break
		}
	}
}

// ForEachException calls f on each exception in the current AST.
func (t *Thrift) ForEachException(f func(v *StructLike) bool) {
	for _, v := range t.Exceptions {
		if !f(v) {
			break
		}
	}
}

// ForEachService calls f on each service in the current AST.
func (t *Thrift) ForEachService(f func(v *Service) bool) {
	for _, v := range t.Services {
		if !f(v) {
			break
		}
	}
}

// ForEachFunction calls f on each function in the current service.
func (s *Service) ForEachFunction(f func(v *Function) bool) {
	for _, v := range s.Functions {
		if !f(v) {
			break
		}
	}
}

// ForEachArgument calls f on each argument of the current function.
func (p *Function) ForEachArgument(f func(v *Field) bool) {
	for _, v := range p.Arguments {
		if !f(v) {
			break
		}
	}
}

// ForEachThrow calls f on each throw of the current function.
func (p *Function) ForEachThrow(f func(v *Field) bool) {
	for _, v := range p.Throws {
		if !f(v) {
			break
		}
	}
}

// ForEachField calls f on each field of the current struct-like.
func (s *StructLike) ForEachField(f func(v *Field) bool) {
	for _, v := range s.Fields {
		if !f(v) {
			break
		}
	}
}
