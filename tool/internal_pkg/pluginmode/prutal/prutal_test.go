/*
 * Copyright 2025 CloudWeGo Authors
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

package prutal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
)

func TestPrutal(t *testing.T) {
	dir := t.TempDir()
	fn := filepath.Join(dir, "test.proto")

	os.WriteFile(fn, []byte(`option go_package = "test";
	message RequestA {
  string name = 1;
}

message ReplyA {
  string message = 1;
}

message RequestB {
  string name = 1;
}

message ReplyB {
  string message = 1;
}

message RequestC {
  string name = 1;
}

message ReplyC {
  string message = 1;
}

service ServiceA {
  rpc EchoA (RequestA) returns (ReplyA) {}
}

service ServiceB {
  rpc EchoB (stream RequestB) returns (ReplyB) {}
}

service ServiceC {
  rpc EchoC (RequestC) returns (stream ReplyC) {}
}
	`), 0o644)

	fastpbFile := filepath.Join(dir, "kitex_gen/test/test.pb.fast.go")
	err := os.MkdirAll(filepath.Dir(fastpbFile), 0o755)
	test.Assert(t, err == nil, err)
	err = os.WriteFile(fastpbFile, []byte("package test"), 0o644)
	test.Assert(t, err == nil, err)

	c := generator.Config{IDL: fn}
	c.GenerateMain = true
	c.CombineService = true
	c.PackagePrefix = "github.com/cloudwego/kitex/tool/internal_pkg/prutal/kitex_gen"
	c.OutputPath = dir
	c.GenPath = "kitex_gen"
	p := NewPrutalGen(c)
	err = p.Process()
	test.Assert(t, err == nil, err)

	b, err := os.ReadFile(filepath.Join(dir, "kitex_gen/test/test.pb.go"))
	test.Assert(t, err == nil, err)
	t.Log(string(b))

	b, err = os.ReadFile(fastpbFile)
	test.Assert(t, err == nil, err)
	test.Assert(t, strings.Contains(string(b), "deprecated"), string(b))
}
