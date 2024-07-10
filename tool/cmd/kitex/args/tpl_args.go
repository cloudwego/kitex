package args

import (
	"flag"
	"fmt"
	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
	"os"
	"strings"
)

func (a *Arguments) addBasicFlags(f *flag.FlagSet, version string) *flag.FlagSet {
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

	for _, e := range a.extends {
		e.Apply(f)
	}
	return f
}

func (a *Arguments) buildInitFlags(version string) *flag.FlagSet {
	f := flag.NewFlagSet("init", flag.ContinueOnError)
	f.StringVar(&a.InitOutputDir, "o", ".", "Specify template init path (default current directory)")
	f = a.addBasicFlags(f, version)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: %s template init [flags]

Examples:
  %s template init -o /path/to/output
  %s template init

Flags:
`, version, os.Args[0], os.Args[0], os.Args[0])
		f.PrintDefaults()
	}
	return f
}

func (a *Arguments) buildRenderFlags(version string) *flag.FlagSet {
	f := flag.NewFlagSet("render", flag.ContinueOnError)
	f.StringVar(&a.TemplateFile, "f", "", "Specify template init path")
	f = a.addBasicFlags(f, version)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: %s template render [template dir_path] [flags] IDL

Examples:
  %s template render ${template dir_path} -module ${module_name} idl/hello.thrift
  %s template render ${template dir_path} -f service.go.tpl -module ${module_name} idl/hello.thrift
  %s template render ${template dir_path} -module ${module_name} -I xxx.git idl/hello.thrift

Flags:
`, version, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
		f.PrintDefaults()
	}
	return f
}

func (a *Arguments) buildCleanFlags(version string) *flag.FlagSet {
	f := flag.NewFlagSet("clean", flag.ContinueOnError)
	f = a.addBasicFlags(f, version)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: %s template clean

Examples:
  %s template clean

Flags:
`, version, os.Args[0], os.Args[0])
		f.PrintDefaults()
	}
	return f
}

func (a *Arguments) TemplateArgs(version, curpath string) error {
	templateCmd := &util.Command{
		Use:   "template",
		Short: "Template command",
		RunE: func(cmd *util.Command, args []string) error {
			fmt.Println("Template command executed")
			return nil
		},
	}
	initCmd := &util.Command{
		Use:   "init",
		Short: "Init command",
		RunE: func(cmd *util.Command, args []string) error {
			fmt.Println("Init command executed")
			f := a.buildInitFlags(version)
			if err := f.Parse(args); err != nil {
				return err
			}
			log.Verbose = a.Verbose

			for _, e := range a.extends {
				err := e.Check(a)
				if err != nil {
					return err
				}
			}

			err := a.checkIDL(f.Args())
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
		},
	}
	renderCmd := &util.Command{
		Use:   "render",
		Short: "Render command",
		RunE: func(cmd *util.Command, args []string) error {
			fmt.Println("Render command executed")
			if len(args) > 0 {
				a.TplDir = args[0]
			}
			var tplDir string
			for i, arg := range args {
				if !strings.HasPrefix(arg, "-") {
					tplDir = arg
					args = append(args[:i], args[i+1:]...)
					break
				}
			}
			if tplDir == "" {
				cmd.PrintUsage()
				return fmt.Errorf("template directory is required")
			}

			f := a.buildRenderFlags(version)
			if err := f.Parse(args); err != nil {
				return err
			}
			log.Verbose = a.Verbose

			for _, e := range a.extends {
				err := e.Check(a)
				if err != nil {
					return err
				}
			}

			err := a.checkIDL(f.Args())
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
		},
	}
	cleanCmd := &util.Command{
		Use:   "clean",
		Short: "Clean command",
		RunE: func(cmd *util.Command, args []string) error {
			fmt.Println("Clean command executed")
			f := a.buildCleanFlags(version)
			if err := f.Parse(args); err != nil {
				return err
			}
			log.Verbose = a.Verbose

			for _, e := range a.extends {
				err := e.Check(a)
				if err != nil {
					return err
				}
			}

			err := a.checkIDL(f.Args())
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
		},
	}
	templateCmd.AddCommand(initCmd)
	templateCmd.AddCommand(renderCmd)
	templateCmd.AddCommand(cleanCmd)

	if _, err := templateCmd.ExecuteC(); err != nil {
		return err
	}
	return nil
}
