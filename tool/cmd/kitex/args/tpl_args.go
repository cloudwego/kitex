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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/tpl"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
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

	MultipleServicesFileName = "multiple_services.go"
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

var multipleServicesTpl = map[string]string{
	MultipleServicesFileName: tpl.MultipleServicesTpl,
}

const (
	DefaultType          = "default"
	MultipleServicesType = "multiple_services"
)

type TemplateGenerator func(string) error

var genTplMap = map[string]TemplateGenerator{
	DefaultType:          GenTemplates,
	MultipleServicesType: GenMultipleServicesTemplates,
}

// GenTemplates is the entry for command kitex template,
// it will create the specified path
func GenTemplates(path string) error {
	return InitTemplates(path, defaultTemplates)
}

func GenMultipleServicesTemplates(path string) error {
	return InitTemplates(path, multipleServicesTpl)
}

// InitTemplates creates template files.
func InitTemplates(path string, templates map[string]string) error {
	if err := MkdirIfNotExist(path); err != nil {
		return err
	}

	for name, content := range templates {
		var filePath string
		if name == BootstrapFileName {
			bootstrapDir := filepath.Join(path, "script")
			if err := MkdirIfNotExist(bootstrapDir); err != nil {
				return err
			}
			filePath = filepath.Join(bootstrapDir, name+".tpl")
		} else {
			filePath = filepath.Join(path, name+".tpl")
		}
		if err := createTemplate(filePath, content); err != nil {
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
	log.Verbose = a.Verbose

	for _, e := range a.extends {
		err := e.Check(a)
		if err != nil {
			return err
		}
	}

	err = a.checkIDL(args)
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

	magicString := "// Kitex template debug file. use template clean to delete it."
	err = filepath.WalkDir(curpath, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file %s faild: %v", path, err)
		}
		if strings.Contains(string(content), magicString) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("delete file %s failed: %v", path, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error cleaning debug template files: %v", err)
	}
	fmt.Println("clean debug template files successfully...")
	os.Exit(0)
	return nil
}

func (a *Arguments) TemplateArgs(version string) error {
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
	initCmd.Flags().StringVar(&a.InitType, "type", "", "Specify template init type")
	renderCmd.Flags().StringVar(&a.RenderTplDir, "dir", "", "Use custom template to generate codes.")
	renderCmd.Flags().StringVar(&a.ModuleName, "module", "",
		"Specify the Go module name to generate go.mod.")
	renderCmd.Flags().StringVar(&a.IDLType, "type", "unknown", "Specify the type of IDL: 'thrift' or 'protobuf'.")
	renderCmd.Flags().StringVar(&a.GenPath, "gen-path", generator.KitexGenPath,
		"Specify a code gen path.")
	renderCmd.Flags().StringArrayVar(&a.TemplateFiles, "file", []string{}, "Specify single template path")
	renderCmd.Flags().BoolVar(&a.DebugTpl, "debug", false, "turn on debug for template")
	renderCmd.Flags().VarP(&a.Includes, "Includes", "I", "Add IDL search path and template search path for includes.")
	initCmd.SetUsageFunc(func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: kitex template init [flags]
`, version)
	})
	renderCmd.SetUsageFunc(func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: template render --dir [template dir_path] [flags] IDL
`, version)
	})
	cleanCmd.SetUsageFunc(func() {
		fmt.Fprintf(os.Stderr, `Version %s
Usage: kitex template clean
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
