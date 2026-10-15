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

package util

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestCombineOutputPath(t *testing.T) {
	ns := "aaa.bbb.ccc"
	path1 := "kitex_path/code"
	output1 := CombineOutputPath(path1, ns)
	test.Assert(t, output1 == "kitex_path/code/aaa/bbb/ccc")
	path2 := "kitex_path/{namespace}/code"
	output2 := CombineOutputPath(path2, ns)
	test.Assert(t, output2 == "kitex_path/aaa/bbb/ccc/code")
	path3 := "kitex_path/{namespaceUnderscore}/code"
	output3 := CombineOutputPath(path3, ns)
	test.Assert(t, output3 == "kitex_path/aaa_bbb_ccc/code")
}

func TestGetGOPATH(t *testing.T) {
	first := filepath.Join(string(filepath.Separator), "first", "go")
	second := filepath.Join(string(filepath.Separator), "second", "go")
	if runtime.GOOS == "windows" {
		first = `C:\first\go`
		second = `D:\second\go`
	}

	t.Setenv("GOPATH", strings.Join([]string{first, second}, string(os.PathListSeparator)))
	gopath, err := GetGOPATH()
	test.Assert(t, err == nil && gopath == first)
	t.Setenv("GOPATH", "")
	gopath, err = GetGOPATH()
	test.Assert(t, err == nil && gopath != "")
}
