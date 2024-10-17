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
	"github.com/cloudwego/kitex/tool/internal_pkg/tpl"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
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
	MultipleServicesFileName: tpl.MainMultipleServicesTpl,
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
		var dir string
		if name == BootstrapFileName {
			dir = filepath.Join(path, "script")
		} else {
			dir = path
		}
		if err := MkdirIfNotExist(dir); err != nil {
			return err
		}
		filePath := filepath.Join(dir, fmt.Sprintf("%s.tpl", name))
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
	var path string
	if InitOutputDir == "" {
		path = curpath
	} else {
		path = InitOutputDir
	}
	if len(InitTypes) == 0 {
		typ := DefaultType
		if err := genTplMap[typ](path); err != nil {
			return err
		}
	} else {
		for _, typ := range InitTypes {
			if _, ok := genTplMap[typ]; !ok {
				return fmt.Errorf("invalid type: %s", typ)
			}
			if err := genTplMap[typ](path); err != nil {
				return err
			}
		}
	}
	os.Exit(0)
	return nil
}

func (a *Arguments) checkTplArgs() error {
	if a.TemplateDir != "" && a.RenderTplDir != "" {
		return fmt.Errorf("template render --dir and -template-dir cannot be used at the same time")
	}
	if a.RenderTplDir != "" && len(a.TemplateFiles) > 0 {
		return fmt.Errorf("template render --dir and --file option cannot be specified at the same time")
	}
	return nil
}

func (a *Arguments) Root(cmd *util.Command, args []string) error {
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
	err = a.checkTplArgs()
	if err != nil {
		return err
	}
	// todo finish protobuf
	if a.IDLType != "thrift" {
		a.GenPath = generator.KitexGenPath
	}
	return a.checkPath(curpath)
}

func (a *Arguments) Template(cmd *util.Command, args []string) error {
	if len(args) == 0 {
		return util.ErrHelp
	}
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
	err = a.checkTplArgs()
	if err != nil {
		return err
	}
	// todo finish protobuf
	if a.IDLType != "thrift" {
		a.GenPath = generator.KitexGenPath
	}
	return a.checkPath(curpath)
}

func parseYAMLFiles(directory string) ([]generator.Template, error) {
	var templates []generator.Template
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".yaml" {
			data, err := os.ReadFile(filepath.Join(directory, file.Name()))
			if err != nil {
				return nil, err
			}
			var template generator.Template
			err = yaml.Unmarshal(data, &template)
			if err != nil {
				return nil, err
			}
			templates = append(templates, template)
		}
	}
	return templates, nil
}

func createFilesFromTemplates(templates []generator.Template, baseDirectory string) error {
	for _, template := range templates {
		fullPath := filepath.Join(baseDirectory, fmt.Sprintf("%s.tpl", template.Path))
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return err
		}
		err = os.WriteFile(fullPath, []byte(template.Body), 0o644)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateMetadata(templates []generator.Template, outputFile string) error {
	var metadata generator.Meta
	for _, template := range templates {
		meta := generator.Template{
			Path:           template.Path,
			UpdateBehavior: template.UpdateBehavior,
			LoopMethod:     template.LoopMethod,
			LoopService:    template.LoopService,
		}
		metadata.Templates = append(metadata.Templates, meta)
	}
	data, err := yaml.Marshal(&metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(outputFile, data, 0o644)
}

func (a *Arguments) Render(cmd *util.Command, args []string) error {
	curpath, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("get current path failed: %s", err.Error())
	}
	if len(args) == 0 {
		return util.ErrHelp
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
	err = a.checkTplArgs()
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
	err = filepath.WalkDir(curpath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file %s failed: %v", path, err)
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

var (
	InitOutputDir string   // specify the location path of init subcommand
	InitTypes     []string // specify the type for init subcommand
)

func (a *Arguments) TemplateArgs(version string) error {
	kitexCmd := &util.Command{
		Use:   "kitex",
		Short: "Kitex command",
		RunE:  a.Root,
	}
	templateCmd := &util.Command{
		Use:   "template",
		Short: "Template command",
		RunE:  a.Template,
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
	kitexCmd.Flags().StringVar(&a.GenPath, "gen-path", generator.KitexGenPath,
		"Specify a code gen path.")
	templateCmd.Flags().StringVar(&a.GenPath, "gen-path", generator.KitexGenPath,
		"Specify a code gen path.")
	initCmd.Flags().StringVar(&a.GenPath, "gen-path", generator.KitexGenPath,
		"Specify a code gen path.")
	initCmd.Flags().StringVarP(&InitOutputDir, "output", "o", ".", "Specify template init path (default current directory)")
	initCmd.Flags().StringArrayVar(&InitTypes, "type", []string{}, "Specify template init type")
	renderCmd.Flags().StringVar(&a.RenderTplDir, "dir", "", "Use custom template to generate codes.")
	renderCmd.Flags().StringVar(&a.ModuleName, "module", "",
		"Specify the Go module name to generate go.mod.")
	renderCmd.Flags().StringVar(&a.IDLType, "type", "unknown", "Specify the type of IDL: 'thrift' or 'protobuf'.")
	renderCmd.Flags().StringVar(&a.GenPath, "gen-path", generator.KitexGenPath,
		"Specify a code gen path.")
	renderCmd.Flags().StringArrayVar(&a.TemplateFiles, "file", []string{}, "Specify single template path")
	renderCmd.Flags().BoolVar(&a.DebugTpl, "debug", false, "turn on debug for template")
	renderCmd.Flags().StringVarP(&a.IncludesTpl, "Includes", "I", "", "Add IDL search path and template search path for includes.")
	renderCmd.Flags().StringVar(&a.MetaFlags, "meta", "", "Meta data in key=value format, keys separated by ';' values separated by ',' ")
	templateCmd.SetHelpFunc(func(*util.Command, []string) {
		fmt.Fprintln(os.Stderr, `
Template operation

Usage:
  kitex template [command]

Available Commands:
  init        Initialize the templates according to the type
  render      Render the template files 
  clean       Clean the debug templates
	`)
	})
	initCmd.SetHelpFunc(func(*util.Command, []string) {
		fmt.Fprintln(os.Stderr, `
Initialize the templates according to the type

Usage:
  kitex template init [flags]

Flags:
  -o, --output string        Output directory
  -t, --type   string        The init type of the template
	`)
	})
	renderCmd.SetHelpFunc(func(*util.Command, []string) {
		fmt.Fprintln(os.Stderr, `
Render the template files 

Usage:
  kitex template render [flags]

Flags:
  --dir          string         Output directory
  --debug        bool           Turn on the debug mode
  --file         stringArray    Specify multiple files for render
  -I, --Includes string         Add an template git search path for includes.
  --meta         string         Specify meta data for render
  --module       string         Specify the Go module name to generate go.mod.
  -t, --type     string         The init type of the template
	`)
	})
	cleanCmd.SetHelpFunc(func(*util.Command, []string) {
		fmt.Fprintln(os.Stderr, `
Clean the debug templates

Usage:
  kitex template clean
	`)
	})
	templateCmd.AddCommand(initCmd, renderCmd, cleanCmd)
	kitexCmd.AddCommand(templateCmd)
	if _, err := kitexCmd.ExecuteC(); err != nil {
		return err
	}
	return nil
}
