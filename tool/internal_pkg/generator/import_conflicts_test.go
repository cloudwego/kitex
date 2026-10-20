// Copyright 2026 CloudWeGo Authors
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

package generator

import (
	"strings"
	"testing"
)

// buildIssue1892Services fabricates the ServiceInfo graph the protoc plugin
// produces for the layout in kitex#1892: combine.proto (example/v1) imports
// both packagea/v1 and packageb/v1, so every alias derives to "v1".
func buildIssue1892Services() []*ServiceInfo {
	dep := func(importPath, goName string) *Parameter {
		base := importPath[strings.LastIndex(importPath, "/")+1:]
		return &Parameter{
			Deps: []PkgInfo{{PkgRefName: base, ImportPath: importPath}},
			Name: "Req",
			Type: "*" + base + "." + goName,
		}
	}
	method := func(name, depPath, goName string) *MethodInfo {
		return &MethodInfo{
			PkgInfo:     PkgInfo{PkgRefName: "v1", ImportPath: "kitexidltest/kitex_gen/example/v1"},
			ServiceName: "CombineService",
			Name:        name,
			Args:        []*Parameter{dep(depPath, goName)},
			Resp:        dep(depPath, goName),
		}
	}
	si := &ServiceInfo{
		PkgInfo:     PkgInfo{PkgName: "example", PkgRefName: "v1", ImportPath: "kitexidltest/kitex_gen/example/v1"},
		ServiceName: "CombineService",
		Methods: []*MethodInfo{
			method("GetA", "kitexidltest/kitex_gen/base/packagea/v1", "MessageA"),
			method("GetB", "kitexidltest/kitex_gen/base/packageb/v1", "MessageB"),
		},
	}
	si.ServiceTypeName = func() string { return si.PkgRefName + "." + si.ServiceName }
	return []*ServiceInfo{si}
}

func allPkgInfos(ss []*ServiceInfo) []*PkgInfo {
	var out []*PkgInfo
	for _, si := range ss {
		out = append(out, &si.PkgInfo)
		for _, m := range si.Methods {
			out = append(out, &m.PkgInfo)
			for _, a := range m.Args {
				for i := range a.Deps {
					out = append(out, &a.Deps[i])
				}
			}
			if m.Resp != nil {
				for i := range m.Resp.Deps {
					out = append(out, &m.Resp.Deps[i])
				}
			}
		}
	}
	return out
}

func TestFixImportConflicts_Issue1892(t *testing.T) {
	ss := buildIssue1892Services()
	FixImportConflicts(ss)

	// every distinct import path must now have a unique alias
	aliasByPath := map[string]string{}
	for _, pi := range allPkgInfos(ss) {
		if prev, ok := aliasByPath[pi.ImportPath]; ok && prev != pi.PkgRefName {
			t.Fatalf("inconsistent alias for %q: %q vs %q", pi.ImportPath, prev, pi.PkgRefName)
		}
		aliasByPath[pi.ImportPath] = pi.PkgRefName
	}
	seen := map[string]string{}
	for path, alias := range aliasByPath {
		if other, dup := seen[alias]; dup {
			t.Fatalf("alias %q shared by %q and %q", alias, other, path)
		}
		seen[alias] = path
	}

	// type strings must reference the renamed aliases
	wantTypes := map[string]string{
		"GetA": "*packagea_v1.MessageA",
		"GetB": "*packageb_v1.MessageB",
	}
	for _, m := range ss[0].Methods {
		if want := wantTypes[m.Name]; want != "" {
			if m.Args[0].Type != want {
				t.Errorf("%s arg type = %q, want %q", m.Name, m.Args[0].Type, want)
			}
			if m.Resp.Type != want {
				t.Errorf("%s resp type = %q, want %q", m.Name, m.Resp.Type, want)
			}
		}
	}

	// the service type name closure must keep the service package alias
	if stn := ss[0].ServiceTypeName(); stn != "v1.CombineService" {
		t.Errorf("ServiceTypeName() = %q, want v1.CombineService", stn)
	}
}

func TestFixImportConflicts_NoCollision(t *testing.T) {
	si := &ServiceInfo{
		PkgInfo:     PkgInfo{PkgRefName: "echo", ImportPath: "example.com/kitex_gen/echo"},
		ServiceName: "Echo",
		Methods: []*MethodInfo{{
			PkgInfo: PkgInfo{PkgRefName: "echo", ImportPath: "example.com/kitex_gen/echo"},
			Name:    "Ping",
			Args: []*Parameter{{
				Deps: []PkgInfo{{PkgRefName: "base", ImportPath: "example.com/kitex_gen/base"}},
				Type: "*base.Request",
			}},
		}},
	}
	ss := []*ServiceInfo{si}
	FixImportConflicts(ss)

	if si.PkgRefName != "echo" {
		t.Errorf("service alias changed: %q", si.PkgRefName)
	}
	if si.Methods[0].Args[0].Deps[0].PkgRefName != "base" {
		t.Errorf("dep alias changed: %q", si.Methods[0].Args[0].Deps[0].PkgRefName)
	}
	if si.Methods[0].Args[0].Type != "*base.Request" {
		t.Errorf("type changed: %q", si.Methods[0].Args[0].Type)
	}
}

func TestFixImportConflicts_WidensWithParentSegments(t *testing.T) {
	// "foo_v1" is already taken by an unrelated import, so the colliding
	// group must widen further to stay unique
	mk := func(alias, path, typ string) *Parameter {
		return &Parameter{Deps: []PkgInfo{{PkgRefName: alias, ImportPath: path}}, Type: typ}
	}
	si := &ServiceInfo{
		PkgInfo:     PkgInfo{PkgRefName: "svc", ImportPath: "example.com/gen/svc"},
		ServiceName: "S",
		Methods: []*MethodInfo{{
			PkgInfo: PkgInfo{PkgRefName: "svc", ImportPath: "example.com/gen/svc"},
			Name:    "M",
			Args: []*Parameter{
				mk("v1", "example.com/a/foo/v1", "*v1.MA"),
				mk("v1", "example.com/b/foo/v1", "*v1.MB"),
				mk("foo_v1", "example.com/c/foo_v1", "*foo_v1.MC"),
			},
		}},
	}
	FixImportConflicts([]*ServiceInfo{si})

	got := map[string]string{}
	for _, a := range si.Methods[0].Args {
		d := a.Deps[0]
		got[d.ImportPath] = d.PkgRefName
	}
	if got["example.com/a/foo/v1"] != "v1" {
		t.Errorf("first path should keep original alias, got %q", got["example.com/a/foo/v1"])
	}
	if got["example.com/b/foo/v1"] != "b_foo_v1" {
		t.Errorf("expected widened alias b_foo_v1, got %q", got["example.com/b/foo/v1"])
	}
	if got["example.com/c/foo_v1"] != "foo_v1" {
		t.Errorf("unrelated unique alias changed: %q", got["example.com/c/foo_v1"])
	}
	if si.Methods[0].Args[1].Type != "*b_foo_v1.MB" {
		t.Errorf("type not rewritten: %q", si.Methods[0].Args[1].Type)
	}
}

func TestFixImportConflicts_Deterministic(t *testing.T) {
	first := buildIssue1892Services()
	second := buildIssue1892Services()
	FixImportConflicts(first)
	FixImportConflicts(second)

	pi1, pi2 := allPkgInfos(first), allPkgInfos(second)
	if len(pi1) != len(pi2) {
		t.Fatalf("pkg info count mismatch")
	}
	for i := range pi1 {
		if pi1[i].PkgRefName != pi2[i].PkgRefName {
			t.Fatalf("nondeterministic rename at %d: %q vs %q", i, pi1[i].PkgRefName, pi2[i].PkgRefName)
		}
	}
}

func TestFixImportConflicts_SharedCombineServiceMethods(t *testing.T) {
	// combine services share method pointers; the walk must not double-apply
	ss := buildIssue1892Services()
	combine := &ServiceInfo{
		PkgInfo:         PkgInfo{PkgRefName: "v1", ImportPath: "kitexidltest/kitex_gen/example/v1"},
		ServiceName:     "CombineService",
		CombineServices: ss,
		Methods:         ss[0].AllMethods(),
	}
	FixImportConflicts(append(ss, combine))

	for _, m := range combine.Methods {
		if !strings.HasPrefix(m.Args[0].Type, "*packagea_v1.") && !strings.HasPrefix(m.Args[0].Type, "*packageb_v1.") {
			t.Errorf("shared method type not rewritten exactly once: %q", m.Args[0].Type)
		}
	}
}

func TestUniqueRefNameFallback(t *testing.T) {
	used := map[string]bool{"v1": true, "v10": true, "v11": true}
	// single-segment path cannot be widened at all
	if got := uniqueRefName("v1", "v1", used); got != "v12" {
		t.Errorf("uniqueRefName fallback = %q, want v12", got)
	}
	used2 := map[string]bool{"v1": true}
	if got := uniqueRefName("v1", "example.com/base/packagea/v1", used2); got != "packagea_v1" {
		t.Errorf("uniqueRefName widen = %q, want packagea_v1", got)
	}
}

func TestRewriteTypeRef(t *testing.T) {
	cases := []struct{ in, old, new, want string }{
		{"*v1.MessageA", "v1", "packagea_v1", "*packagea_v1.MessageA"},
		{"v1.MessageA", "v1", "packagea_v1", "packagea_v1.MessageA"},
		{"*v10.MessageA", "v1", "packagea_v1", "*v10.MessageA"}, // prefix must include the dot
		{"*base.Request", "v1", "packagea_v1", "*base.Request"}, // no match, untouched
	}
	for _, tc := range cases {
		if got := rewriteTypeRef(tc.in, tc.old, tc.new); got != tc.want {
			t.Errorf("rewriteTypeRef(%q,%q,%q) = %q, want %q", tc.in, tc.old, tc.new, got, tc.want)
		}
	}
}
