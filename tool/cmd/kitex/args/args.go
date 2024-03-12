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
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
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
	f.Var(&a.Hessian2Options, "hessian2", "Specify arguments for the hessian2 codec.")
	f.DurationVar(&a.ThriftPluginTimeLimit, "thrift-plugin-time-limit", generator.DefaultThriftPluginTimeLimit, "Specify thrift plugin execution time limit.")
	f.StringVar(&a.CompilerPath, "compiler-path", "", "Specify the path of thriftgo/protoc.")
	f.Var(&a.ThriftPlugins, "thrift-plugin", "Specify thrift plugin arguments for the thrift compiler.")
	f.Var(&a.ProtobufOptions, "protobuf", "Specify arguments for the protobuf compiler.")
	f.Var(&a.ProtobufPlugins, "protobuf-plugin", "Specify protobuf plugin arguments for the protobuf compiler.(plugin_name:options:out_dir)")
	f.BoolVar(&a.CombineService, "combine-service", false,
		"Combine services in root thrift file.")
	f.BoolVar(&a.CopyIDL, "copy-idl", false,
		"Copy each IDL file to the output path.")
	f.BoolVar(&a.HandlerReturnKeepResp, "handler-return-keep-resp", false,
		"When the server-side handler returns both err and resp, the resp return is retained for use in middleware where both err and resp can be used simultaneously. Note: At the RPC communication level, if the handler returns an err, the framework still only returns err to the client without resp.")
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
	f.StringVar(&a.Protocol, "protocol", "",
		"Specify a protocol for codec")
	a.RecordCmd = os.Args
	a.Version = version
	a.ThriftOptions = append(a.ThriftOptions,
		"naming_style=golint",
		"ignore_initialisms",
		"gen_setter",
		"gen_deep_equal",
		"compatible_names",
		"frugal_tag",
		"thrift_streaming",
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
		log.Warn("No IDL file found; Please make your IDL file the last parameter, for example: " +
			"\"kitex -service demo idl.thrift\".")
		os.Exit(2)
	}
	if len(files) != 1 {
		log.Warn("Require exactly 1 IDL file; Please make your IDL file the last parameter, for example: " +
			"\"kitex -service demo idl.thrift\".")
		os.Exit(2)
	}
	a.IDL = files[0]

	switch a.IDLType {
	case "thrift", "protobuf":
	case "unknown":
		if typ, ok := a.guessIDLType(); ok {
			a.IDLType = typ
		} else {
			log.Warnf("Can not guess an IDL type from %q (unknown suffix), please specify with the '-type' flag.", a.IDL)
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
				log.Warn("failed to pull IDL from git:", errMsg)
				log.Warn("You can execute 'rm -rf ~/.kitex' to clean the git cache and try again.")
				os.Exit(1)
			}
			a.Includes[i] = localGitPath
		}
	}

	kas := strings.Join(a.Config.Pack(), ",")
	cmd := &exec.Cmd{
		Path:   LookupTool(a.IDLType, a.CompilerPath),
		Stdin:  os.Stdin,
		Stdout: io.MultiWriter(out, os.Stdout),
		Stderr: io.MultiWriter(out, os.Stderr),
	}

	ValidateCMD(cmd.Path, a.IDLType)

	if a.IDLType == "thrift" {
		os.Setenv(EnvPluginMode, thriftgo.PluginName)
		cmd.Args = append(cmd.Args, "thriftgo")
		for _, inc := range a.Includes {
			cmd.Args = append(cmd.Args, "-i", inc)
		}
		if thriftgo.IsHessian2(a.Config) {
			thriftgo.Hessian2PreHook(&a.Config)
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

// ValidateCMD check if the path exists and if the version is satisfied
func ValidateCMD(path, idlType string) {
	// check if the path exists
	if _, err := os.Stat(path); err != nil {
		tool := "thriftgo"
		if idlType == "protobuf" {
			tool = "protoc"
		}

		if idlType == "thrift" {
			_, err = runCommand("go install github.com/cloudwego/thriftgo@latest")
			if err != nil {
				log.Warnf("[ERROR] %s is also unavailable, please install %s first.\n", path, tool)
				log.Warn("Refer to https://github.com/cloudwego/thriftgo, or simple run:\n")
				log.Warn("  go install -v github.com/cloudwego/thriftgo@latest\n")
				os.Exit(1)
			}
		} else {
			log.Warnf("[ERROR] %s is also unavailable, please install %s first.\n", path, tool)
			log.Warn("Refer to https://github.com/protocolbuffers/protobuf\n")
			os.Exit(1)
		}
	}

	// check if the version is satisfied
	if idlType == "thrift" {
		// run `thriftgo -versions and get the output
		cmd := exec.Command(path, "-version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Warnf("Failed to get thriftgo version: %s\n", err.Error())
			os.Exit(1)
		}
		if !strings.HasPrefix(string(out), "thriftgo ") {
			log.Warnf("thriftgo -version returns '%s', please reinstall thriftgo first.\n", string(out))
			os.Exit(1)
		}
		version := strings.Replace(strings.TrimSuffix(string(out), "\n"), "thriftgo ", "", 1)
		if !versionSatisfied(version, requiredThriftGoVersion) {
			_, err = runCommand("go install github.com/cloudwego/thriftgo@latest")
			if err != nil {
				log.Warnf("[ERROR] thriftgo version(%s) not satisfied, please install version >= %s\n",
					version, requiredThriftGoVersion)
				os.Exit(1)
			}
		}
		return
	}
}

var versionSuffixPattern = regexp.MustCompile(`-.*$`)

func removeVersionPrefixAndSuffix(version string) string {
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimSuffix(version, "\n")
	version = versionSuffixPattern.ReplaceAllString(version, "")
	return version
}

func versionSatisfied(current, required string) bool {
	currentSegments := strings.SplitN(removeVersionPrefixAndSuffix(current), ".", 3)
	requiredSegments := strings.SplitN(removeVersionPrefixAndSuffix(required), ".", 3)

	requiredHasSuffix := versionSuffixPattern.MatchString(required)
	if requiredHasSuffix {
		return false // required version should be a formal version
	}

	for i := 0; i < 3; i++ {
		var currentSeg, minimalSeg int
		var err error
		if currentSeg, err = strconv.Atoi(currentSegments[i]); err != nil {
			klog.Warnf("invalid current version: %s, seg=%v, err=%v", current, currentSegments[i], err)
			return false
		}
		if minimalSeg, err = strconv.Atoi(requiredSegments[i]); err != nil {
			klog.Warnf("invalid required version: %s, seg=%v, err=%v", required, requiredSegments[i], err)
			return false
		}
		if currentSeg > minimalSeg {
			return true
		} else if currentSeg < minimalSeg {
			return false
		}
	}
	if currentHasSuffix := versionSuffixPattern.MatchString(current); currentHasSuffix {
		return false
	}
	return true
}

// LookupTool returns the compiler path found in $PATH; if not found, returns $GOPATH/bin/$tool
func LookupTool(idlType, compilerPath string) string {
	tool := "thriftgo"
	if idlType == "protobuf" {
		tool = "protoc"
	}
	if compilerPath != "" {
		log.Infof("Will use the specified %s: %s\n", tool, compilerPath)
		return compilerPath
	}

	path, err := exec.LookPath(tool)
	if err != nil {
		// log.Warnf("Failed to find %q from $PATH: %s.\nTry $GOPATH/bin/%s instead\n", path, err.Error(), tool)
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

func runCommand(input string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	arr := strings.Split(input, " ")
	var cmd *exec.Cmd
	if len(arr) > 1 {
		cmd = exec.CommandContext(ctx, arr[0], arr[1:]...)
	} else {
		cmd = exec.CommandContext(ctx, arr[0])
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	result := strings.TrimSpace(string(output))
	return result, nil
}
