// Copyright 2024 CloudWeGo Authors
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
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

func (a *Arguments) TemplateArgs(version, curpath string) error {
	kitexCmd := &util.Command{
		Use:   "kitex",
		Short: "Kitex command",
	}
	templateCmd := &util.Command{
		Use:   "template",
		Short: "Template command",
	}
	initCmd := &util.Command{
		Use:   "init",
		Short: "Init command",
		RunE: func(cmd *util.Command, args []string) error {
			log.Verbose = a.Verbose

			for _, e := range a.extends {
				err := e.Check(a)
				if err != nil {
					return err
				}
			}

			err := a.checkIDL(cmd.Flags().Args())
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
			if len(args) > 0 {
				a.RenderTplDir = args[0]
			}
			var tplDir string
			for _, arg := range args {
				if !strings.HasPrefix(arg, "-") {
					tplDir = arg
					break
				}
			}
			if tplDir == "" {
				return fmt.Errorf("template directory is required")
			}
			log.Verbose = a.Verbose

			for _, e := range a.extends {
				err := e.Check(a)
				if err != nil {
					return err
				}
			}

			err := a.checkIDL(cmd.Flags().Args()[1:])
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
			log.Verbose = a.Verbose

			for _, e := range a.extends {
				err := e.Check(a)
				if err != nil {
					return err
				}
			}

			err := a.checkIDL(cmd.Flags().Args())
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
	initCmd.Flags().StringVarP(&a.InitOutputDir, "output", "o", ".", "Specify template init path (default current directory)")
	renderCmd.Flags().StringVarP(&a.ModuleName, "module", "m", "",
		"Specify the Go module name to generate go.mod.")
	renderCmd.Flags().StringVar(&a.IDLType, "type", "unknown", "Specify the type of IDL: 'thrift' or 'protobuf'.")
	renderCmd.Flags().StringVar(&a.GenPath, "gen-path", generator.KitexGenPath,
		"Specify a code gen path.")
	renderCmd.Flags().StringVarP(&a.TemplateFile, "file", "f", "", "Specify single template path")
	renderCmd.Flags().VarP(&a.Includes, "Includes", "I", "Add IDL search path and template search path for includes.")
	initCmd.SetUsageFunc(func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: kitex template init [flags]

Examples:
  kitex template init -o /path/to/output
  kitex template init

Flags:
`, version)
	})
	renderCmd.SetUsageFunc(func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: template render [template dir_path] [flags] IDL
`, version)
	})
	cleanCmd.SetUsageFunc(func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: kitex template clean

Examples:
  kitex template clean

Flags:
`, version)
	})
	// renderCmd.PrintUsage()
	templateCmd.AddCommand(initCmd, renderCmd, cleanCmd)
	kitexCmd.AddCommand(templateCmd)
	if _, err := kitexCmd.ExecuteC(); err != nil {
		return err
	}
	return nil
}
