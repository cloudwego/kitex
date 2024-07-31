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
	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/tpl"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
	"os"
	"path/filepath"
)

// Constants .
const (
	KitexGenPath = "kitex_gen"
	DefaultCodec = "thrift"

	BuildFileName       = "build.sh"
	BootstrapFileName   = "bootstrap.sh"
	ToolVersionFileName = "kitex_info.yaml"
	HandlerFileName     = "handler.go"
	MainFileName        = "main.go"
	ClientFileName      = "client.go"
	ServerFileName      = "server.go"
	InvokerFileName     = "invoker.go"
	ServiceFileName     = "*service.go"
	ExtensionFilename   = "extensions.yaml"

	ClientMockFilename = "client_mock.go"
)

var defaultTemplates = map[string]string{
	BuildFileName:       tpl.BuildTpl,
	BootstrapFileName:   tpl.BootstrapTpl,
	ToolVersionFileName: tpl.ToolVersionTpl,
	HandlerFileName:     tpl.HandlerTpl,
	MainFileName:        tpl.MainTpl,
	ClientFileName:      tpl.ClientTpl,
	ServerFileName:      tpl.ServerTpl,
	InvokerFileName:     tpl.InvokerTpl,
	ServiceFileName:     tpl.ServiceTpl,
}

var mockTemplates = map[string]string{
	ClientMockFilename: tpl.ClientMockTpl,
}

const (
	DefaultType = "default"
	MockType    = "mock"
)

type TemplateGenerator func(string) error

var genTplMap = map[string]TemplateGenerator{
	DefaultType: GenTemplates,
	MockType:    GenMockTemplates,
}

// GenTemplates is the entry for command kitex template,
// it will create the specified path
func GenTemplates(path string) error {
	for key := range defaultTemplates {
		if key == BootstrapFileName {
			defaultTemplates[key] = util.JoinPath(path, "script", BootstrapFileName)
		}
	}
	return InitTemplates(path, defaultTemplates)
}

func GenMockTemplates(path string) error {
	return InitTemplates(path, mockTemplates)
}

// InitTemplates creates template files.
func InitTemplates(path string, templates map[string]string) error {
	if err := MkdirIfNotExist(path); err != nil {
		return err
	}

	for k, v := range templates {
		if err := createTemplate(filepath.Join(path, k+".tpl"), v); err != nil {
			return err
		}
	}

	return nil
}

// GetTemplateDir returns the category path.
func GetTemplateDir(category string) (string, error) {
	home, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	return filepath.Join(home, category), nil
}

// MkdirIfNotExist makes directories if the input path is not exists
func MkdirIfNotExist(dir string) error {
	if len(dir) == 0 {
		return nil
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, os.ModePerm)
	}

	return nil
}

func createTemplate(file, content string) error {
	if util.Exists(file) {
		return nil
	}

	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
}

func (a *Arguments) Init(cmd *util.Command, args []string) error {
	curpath, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("get current path failed: %s", err.Error())
	}
	path := a.InitOutputDir
	initType := a.InitType
	if initType == "" {
		initType = DefaultType
	}
	if path == "" {
		path = curpath
	}
	if err := genTplMap[initType](path); err != nil {
		return err
	}
	fmt.Printf("Templates are generated in %s\n", path)
	os.Exit(0)
	return nil
}

func (a *Arguments) Render(cmd *util.Command, args []string) error {
	curpath, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("get current path failed: %s", err.Error())
	}
	if len(args) < 2 {
		return fmt.Errorf("both template directory and idl is required")
	}
	a.RenderTplDir = args[0]
	log.Verbose = a.Verbose

	for _, e := range a.extends {
		err := e.Check(a)
		if err != nil {
			return err
		}
	}

	err = a.checkIDL(cmd.Flags().Args()[1:])
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

func (a *Arguments) Clean(cmd *util.Command, args []string) error {
	curpath, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("get current path failed: %s", err.Error())
	}
	log.Verbose = a.Verbose

	for _, e := range a.extends {
		err := e.Check(a)
		if err != nil {
			return err
		}
	}

	err = a.checkIDL(cmd.Flags().Args())
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
		RunE:  a.Init,
	}
	renderCmd := &util.Command{
		Use:   "render",
		Short: "Render command",
		RunE:  a.Render,
	}
	cleanCmd := &util.Command{
		Use:   "clean",
		Short: "Clean command",
		RunE:  a.Clean,
	}
	initCmd.Flags().StringVarP(&a.InitOutputDir, "output", "o", ".", "Specify template init path (default current directory)")
	initCmd.Flags().StringVarP(&a.InitType, "type", "t", "", "Specify template init type")
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
