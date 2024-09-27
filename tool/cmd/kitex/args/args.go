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
	Check func(*Arguments) error
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
	f.BoolVar(&a.NoDependencyCheck, "no-dependency-check", false,
		"Skip dependency checking.")
	f.BoolVar(&a.Rapid, "rapid", false,
		"Use embedded thriftgo.")
	f.Var(&a.BuiltinTpl, "tpl", "Specify kitex built-in template.")

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
		"no_processor",
	)

	f.Usage = func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: %s [flags] IDL

Examples:
%s
Flags:
`, a.Version, os.Args[0], cmdExample)
		f.PrintDefaults()
	}

	for _, e := range a.extends {
		e.Apply(f)
	}

	return f
}

// ParseArgs parses command line arguments.
func (a *Arguments) ParseArgs(version, curpath string, kitexArgs []string) (err error) {
	f := a.buildFlags(version)
	if err = f.Parse(kitexArgs); err != nil {
		return err
	}

	log.Verbose = a.Verbose

	for _, e := range a.extends {
		err = e.Check(a)
		if err != nil {
			return err
		}
	}

	err = a.checkIDL(f.Args())
	if err != nil {
		return err
	}
	err = a.checkServiceName()
	if err != nil {
		return err
	}
	// todo finish protobuf
	if a.IDLType != "thrift" {
		a.GenPath = generator.KitexGenPath
	}
	return a.checkPath(curpath)
}

func (a *Arguments) checkIDL(files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("no IDL file found; Please make your IDL file the last parameter, for example: " +
			"\"kitex -service demo idl.thrift\"")
	}
	if len(files) != 1 {
		return fmt.Errorf("require exactly 1 IDL file; Please make your IDL file the last parameter, for example: " +
			"\"kitex -service demo idl.thrift\"")
	}
	a.IDL = files[0]

	switch a.IDLType {
	case "thrift", "protobuf":
	case "unknown":
		if typ, ok := a.guessIDLType(); ok {
			a.IDLType = typ
		} else {
			return fmt.Errorf("can not guess an IDL type from %q (unknown suffix), please specify with the '-type' flag", a.IDL)
		}
	default:
		return fmt.Errorf("unsupported IDL type: %s", a.IDLType)
	}
	return nil
}

func (a *Arguments) checkServiceName() error {
	if a.ServiceName == "" && a.TemplateDir == "" {
		if a.Use != "" {
			return fmt.Errorf("-use must be used with -service or -template-dir")
		}
	}
	if a.ServiceName != "" && a.TemplateDir != "" {
		return fmt.Errorf("-template-dir and -service cannot be specified at the same time")
	}
	if a.ServiceName != "" {
		a.GenerateMain = true
	}
	return nil
}

func (a *Arguments) checkPath(curpath string) error {
	pathToGo, err := exec.LookPath("go")
	if err != nil {
		return err
	}

	gopath, err := util.GetGOPATH()
	if err != nil {
		return err
	}
	gosrc := util.JoinPath(gopath, "src")
	gosrc, err = filepath.Abs(gosrc)
	if err != nil {
		return fmt.Errorf("get GOPATH/src path failed: %s", err.Error())
	}

	if strings.HasPrefix(curpath, gosrc) {
		if a.PackagePrefix, err = filepath.Rel(gosrc, curpath); err != nil {
			return fmt.Errorf("get GOPATH/src relpath failed: %s", err.Error())
		}
		a.PackagePrefix = util.JoinPath(a.PackagePrefix, a.GenPath)
	} else {
		if a.ModuleName == "" {
			return fmt.Errorf("outside of $GOPATH. Please specify a module name with the '-module' flag")
		}
	}

	if a.ModuleName != "" {
		module, path, ok := util.SearchGoMod(curpath)
		if ok {
			// go.mod exists
			if module != a.ModuleName {
				return fmt.Errorf("the module name given by the '-module' option ('%s') is not consist with the name defined in go.mod ('%s' from %s)",
					a.ModuleName, module, path)
			}
			if a.PackagePrefix, err = filepath.Rel(path, curpath); err != nil {
				return fmt.Errorf("get package prefix failed: %s", err.Error())
			}
			a.PackagePrefix = util.JoinPath(a.ModuleName, a.PackagePrefix, a.GenPath)
		} else {
			if err = initGoMod(pathToGo, a.ModuleName); err != nil {
				return fmt.Errorf("init go mod failed: %s", err.Error())
			}
			a.PackagePrefix = util.JoinPath(a.ModuleName, a.GenPath)
		}
	}

	if a.Use != "" {
		a.PackagePrefix = a.Use
	}
	a.OutputPath = curpath
	return nil
}

// BuildCmd builds an exec.Cmd.
func (a *Arguments) BuildCmd(out io.Writer) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to detect current executable: %s", err.Error())
	}

	for i, inc := range a.Includes {
		if strings.HasPrefix(inc, "git@") || strings.HasPrefix(inc, "http://") || strings.HasPrefix(inc, "https://") {
			localGitPath, errMsg, gitErr := util.RunGitCommand(inc)
			if gitErr != nil {
				if errMsg == "" {
					errMsg = gitErr.Error()
				}
				return nil, fmt.Errorf("failed to pull IDL from git:%s\nYou can execute 'rm -rf ~/.kitex' to clean the git cache and try again", errMsg)
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

	if a.IDLType == "thrift" {
		os.Setenv(EnvPluginMode, thriftgo.PluginName)
		cmd.Args = append(cmd.Args, "thriftgo")
		for _, inc := range a.Includes {
			cmd.Args = append(cmd.Args, "-i", inc)
		}
		if thriftgo.IsHessian2(a.Config) {
			err = thriftgo.Hessian2PreHook(&a.Config)
			if err != nil {
				return nil, err
			}
		}
		a.ThriftOptions = append(a.ThriftOptions, "package_prefix="+a.PackagePrefix)

		// see README.md in `bthrift`
		a.ThriftOptions = append(a.ThriftOptions,
			"thrift_import_path=github.com/cloudwego/kitex/pkg/protocol/bthrift/apache")

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
				return nil, fmt.Errorf("failed to get the correct protoc plugin parameters for %s. Please specify the protoc plugin in the form of \"plugin_name:options:out_dir\"", p)
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
	return cmd, nil
}

// ValidateCMD check if the path exists and if the version is satisfied
func ValidateCMD(path, idlType string) error {
	// check if the path exists
	if _, err := os.Stat(path); err != nil {
		tool := "thriftgo"
		if idlType == "protobuf" {
			tool = "protoc"
		}

		if idlType == "thrift" {
			_, err = runCommand("go install github.com/cloudwego/thriftgo@latest")
			if err != nil {
				return fmt.Errorf("[ERROR] %s is also unavailable, please install %s first.\n"+
					"Refer to https://github.com/cloudwego/thriftgo, or simple run:\n"+
					"  go install -v github.com/cloudwego/thriftgo@latest", path, tool)
			}
		} else {
			return fmt.Errorf("[ERROR] %s is also unavailable, please install %s first.\n"+
				"Refer to https://github.com/protocolbuffers/protobuf", path, tool)
		}
	}

	// check if the version is satisfied
	if idlType == "thrift" {
		// run `thriftgo -versions and get the output
		cmd := exec.Command(path, "-version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to get thriftgo version: %s", err.Error())
		}
		if !strings.HasPrefix(string(out), "thriftgo ") {
			return fmt.Errorf("thriftgo -version returns '%s', please reinstall thriftgo first", string(out))
		}
		version := strings.Replace(strings.TrimSuffix(string(out), "\n"), "thriftgo ", "", 1)
		if !versionSatisfied(version, requiredThriftGoVersion) {
			_, err = runCommand("go install github.com/cloudwego/thriftgo@latest")
			if err != nil {
				return fmt.Errorf("[ERROR] thriftgo version(%s) not satisfied, please install version >= %s",
					version, requiredThriftGoVersion)
			}
		}
		return nil
	}
	return nil
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
		gopath, er := util.GetGOPATH()
		if er != nil {
			return ""
		}
		path = util.JoinPath(gopath, "bin", tool)
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
