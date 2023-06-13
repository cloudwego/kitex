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

package args

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/protoc"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/thriftgo"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

// EnvPluginMode is an environment that kitex uses to distinguish run modes.
const EnvPluginMode = "KITEX_PLUGIN_MODE"

// ExtraFlag is designed for adding flags that is irrelevant to
// code generation.
type ExtraFlag struct {
	// apply may add flags to the FlagSet.
	Apply func(*flag.FlagSet)

	// check may perform any value checking for flags added by apply above.
	// When an error occur, check should directly terminate the program by
	// os.Exit with exit code 1 for internal error and 2 for invalid arguments.
	Check func(*Arguments)
}

// Arguments .
type Arguments struct {
	generator.Config
	extends []*ExtraFlag
}

const cmdExample = `  # Generate client codes or update kitex_gen codes when a project is in $GOPATH:
  kitex {{path/to/IDL_file.thrift}}

  # Generate client codes or update kitex_gen codes  when a project is not in $GOPATH:
  kitex -module {{github.com/xxx_org/xxx_name}} {{path/to/IDL_file.thrift}}

  # Generate server codes:
  kitex -service {{svc_name}} {{path/to/IDL_file.thrift}}
`

// AddExtraFlag .
func (a *Arguments) AddExtraFlag(e *ExtraFlag) {
	a.extends = append(a.extends, e)
}

func (a *Arguments) guessIDLType() (string, bool) {
	switch {
	case strings.HasSuffix(a.IDL, ".thrift"):
		return "thrift", true
	case strings.HasSuffix(a.IDL, ".proto"):
		return "protobuf", true
	}
	return "unknown", false
}

func (a *Arguments) buildFlags(version string) *flag.FlagSet {
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
	f.Var(&a.ThriftOptions, "thrift", "Specify arguments for the thrift go compiler.")
	f.DurationVar(&a.ThriftPluginTimeLimit, "thrift-plugin-time-limit", generator.DefaultThriftPluginTimeLimit, "Specify thrift plugin execution time limit.")
	f.Var(&a.ThriftPlugins, "thrift-plugin", "Specify thrift plugin arguments for the thrift compiler.")
	f.Var(&a.ProtobufOptions, "protobuf", "Specify arguments for the protobuf compiler.")
	f.Var(&a.ProtobufPlugins, "protobuf-plugin", "Specify protobuf plugin arguments for the protobuf compiler.(plugin_name:options:out_dir)")
	f.BoolVar(&a.CombineService, "combine-service", false,
		"Combine services in root thrift file.")
	f.BoolVar(&a.CopyIDL, "copy-idl", false,
		"Copy each IDL file to the output path.")
	f.StringVar(&a.ExtensionFile, "template-extension", a.ExtensionFile,
		"Specify a file for template extension.")
	f.BoolVar(&a.FrugalPretouch, "frugal-pretouch", false,
		"Use frugal to compile arguments and results when new clients and servers.")
	f.BoolVar(&a.Record, "record", false,
		"Record Kitex cmd into kitex-all.sh.")
	f.StringVar(&a.TemplateDir, "template-dir", "",
		"Use custom template to generate codes.")
	f.StringVar(&a.GenPath, "gen-path", generator.KitexGenPath,
		"Specify a code gen path.")
	f.BoolVar(&a.DeepCopyAPI, "deep-copy-api", false,
		"Generate codes with injecting deep copy method.")
	a.RecordCmd = os.Args
	a.Version = version
	a.ThriftOptions = append(a.ThriftOptions,
		"naming_style=golint",
		"ignore_initialisms",
		"gen_setter",
		"gen_deep_equal",
		"compatible_names",
		"frugal_tag",
	)

	for _, e := range a.extends {
		e.Apply(f)
	}
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: %s [flags] IDL

Examples:
%s
Flags:
`, a.Version, os.Args[0], cmdExample)
		f.PrintDefaults()
		os.Exit(1)
	}
	return f
}

// ParseArgs parses command line arguments.
func (a *Arguments) ParseArgs(version string) {
	f := a.buildFlags(version)
	if err := f.Parse(os.Args[1:]); err != nil {
		log.Warn(os.Stderr, err)
		os.Exit(2)
	}

	log.Verbose = a.Verbose

	for _, e := range a.extends {
		e.Check(a)
	}

	a.checkIDL(f.Args())
	a.checkServiceName()
	// todo finish protobuf
	if a.IDLType != "thrift" {
		a.GenPath = generator.KitexGenPath
	}
	a.checkPath()
}

func (a *Arguments) checkIDL(files []string) {
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
	case "thrift", "protobuf":
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

func (a *Arguments) checkServiceName() {
	if a.ServiceName == "" && a.TemplateDir == "" {
		if a.Use != "" {
			log.Warn("-use must be used with -service or -template-dir")
			os.Exit(2)
		}
	}
	if a.ServiceName != "" && a.TemplateDir != "" {
		log.Warn("-template-dir and -service cannot be specified at the same time")
		os.Exit(2)
	}
	if a.ServiceName != "" {
		a.GenerateMain = true
	}
}

func (a *Arguments) checkPath() {
	pathToGo, err := exec.LookPath("go")
	if err != nil {
		log.Warn(err)
		os.Exit(1)
	}

	gosrc := util.JoinPath(util.GetGOPATH(), "src")
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
		a.PackagePrefix = util.JoinPath(a.PackagePrefix, a.GenPath)
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
			a.PackagePrefix = util.JoinPath(a.ModuleName, a.PackagePrefix, a.GenPath)
		} else {
			if err = initGoMod(pathToGo, a.ModuleName); err != nil {
				log.Warn("Init go mod failed:", err.Error())
				os.Exit(1)
			}
			a.PackagePrefix = util.JoinPath(a.ModuleName, a.GenPath)
		}
	}

	if a.Use != "" {
		a.PackagePrefix = a.Use
	}
	a.OutputPath = curpath
}

// BuildCmd builds an exec.Cmd.
func (a *Arguments) BuildCmd(out io.Writer) *exec.Cmd {
	exe, err := os.Executable()
	if err != nil {
		log.Warn("Failed to detect current executable:", err.Error())
		os.Exit(1)
	}

	for i, inc := range a.Includes {
		if strings.HasPrefix(inc, "git@") || strings.HasPrefix(inc, "http://") || strings.HasPrefix(inc, "https://") {
			localGitPath, errMsg, gitErr := util.RunGitCommand(inc)
			if gitErr != nil {
				if errMsg == "" {
					errMsg = gitErr.Error()
				}
				log.Warn("Failed to pull IDL from git:", errMsg)
				os.Exit(1)
			}
			a.Includes[i] = localGitPath
		}
	}

	kas := strings.Join(a.Config.Pack(), ",")
	cmd := &exec.Cmd{
		Path:   lookupTool(a.IDLType),
		Stdin:  os.Stdin,
		Stdout: io.MultiWriter(out, os.Stdout),
		Stderr: io.MultiWriter(out, os.Stderr),
	}
	if a.IDLType == "thrift" {
		os.Setenv(EnvPluginMode, thriftgo.PluginName)
		cmd.Args = append(cmd.Args, "thriftgo")
		for _, inc := range a.Includes {
			cmd.Args = append(cmd.Args, "-i", inc)
		}
		a.ThriftOptions = append(a.ThriftOptions, "package_prefix="+a.PackagePrefix)
		gas := "go:" + strings.Join(a.ThriftOptions, ",")
		if a.Verbose {
			cmd.Args = append(cmd.Args, "-v")
		}
		if a.Use == "" {
			cmd.Args = append(cmd.Args, "-r")
		}
		cmd.Args = append(cmd.Args,
			"-o", a.GenPath,
			"-g", gas,
			"-p", "kitex="+exe+":"+kas,
		)
		if a.ThriftPluginTimeLimit != generator.DefaultThriftPluginTimeLimit {
			cmd.Args = append(cmd.Args,
				"--plugin-time-limit", a.ThriftPluginTimeLimit.String(),
			)
		}
		for _, p := range a.ThriftPlugins {
			cmd.Args = append(cmd.Args, "-p", p)
		}
		cmd.Args = append(cmd.Args, a.IDL)
	} else {
		os.Setenv(EnvPluginMode, protoc.PluginName)
		a.ThriftOptions = a.ThriftOptions[:0]
		// "protobuf"
		cmd.Args = append(cmd.Args, "protoc")
		for _, inc := range a.Includes {
			cmd.Args = append(cmd.Args, "-I", inc)
		}
		outPath := util.JoinPath(".", a.GenPath)
		if a.Use == "" {
			os.MkdirAll(outPath, 0o755)
		} else {
			outPath = "."
		}
		cmd.Args = append(cmd.Args,
			"--plugin=protoc-gen-kitex="+exe,
			"--kitex_out="+outPath,
			"--kitex_opt="+kas,
		)
		for _, po := range a.ProtobufOptions {
			cmd.Args = append(cmd.Args, "--kitex_opt="+po)
		}
		for _, p := range a.ProtobufPlugins {
			pluginParams := strings.Split(p, ":")
			if len(pluginParams) != 3 {
				log.Warnf("Failed to get the correct protoc plugin parameters for %. Please specify the protoc plugin in the form of \"plugin_name:options:out_dir\"", p)
				os.Exit(1)
			}
			// pluginParams[0] -> plugin name, pluginParams[1] -> plugin options, pluginParams[2] -> out_dir
			cmd.Args = append(cmd.Args,
				fmt.Sprintf("--%s_out=%s", pluginParams[0], pluginParams[2]),
				fmt.Sprintf("--%s_opt=%s", pluginParams[0], pluginParams[1]),
			)
		}

		cmd.Args = append(cmd.Args, a.IDL)
	}
	log.Info(strings.ReplaceAll(strings.Join(cmd.Args, " "), kas, fmt.Sprintf("%q", kas)))
	return cmd
}

func lookupTool(idlType string) string {
	tool := "thriftgo"
	if idlType == "protobuf" {
		tool = "protoc"
	}

	path, err := exec.LookPath(tool)
	if err != nil {
		log.Warnf("Failed to find %q from $PATH: %s. Try $GOPATH/bin/%s instead\n", path, err.Error(), tool)
		path = util.JoinPath(util.GetGOPATH(), "bin", tool)
	}
	return path
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
