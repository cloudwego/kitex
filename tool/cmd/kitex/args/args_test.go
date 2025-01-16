/*
 * Copyright 2023 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package args

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestArguments_refGoSrcPath(t *testing.T) {
	gopath, _ := os.Getwd()
	t.Setenv("GOPATH", gopath)
	pkgpath := filepath.Join(gopath, "src", "github.com", "cloudwego", "kitex")
	ret, ok := refGoSrcPath(pkgpath)
	test.Assert(t, ok)
	test.Assert(t, filepath.ToSlash(ret) == "github.com/cloudwego/kitex")
}

func TestArguments_guessIDLType(t *testing.T) {
	var ok bool
	a := &Arguments{}
	a.IDL = "a.thrift"
	a.IDLType, ok = guessIDLType(a.IDL)
	test.Assert(t, ok)
	test.Assert(t, a.IsThrift())

	a.IDL = "a.proto"
	a.IDLType, ok = guessIDLType(a.IDL)
	test.Assert(t, ok)
	test.Assert(t, a.IsProtobuf())

	a.IDL = "a.txt"
	a.IDLType, ok = guessIDLType(a.IDL)
	test.Assert(t, ok == false)
	test.Assert(t, !a.IsProtobuf() && !a.IsThrift())
}
