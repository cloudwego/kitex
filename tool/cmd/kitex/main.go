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
	"syscall"

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

func main() {
	// run as a plugin
	switch filepath.Base(os.Args[0]) {
	case thriftgo.PluginName:
		os.Exit(thriftgo.Run())
	case protoc.PluginName:
		os.Exit(protoc.Run())
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
	exe, err := os.Executable()
	if err != nil {
		log.Warn("Failed to detect current executable:", err.Error())
		os.Exit(1)
	}

	dir := filepath.Dir(exe)
	pgk := filepath.Join(dir, protoc.PluginName)
	tgk := filepath.Join(dir, thriftgo.PluginName)

	link(exe, pgk)
	link(exe, tgk)

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

// link removes the previous symbol link and rebuilds a new one.
func link(src, dst string) {
	err := syscall.Unlink(dst)
	if err != nil && !os.IsNotExist(err) {
		log.Warnf("failed to unlink '%s': %s\n", dst, err)
		os.Exit(1)
	}
	err = os.Symlink(src, dst)
	if err != nil {
		log.Warnf("failed to link '%s' -> '%s': %s\n", src, dst, err)
		os.Exit(1)
	}
}

func buildCmd(a *arguments, out io.Writer) *exec.Cmd {
	kas := strings.Join(a.Config.Pack(), ",")
	cmd := &exec.Cmd{
		Path:   lookupTool(a.IDLType),
		Stdin:  os.Stdin,
		Stdout: &teeWriter{out, os.Stdout},
		Stderr: &teeWriter{out, os.Stderr},
	}
	if a.IDLType == "thrift" {
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
			"-p", "kitex:"+kas,
		)
		for _, p := range a.ThriftPlugins {
			cmd.Args = append(cmd.Args, "-p", p)
		}
		cmd.Args = append(cmd.Args, a.IDL)
	} else {
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
			"--kitex_out="+outPath,
			"--kitex_opt="+kas,
			a.IDL,
		)
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
