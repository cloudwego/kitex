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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudwego/kitex"
	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

// extraFlag is designed for adding flags that is irrelevant to
// code generation.
type extraFlag struct {
	// apply may add flags to the FlagSet.
	apply func(*flag.FlagSet)

	// check may perform any value checking for flags added by apply above.
	// When an error occur, check should directly terminate the program by
	// os.Exit with exit code 1 for internal error and 2 for invalid arguments.
	check func(*arguments)
}

type arguments struct {
	generator.Config
	extends []*extraFlag
}

func (a *arguments) addExtraFlag(e *extraFlag) {
	a.extends = append(a.extends, e)
}

func (a *arguments) checkExtension(ext string) {
	if !strings.HasSuffix(a.IDL, ext) {
		fmt.Fprintf(os.Stderr, "Expect the IDL file to have an extension of '%s'. Got '%s'.", ext, a.IDL)
		os.Exit(2)
	}
}

func (a *arguments) guessIDLType() (string, bool) {
	switch {
	case strings.HasSuffix(a.IDL, ".thrift"):
		return "thrift", true
	case strings.HasSuffix(a.IDL, ".proto"):
		return "protobuf", true
	}
	return "unknown", false
}

func (a *arguments) buildFlags() *flag.FlagSet {
	f := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	f.BoolVar(&a.NoFastAPI, "no-fast-api", false,
		"Generate codes without injecting fast method.")
	f.StringVar(&a.ModuleName, "module", "",
		"Specify the Go module name to generate go.mod.")
	f.StringVar(&a.ServiceName, "service", "",
		"Specify the service name to generate server side codes.")
	f.StringVar(&a.Use, "use", "",
		"Specify the kitex_gen package to import when generate server side codes.")
	f.BoolVar(&a.Verbose, "v", false, "") // short for -verbose
	f.BoolVar(&a.Verbose, "verbose", false,
		"Turn on verbose mode.")
	f.BoolVar(&a.GenerateInvoker, "invoker", false,
		"Generate invoker side codes when service name is specified.")
	f.StringVar(&a.IDLType, "type", "unknown", "Specify the type of IDL: 'thrift' or 'protobuf'.")
	f.Var(&a.Includes, "I", "Add an IDL search path for includes.")
	f.Var(&a.ThriftOptions, "thrift", "Specify arguments for the thrift compiler.")
	f.Var(&a.ThriftPlugins, "thrift-plugin", "Specify thrift plugin arguments for the thrift compiler.")
	f.Var(&a.ProtobufOptions, "protobuf", "Specify arguments for the protobuf compiler.")
	f.BoolVar(&a.CombineService, "combine-service", false,
		"Combine services in root thrift file.")
	f.BoolVar(&a.CopyIDL, "copy-idl", false,
		"Copy each IDL file to the output path.")

	a.Version = kitex.Version
	a.ThriftOptions = append(a.ThriftOptions,
		"naming_style=golint",
		"ignore_initialisms",
		"gen_setter",
		"gen_deep_equal",
		"compatible_names",
	)

	for _, e := range a.extends {
		e.apply(f)
	}
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: %s [flags] IDL

flags:
`, kitex.Version, os.Args[0])
		f.PrintDefaults()
		os.Exit(1)
	}
	return f
}

func (a *arguments) parseArgs() {
	f := a.buildFlags()
	if err := f.Parse(os.Args[1:]); err != nil {
		log.Warn(os.Stderr, err)
		os.Exit(2)
	}

	log.Verbose = a.Verbose

	for _, e := range a.extends {
		e.check(a)
	}

	a.checkIDL(f.Args())
	a.checkServiceName()
	a.checkPath()
}

func (a *arguments) checkIDL(files []string) {
	if len(files) == 0 {
		log.Warn("No IDL file found.")
		os.Exit(2)
	}
	if len(files) != 1 {
		log.Warn("Require exactly 1 IDL file.")
		os.Exit(2)
	}
	a.IDL = files[0]

	switch a.IDLType {
	case "thrift":
		a.checkExtension(".thrift")
	case "protobuf":
		a.checkExtension(".proto")
	case "unknown":
		if typ, ok := a.guessIDLType(); ok {
			a.IDLType = typ
		} else {
			log.Warn("Can not guess an IDL type from %q, please specify with the '-type' flag.", a.IDL)
			os.Exit(2)
		}
	default:
		log.Warn("Unsupported IDL type:", a.IDLType)
		os.Exit(2)
	}
}

func (a *arguments) checkServiceName() {
	if a.ServiceName == "" {
		if a.Use != "" {
			log.Warn("-use must be used with -service")
			os.Exit(2)
		}
	} else {
		a.GenerateMain = true
	}
}

func (a *arguments) checkPath() {
	pathToGo, err := exec.LookPath("go")
	if err != nil {
		log.Warn(err)
		os.Exit(1)
	}

	gosrc := filepath.Join(util.GetGOPATH(), "src")
	gosrc, err = filepath.Abs(gosrc)
	if err != nil {
		log.Warn("Get GOPATH/src path failed:", err.Error())
		os.Exit(1)
	}
	curpath, err := filepath.Abs(".")
	if err != nil {
		log.Warn("Get current path failed:", err.Error())
		os.Exit(1)
	}

	if strings.HasPrefix(curpath, gosrc) {
		if a.PackagePrefix, err = filepath.Rel(gosrc, curpath); err != nil {
			log.Warn("Get GOPATH/src relpath failed:", err.Error())
			os.Exit(1)
		}
		a.PackagePrefix = filepath.Join(a.PackagePrefix, generator.KitexGenPath)
	} else {
		if a.ModuleName == "" {
			log.Warn("Outside of $GOPATH. Please specify a module name with the '-module' flag.")
			os.Exit(1)
		}
	}

	if a.ModuleName != "" {
		module, path, ok := util.SearchGoMod(curpath)
		if ok {
			// go.mod exists
			if module != a.ModuleName {
				log.Warnf("The module name given by the '-module' option ('%s') is not consist with the name defined in go.mod ('%s' from %s)\n",
					a.ModuleName, module, path)
				os.Exit(1)
			}
			if a.PackagePrefix, err = filepath.Rel(path, curpath); err != nil {
				log.Warn("Get package prefix failed:", err.Error())
				os.Exit(1)
			}
			a.PackagePrefix = filepath.Join(a.ModuleName, a.PackagePrefix, generator.KitexGenPath)
		} else {
			if err = initGoMod(pathToGo, a.ModuleName); err != nil {
				log.Warn("Init go mod failed:", err.Error())
				os.Exit(1)
			}
			a.PackagePrefix = filepath.Join(a.ModuleName, generator.KitexGenPath)
		}
	}

	if a.Use != "" {
		a.PackagePrefix = a.Use
	}
	a.OutputPath = curpath
}

func initGoMod(pathToGo, module string) error {
	if util.Exists("go.mod") {
		return nil
	}

	cmd := &exec.Cmd{
		Path:   pathToGo,
		Args:   []string{"go", "mod", "init", module},
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return cmd.Run()
}
