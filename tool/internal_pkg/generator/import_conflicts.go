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
	"fmt"
	"go/token"
	"sort"
	"strings"
	"unicode"
)

// FixImportConflicts rewrites the PkgRefName aliases of every PkgInfo
// reachable from ss so that distinct import paths never share one alias.
//
// The protoc and prutal plugins derive aliases from the last segment of the
// import path (path.Base), so any two dependencies whose import paths end
// with the same directory name (e.g. base/packagea/v1 vs base/packageb/v1)
// used to get identical aliases. The generated import block then declared
// the same alias twice and the code did not compile (kitex#1892).
//
// Renames are deterministic: within one colliding group the service's own
// package keeps the original alias (or the lexicographically smallest import
// path when no service package is involved), and the others are widened with
// parent path segments only as much as needed to become unique. Type strings
// that embed a renamed alias are rewritten to match.
func FixImportConflicts(ss []*ServiceInfo) {
	var params []*Parameter
	var pis []*PkgInfo

	collectParam := func(p *Parameter) {
		if p == nil {
			return
		}
		params = append(params, p)
		for i := range p.Deps {
			pis = append(pis, &p.Deps[i])
		}
	}

	seen := make(map[*ServiceInfo]bool)
	svcPaths := make(map[string]bool)
	var walk func(si *ServiceInfo)
	walk = func(si *ServiceInfo) {
		if si == nil || seen[si] {
			return
		}
		seen[si] = true
		pis = append(pis, &si.PkgInfo)
		if si.PkgInfo.ImportPath != "" {
			svcPaths[si.PkgInfo.ImportPath] = true
		}
		for _, m := range si.Methods {
			if m == nil {
				continue
			}
			pis = append(pis, &m.PkgInfo)
			for _, a := range m.Args {
				collectParam(a)
			}
			collectParam(m.Resp)
			for _, e := range m.Exceptions {
				collectParam(e)
			}
		}
		for _, c := range si.CombineServices {
			walk(c)
		}
		if si.Base != nil {
			walk(si.Base)
		}
	}
	for _, si := range ss {
		walk(si)
	}

	// group distinct import paths by alias
	type ref struct{ alias, path string }
	byAlias := make(map[string]map[string]bool)
	for _, pi := range pis {
		if pi.PkgRefName == "" || pi.ImportPath == "" || pi.PkgRefName == pi.ImportPath {
			continue
		}
		if byAlias[pi.PkgRefName] == nil {
			byAlias[pi.PkgRefName] = make(map[string]bool)
		}
		byAlias[pi.PkgRefName][pi.ImportPath] = true
	}

	// all currently used aliases are off-limits for renamed ones, even if
	// their own group does not collide
	used := make(map[string]bool)
	var sortedAliases []string
	for alias := range byAlias {
		used[alias] = true
		sortedAliases = append(sortedAliases, alias)
	}
	sort.Strings(sortedAliases)

	renames := make(map[ref]string)
	for _, alias := range sortedAliases {
		paths := byAlias[alias]
		if len(paths) < 2 {
			continue
		}
		sortedPaths := make([]string, 0, len(paths))
		for p := range paths {
			sortedPaths = append(sortedPaths, p)
		}
		sort.Strings(sortedPaths)
		// the service's own package keeps the original alias (it is the
		// most referenced one in the generated code); otherwise the
		// lexicographically smallest path keeps it, to minimize churn
		keeper := sortedPaths[0]
		for _, p := range sortedPaths {
			if svcPaths[p] {
				keeper = p
				break
			}
		}
		for _, p := range sortedPaths {
			if p == keeper {
				continue
			}
			na := uniqueRefName(alias, p, used)
			renames[ref{alias, p}] = na
			used[na] = true
		}
	}
	if len(renames) == 0 {
		return
	}

	rewrite := func(pi *PkgInfo) (oldRef, newRef string, ok bool) {
		na, found := renames[ref{pi.PkgRefName, pi.ImportPath}]
		if !found {
			return "", "", false
		}
		oldRef = pi.PkgRefName
		pi.PkgRefName = na
		return oldRef, na, true
	}

	// deps first: their alias is embedded in the parameter Type string
	for _, p := range params {
		for i := range p.Deps {
			if oldRef, newRef, ok := rewrite(&p.Deps[i]); ok {
				p.Type = rewriteTypeRef(p.Type, oldRef, newRef)
			}
		}
	}
	// service/method PkgInfos (dep entries above no longer match renames)
	for _, pi := range pis {
		rewrite(pi)
	}
}

// uniqueRefName derives an alias for importPath that is not present in used.
// It widens the original alias with parent path segments (packagea_v1,
// base_packagea_v1, ...) and falls back to a numeric suffix as a last resort.
func uniqueRefName(alias, importPath string, used map[string]bool) string {
	segs := strings.Split(strings.Trim(importPath, `"`), "/")
	for width := 2; width <= len(segs); width++ {
		cand := sanitizeRefName(strings.Join(segs[len(segs)-width:], "_"))
		if cand != alias && !used[cand] {
			return cand
		}
	}
	for i := 0; ; i++ {
		cand := fmt.Sprintf("%s%d", alias, i)
		if !used[cand] {
			return cand
		}
	}
}

// rewriteTypeRef replaces a leading alias reference inside a generated type
// string, e.g. "*v1.MessageA" -> "*packagea_v1.MessageA".
func rewriteTypeRef(typ, oldRef, newRef string) string {
	if oldRef == "" || oldRef == newRef {
		return typ
	}
	if rest, ok := strings.CutPrefix(typ, "*"+oldRef+"."); ok {
		return "*" + newRef + "." + rest
	}
	if rest, ok := strings.CutPrefix(typ, oldRef+"."); ok {
		return newRef + "." + rest
	}
	return typ
}

// sanitizeRefName keeps s a valid Go identifier, mirroring the sanitizing
// the protoc plugin applies to path.Base derived aliases.
func sanitizeRefName(s string) string {
	s = strings.Map(func(r rune) rune {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return '_'
		}
		return r
	}, s)
	if !token.IsIdentifier(s) {
		return "_" + s
	}
	return s
}
