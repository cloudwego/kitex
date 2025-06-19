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

package main

import (
	"os"
	"path/filepath"

	"github.com/cloudwego/kitex/tool/cmd/kitex/code"

	"github.com/cloudwego/kitex/tool/cmd/kitex/utils"

	"github.com/cloudwego/kitex"
	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/cmd/kitex/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
)

func main() {
	var args kargs.Arguments

	curpath, err := filepath.Abs(".")
	if err != nil {
		log.Errorf("get current path failed: %s", err.Error())
		os.Exit(int(code.ToolExecuteFailed))
	}

	// run kitex tool:
	// 1. check and execute process plugin mode callback
	// 2. parse user input to arguments
	// 3. check dependency version ( details: versions/dependencies_v2.go)
	// 4. execute code generation by choosing specific tool (thriftgo、prutal、protoc?)
	err, retCode := generator.RunKitexTool(curpath, os.Args, &args, kitex.Version)
	if err != nil || !code.Good(retCode) {
		log.Errorf(err.Error())
		os.Exit(int(retCode))
	}
	// only execute after tool finishes code generation (not -help、-version)
	if retCode != code.ToolInterrupted {
		utils.OnKitexToolNormalExit(args)
	}
	os.Exit(int(code.ToolSuccess))
}
