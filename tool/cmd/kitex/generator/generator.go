package generator

import (
	"bytes"
	"fmt"
	"os"

	"github.com/cloudwego/kitex/tool/internal_pkg/thriftgo"

	"github.com/cloudwego/kitex/tool/cmd/kitex/code"

	kargs "github.com/cloudwego/kitex/tool/cmd/kitex/args"
	"github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/protoc"
	thriftgo_plugin "github.com/cloudwego/kitex/tool/internal_pkg/pluginmode/thriftgo"
	"github.com/cloudwego/kitex/tool/internal_pkg/prutal"
	"github.com/cloudwego/kitex/tool/internal_pkg/util/env"

	"github.com/cloudwego/kitex"
	t "github.com/cloudwego/kitex/tool/internal_pkg/thriftgo"
	"github.com/cloudwego/thriftgo/plugin"
)

// RunKitexThriftgoGen run kitex tool to generate thrift as sdk
func RunKitexThriftgoGen(wd string, plugins []plugin.SDKPlugin, kitexOsArgs ...string) (error, code.ToolExitCode) {
	var args kargs.Arguments
	err := args.ParseArgs(kitex.Version, wd, kitexOsArgs)
	if err != nil {
		return err, code.ToolArgsError
	}
	return t.RunKitexThriftgoGenFromArgs(wd, plugins, &args)
}

func RunKitexTool(curpath string, osArgs []string, args *kargs.Arguments, toolVersion string) (err error, retCode code.ToolExitCode) {
	// try run as thriftgo/protoc plugin process
	mode := os.Getenv(kargs.EnvPluginMode)
	if len(osArgs) <= 1 && mode != "" {
		switch mode {
		case thriftgo_plugin.PluginName:
			return code.WrapExitCode(thriftgo_plugin.Run())
		case protoc.PluginName:
			return code.WrapExitCode(protoc.Run())
		}
		return fmt.Errorf("unknown plugin mode %s", mode), code.ToolArgsError
	}

	// parse arguments
	err = args.ParseArgs(toolVersion, curpath, osArgs[1:])
	if err != nil {
		if code.IsInterrupted(err) {
			return nil, code.ToolInterrupted
		}
		if code.IsVersionCheckFailed(err) {
			return err, code.ToolVersionCheckFailed
		}
		return err, code.ToolArgsError
	}

	if args.IsProtobuf() && !env.UseProtoc() {
		if err = prutal.NewPrutalGen(args.Config).Process(); err != nil {
			return err, code.ToolExecuteFailed
		}
	}

	if args.IsThrift() && !args.LocalThriftgo {
		return thriftgo.RunKitexThriftgoGenFromArgs(curpath, nil, args)
	}

	out := new(bytes.Buffer)
	cmd, err := args.BuildCmd(out)
	if err != nil {
		return err, code.ToolArgsError
	}

	err = kargs.ValidateCMD(cmd.Path, args.IDLType)
	if err != nil {
		return err, code.ToolArgsError
	}

	if err = cmd.Run(); err != nil {
		return err, code.ToolExecuteFailed
	}

	return nil, code.ToolSuccess
}
