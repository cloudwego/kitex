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
	"os"
	"os/exec"
	"strings"

	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/thriftgo"
)

func OnKitexToolNormalExit(args kargs.Arguments) {
	if args.IDLType == "thrift" {
		cmd := "go mod edit -replace github.com/apache/thrift=github.com/apache/thrift@v0.13.0"
		argv := strings.Split(cmd, " ")
		err := exec.Command(argv[0], argv[1:]...).Run()

		res := "Done"
		if err != nil {
			res = err.Error()
		}
		log.Warn("Adding apache/thrift@v0.13.0 to go.mod for generated code ..........", res)
	}

	log.Warn("Code Generation is Done!")

	// remove kitex.yaml generated from v0.4.4 which is renamed as kitex_info.yaml
	if args.ServiceName != "" {
		DeleteKitexYaml()
	}

	// If hessian option is java_extension, replace *java.Object to java.Object
	if thriftgo.EnableJavaExtension(args.Config) {
		if err := thriftgo.Hessian2PatchByReplace(args.Config, ""); err != nil {
			log.Warn("replace java object fail, you can fix it then regenerate", err)
		}
	}
}

func DeleteKitexYaml() {
	// try to read kitex.yaml
	data, err := os.ReadFile("kitex.yaml")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn("kitex.yaml, which is used to record tool info, is deprecated, it's renamed as kitex_info.yaml, you can delete it or ignore it.")
		}
		return
	}
	// if kitex.yaml exists, check content and delete it.
	if strings.HasPrefix(string(data), "kitexinfo:") {
		err = os.Remove("kitex.yaml")
		if err != nil {
			log.Warn("kitex.yaml, which is used to record tool info, is deprecated, it's renamed as kitex_info.yaml, you can delete it or ignore it.")
		} else {
			log.Warn("kitex.yaml, which is used to record tool info, is deprecated, it's renamed as kitex_info.yaml, so it's automatically deleted now.")
		}
	}
}
