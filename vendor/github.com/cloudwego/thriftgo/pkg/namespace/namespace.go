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

// Package namespace implements a namespace to solve name collision.
package namespace

import (
	"fmt"
	"strings"
)

// Namespace is a container that maps IDs to names (both strings) while
// avoids collision between names by renaming.
type Namespace interface {
	// Add tries to create a mapping from ID to name. If name
	// is used by other ID, then a renamed one will be returned.
	Add(name, id string) (result string)

	// Get returns the Added name of id. If none found, the result will
	// be an empty string.
	Get(id string) (name string)

	// ID is the reverse operation of Get. It returns the ID that maps
	// to the given name.
	ID(name string) (id string)

	// Reserve tries to create a mapping from ID to name without any
	// renaming. The result indicates whether it succeeds.
	Reserve(name, id string) (ok bool)

	// MustReserve tries to create a mapping from ID to name without any
	// renaming. If the name is occupied, this function will panic.
	MustReserve(name, id string)

	// Iterate passes each key and value in the namespace to f.
	// If f returns false, Iterate stops the iteration.
	Iterate(v func(name, id string) (shouldContinue bool))
}

// NewNamespace creates a namespace with the given rename function.
// The first parameter for the rename function is the preferred name
// which is not usable due to collision. The second parameter indicates
// how many different IDs has claimed the same name.
func NewNamespace(rename func(string, int) string) Namespace {
	return &namespace{
		name2id: make(map[string]string),
		id2name: make(map[string]string),
		rename:  rename,
	}
}

type namespace struct {
	name2id map[string]string
	id2name map[string]string
	rename  func(name string, cnt int) string
}

func (ns *namespace) Reserve(name, id string) (ok bool) {
	if _, exist := ns.name2id[name]; exist {
		return false
	}
	ns.name2id[name] = id
	ns.id2name[id] = name
	return true
}

func (ns *namespace) MustReserve(name, id string) {
	if !ns.Reserve(name, id) {
		panic(fmt.Errorf("failed to reserve %s for %s: %s", name, id, ns.name2id[name]))
	}
}

func (ns *namespace) Get(id string) string {
	return ns.id2name[id]
}

func (ns *namespace) ID(name string) string {
	return ns.name2id[name]
}

func (ns *namespace) Add(name, id string) (res string) {
	var cnt int
	res = name
	cur, exist := ns.name2id[res]
	for exist && cur != id {
		cnt++
		res = ns.rename(name, cnt)
		cur, exist = ns.name2id[res]
	}
	ns.name2id[res] = id
	ns.id2name[id] = res
	return
}

func (ns *namespace) Iterate(v func(name, id string) bool) {
	for n, i := range ns.name2id {
		if !v(n, i) {
			break
		}
	}
}

// UnderscoreSuffix renames names by adding underscores as suffix.
func UnderscoreSuffix(name string, cnt int) string {
	return name + strings.Repeat("_", cnt)
}

// NumberSuffix renames names by adding a numeric suffix.
func NumberSuffix(name string, cnt int) string {
	return fmt.Sprintf("%s%d", name, cnt)
}
