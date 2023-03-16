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
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/cloudwego/kitex"
	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/protoc"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/thriftgo"
)

var args kargs.Arguments

func init() {
	var queryVersion bool
	args.AddExtraFlag(&kargs.ExtraFlag{
		Apply: func(f *flag.FlagSet) {
			f.BoolVar(&queryVersion, "version", false,
				"Show the version of kitex")
		},
		Check: func(a *kargs.Arguments) {
			if queryVersion {
				println(a.Version)
				os.Exit(0)
			}
		},
	})
}

func main() {
	mode := os.Getenv(kargs.EnvPluginMode)
	if len(os.Args) <= 1 && mode != "" {
		// run as a plugin
		switch mode {
		case thriftgo.PluginName:
			os.Exit(thriftgo.Run())
		case protoc.PluginName:
			os.Exit(protoc.Run())
		}
	}

	// run as kitex
	args.ParseArgs(kitex.Version)

	out := new(bytes.Buffer)
	cmd := args.BuildCmd(out)
	err := cmd.Run()
	if err != nil {
		if args.Use != "" {
			out := strings.TrimSpace(out.String())
			if strings.HasSuffix(out, thriftgo.TheUseOptionMessage) {
				goto NormalExit
			}
		}
		os.Exit(1)
	}
NormalExit:
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

	// remove kitex.yaml generated from v0.4.4 which is renamed as kitex_info.yaml
	if args.ServiceName != "" {
		DeleteKitexYaml()
	}
}

func DeleteKitexYaml() {
	// try to read kitex.yaml
	data, err := ioutil.ReadFile("kitex.yaml")
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
