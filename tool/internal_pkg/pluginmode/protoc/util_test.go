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

package protoc

import "testing"

func TestImportPathToPkgRef(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"kitexidltest/kitex_gen/base/packagea/v1", "packagea_v1"},
		{"kitexidltest/kitex_gen/base/packageb/v1", "packageb_v1"},
		{"kitexidltest/kitex_gen/example/v1", "example_v1"},
		{"github.com/cloudwego/kitex/client", "client"},
		{"context", "context"},
	}
	for _, tc := range cases {
		if got := importPathToPkgRef(tc.in); got != tc.want {
			t.Errorf("importPathToPkgRef(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
	// uniqueness for the issue case
	a := importPathToPkgRef("kitexidltest/kitex_gen/base/packagea/v1")
	b := importPathToPkgRef("kitexidltest/kitex_gen/base/packageb/v1")
	e := importPathToPkgRef("kitexidltest/kitex_gen/example/v1")
	if a == b || a == e || b == e {
		t.Fatalf("aliases not unique: %q %q %q", a, b, e)
	}
}
