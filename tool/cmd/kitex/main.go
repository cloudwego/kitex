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

var args arguments

func init() {
	var queryVersion bool
	args.addExtraFlag(&extraFlag{
		apply: func(f *flag.FlagSet) {
			f.BoolVar(&queryVersion, "version", false,
				"Show the version of kitex")
		},
		check: func(a *arguments) {
			if queryVersion {
				println(a.Version)
				os.Exit(0)
			}
		},
	})
}

const pluginMode = "KITEX_PLUGIN_MODE"

func main() {
	mode := os.Getenv(pluginMode)
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
	args.parseArgs()

	out := new(bytes.Buffer)
	cmd := buildCmd(&args, out)
	err := cmd.Run()
	if err != nil {
		if args.Use != "" {
			out := strings.TrimSpace(out.String())
			if strings.HasSuffix(out, thriftgo.TheUseOptionMessage) {
				os.Exit(0)
			}
		}
		os.Exit(1)
	}
}

func lookupTool(idlType string) string {
	tool := "thriftgo"
	if idlType == "protobuf" {
		tool = "protoc"
	}

	path, err := exec.LookPath(tool)
	if err != nil {
		log.Warnf("Failed to find %q from $PATH: %s. Try $GOPATH/bin/%s instead\n", path, err.Error(), tool)
		path = filepath.Join(util.GetGOPATH(), "bin", tool)
	}
	return path
}

func buildCmd(a *arguments, out io.Writer) *exec.Cmd {
	exe, err := os.Executable()
	if err != nil {
		log.Warn("Failed to detect current executable:", err.Error())
		os.Exit(1)
	}

	kas := strings.Join(a.Config.Pack(), ",")
	cmd := &exec.Cmd{
		Path:   lookupTool(a.IDLType),
		Stdin:  os.Stdin,
		Stdout: &teeWriter{out, os.Stdout},
		Stderr: &teeWriter{out, os.Stderr},
	}
	if a.IDLType == "thrift" {
		os.Setenv(pluginMode, thriftgo.PluginName)
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
			"-o", generator.KitexGenPath,
			"-g", gas,
			"-p", "kitex="+exe+":"+kas,
		)
		for _, p := range a.ThriftPlugins {
			cmd.Args = append(cmd.Args, "-p", p)
		}
		cmd.Args = append(cmd.Args, a.IDL)
	} else {
		os.Setenv(pluginMode, protoc.PluginName)
		a.ThriftOptions = a.ThriftOptions[:0]
		// "protobuf"
		cmd.Args = append(cmd.Args, "protoc")
		for _, inc := range a.Includes {
			cmd.Args = append(cmd.Args, "-I", inc)
		}
		outPath := filepath.Join(".", generator.KitexGenPath)
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
		cmd.Args = append(cmd.Args, a.IDL)
	}
	log.Info(strings.ReplaceAll(strings.Join(cmd.Args, " "), kas, fmt.Sprintf("%q", kas)))
	return cmd
}

type teeWriter struct {
	a io.Writer
	b io.Writer
}

func (tw *teeWriter) Write(p []byte) (n int, err error) {
	n, err = tw.a.Write(p)
	if err != nil {
		return
	}
	return tw.b.Write(p)
}
