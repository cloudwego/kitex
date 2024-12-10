// Copyright 2024 CloudWeGo Authors
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

package utils

import (
	"os/exec"
	"strings"

	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/thriftgo"
)

func OnKitexToolNormalExit(args kargs.Arguments) {
	log.Warn("Code Generation is Done!")

	if args.IDLType == "thrift" {
		cmd := "go mod edit -replace github.com/apache/thrift=github.com/apache/thrift@v0.13.0"
		argv := strings.Split(cmd, " ")
		err := exec.Command(argv[0], argv[1:]...).Run()
		if err != nil {
			log.Warnf("Tips: please execute the following command to fix apache thrift lib version.\n%s", cmd)
		}
	}

	// If hessian option is java_extension, replace *java.Object to java.Object
	if thriftgo.EnableJavaExtension(args.Config) {
		if err := thriftgo.Hessian2PatchByReplace(args.Config, ""); err != nil {
			log.Warn("replace java object fail, you can fix it then regenerate", err)
		}
	}
}
