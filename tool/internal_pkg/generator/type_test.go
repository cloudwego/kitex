// Copyright 2022 CloudWeGo Authors
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
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestSetTemplateExtension(t *testing.T) {
	n := "nonsense"
	SetTemplateExtension(n, "whatever")

	_, set := templateExtensions[n]
	test.Assert(t, !set)

	a := templateNames[0]
	b := "hello world"
	SetTemplateExtension(a, b)

	c, set := templateExtensions[a]
	test.Assert(t, set && c == b)
}

func TestAddTemplateFunc(t *testing.T) {
	key := "demo"
	f := func() {}
	AddTemplateFunc(key, f)
	_, ok := funcs[key]
	test.Assert(t, ok)
}

func TestServiceInfo_HasStreamingRecursive(t *testing.T) {
	t.Run("has-streaming", func(t *testing.T) {
		s := &ServiceInfo{HasStreaming: true}
		test.Assert(t, s.HasStreamingRecursive())
	})
	t.Run("no-streaming-no-base", func(t *testing.T) {
		s := &ServiceInfo{HasStreaming: false}
		test.Assert(t, !s.HasStreamingRecursive())
	})
	t.Run("no-streaming-with-base-no-streaming", func(t *testing.T) {
		s := &ServiceInfo{HasStreaming: false, Base: &ServiceInfo{HasStreaming: false}}
		test.Assert(t, !s.HasStreamingRecursive())
	})
	t.Run("no-streaming-with-base-has-streaming", func(t *testing.T) {
		s := &ServiceInfo{HasStreaming: false, Base: &ServiceInfo{HasStreaming: true}}
		test.Assert(t, s.HasStreamingRecursive())
	})
	t.Run("no-streaming-with-base-with-base-has-streaming", func(t *testing.T) {
		s := &ServiceInfo{
			HasStreaming: false,
			Base: &ServiceInfo{
				HasStreaming: false,
				Base:         &ServiceInfo{HasStreaming: true},
			},
		}
		test.Assert(t, s.HasStreamingRecursive())
	})
}

func TestServiceInfo_FixHasStreamingForExtendedService(t *testing.T) {
	t.Run("has-streaming", func(t *testing.T) {
		s := &ServiceInfo{HasStreaming: true}
		s.FixHasStreamingForExtendedService()
		test.Assert(t, s.HasStreaming)
	})
	t.Run("no-streaming-no-base", func(t *testing.T) {
		s := &ServiceInfo{HasStreaming: false}
		s.FixHasStreamingForExtendedService()
		test.Assert(t, !s.HasStreaming)
	})
	t.Run("no-streaming-with-base-no-streaming", func(t *testing.T) {
		s := &ServiceInfo{HasStreaming: false, Base: &ServiceInfo{HasStreaming: false}}
		s.FixHasStreamingForExtendedService()
		test.Assert(t, !s.HasStreaming)
	})
	t.Run("no-streaming-with-base-has-streaming", func(t *testing.T) {
		s := &ServiceInfo{HasStreaming: false, Base: &ServiceInfo{HasStreaming: true}}
		s.FixHasStreamingForExtendedService()
		test.Assert(t, s.HasStreaming)
	})
	t.Run("no-streaming-with-base-with-base-has-streaming", func(t *testing.T) {
		s := &ServiceInfo{
			HasStreaming: false,
			Base: &ServiceInfo{
				HasStreaming: false,
				Base: &ServiceInfo{
					HasStreaming: true,
				},
			},
		}
		s.FixHasStreamingForExtendedService()
		test.Assert(t, s.HasStreaming)
		test.Assert(t, s.Base.HasStreaming)
	})
}

func TestPkgInfo_UpdateImport(t *testing.T) {
	testcases := []struct {
		desc    string
		imports map[string]map[string]bool
		pkg     string
		newPath string
		expect  func(t *testing.T, pkgInfo *PackageInfo, pkg, newPath string)
	}{
		{
			desc: "update path for same alias",
			imports: map[string]map[string]bool{
				"path/to/server": {
					"server": true,
				},
			},
			pkg:     "server",
			newPath: "new/path/to/server",
			expect: func(t *testing.T, pkgInfo *PackageInfo, pkg, newPath string) {
				pkgSet := pkgInfo.Imports[newPath]
				test.Assert(t, pkgSet != nil)
				test.Assert(t, pkgSet[pkg])
			},
		},
		{
			desc: "remove alias",
			imports: map[string]map[string]bool{
				"path/to/context": {
					"context": true,
				},
			},
			pkg:     "context",
			newPath: "context",
			expect: func(t *testing.T, pkgInfo *PackageInfo, pkg, newPath string) {
				pkgSet := pkgInfo.Imports[newPath]
				test.Assert(t, pkgSet == nil)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			pkgInfo := &PackageInfo{
				Imports: tc.imports,
			}
			pkgInfo.UpdateImport(tc.pkg, tc.newPath)
			tc.expect(t, pkgInfo, tc.pkg, tc.newPath)
		})
	}
}
