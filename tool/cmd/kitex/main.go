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
	"path/filepath"
	"strings"

	"github.com/cloudwego/kitex"
	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/cmd/kitex/versions"
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
		Check: func(a *kargs.Arguments) error {
			if queryVersion {
				println(a.Version)
				os.Exit(0)
			}
			return nil
		},
	})
	if err := versions.RegisterMinDepVersion(
		&versions.MinDepVersion{
			RefPath: "github.com/cloudwego/kitex",
			Version: "v0.11.0",
		},
	); err != nil {
		log.Warn(err)
		os.Exit(versions.CompatibilityCheckExitCode)
	}
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

	curpath, err := filepath.Abs(".")
	if err != nil {
		log.Warn("Get current path failed:", err.Error())
		os.Exit(1)
	}
	// run as kitex
	err = args.ParseArgs(kitex.Version, curpath, os.Args[1:])
	if err != nil {
		log.Warn(err.Error())
		os.Exit(2)
	}
	if !args.NoDependencyCheck {
		// check dependency compatibility between kitex cmd tool and dependency in go.mod
		if err := versions.DefaultCheckDependencyAndProcess(); err != nil {
			os.Exit(versions.CompatibilityCheckExitCode)
		}
	}

	out := new(bytes.Buffer)
	cmd, err := args.BuildCmd(out)
	if err != nil {
		log.Warn(err)
		os.Exit(1)
	}
	err = kargs.ValidateCMD(cmd.Path, args.IDLType)
	if err != nil {
		log.Warn(err)
		os.Exit(1)
	}
	err = cmd.Run()
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

	// If hessian option is java_extension, replace *java.Object to java.Object
	if thriftgo.EnableJavaExtension(args.Config) {
		if err = thriftgo.Hessian2PatchByReplace(args.Config, ""); err != nil {
			log.Warn("replace java object fail, you can fix it then regenerate", err)
		}
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
