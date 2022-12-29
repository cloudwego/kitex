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

package generator

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

var DefaultDelimiters = [2]string{"{{", "}}"}

type Template struct {
	// The generated path and its filename. For example: biz/test.go
	// will generate test.go in biz directory.
	Path string `yaml:"path"`
	// Render template content, currently only supports go template syntax
	Body string `yaml:"body"`
	// Chooses whether to cover or skip the file if file exists.
	// If true, kitex will skip the file otherwise it will cover the file.
	UpdateIsSkip bool `yaml:"update_is_skip"`
	// If set this field, kitex will generate file by cycle. For example:
	// test_a/test_b/MethodInfo
	LoopField string `yaml:"loopfield"`
}

func (g *generator) GenerateCustomPackage(pkg *PackageInfo) (fs []*File, err error) {
	g.updatePackageInfo(pkg)

	t, err := readTemplates(g.TemplateDir)
	if err != nil {
		return nil, err
	}

	for _, tpl := range t {
		// special handling Methods field
		if tpl.LoopField == "Methods" {
			tmp := pkg.Methods
			m := pkg.AllMethods()
			for _, method := range m {
				pkg.Methods = []*MethodInfo{method}
				pathTask := &Task{
					Text: tpl.Path,
				}
				renderPath, err := pathTask.RenderString(pkg)
				if err != nil {
					return fs, err
				}
				filePath := filepath.Join(g.OutputPath, renderPath)
				// update
				if util.Exists(filePath) && tpl.UpdateIsSkip {
					continue
				}
				task := &Task{
					Name: path.Base(renderPath),
					Path: filePath,
					Text: tpl.Body,
				}
				f, err := task.Render(pkg)
				if err != nil {
					return fs, err
				}
				fs = append(fs, f)
			}
			pkg.Methods = tmp
			continue
		}

		// custom render
		pathTask := &Task{
			Text: tpl.Path,
		}
		renderPath, err := pathTask.RenderString(pkg)
		if err != nil {
			return fs, err
		}
		filePath := filepath.Join(g.OutputPath, renderPath)
		update := util.Exists(filePath)
		if update && tpl.UpdateIsSkip {
			log.Infof("skip generate file %s", tpl.Path)
			continue
		}

		// just create dir
		if tpl.Path[len(tpl.Path)-1] == '/' && tpl.LoopField == "" {
			os.MkdirAll(filePath, 0o755)
			continue
		}
		task := &Task{
			Name: path.Base(tpl.Path),
			Path: filePath,
			Text: tpl.Body,
		}

		f, err := task.Render(pkg)
		if err != nil {
			return fs, err
		}
		fs = append(fs, f)
	}
	return
}

func readTemplates(dir string) ([]*Template, error) {
	files, _ := ioutil.ReadDir(dir)
	var ts []*Template
	for _, f := range files {
		if f.Name() != ExtensionFilename {
			path := filepath.Join(dir, f.Name())
			tplData, err := ioutil.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read layout config from  %s failed, err: %v", path, err.Error())
			}
			t := &Template{}
			if err = yaml.Unmarshal(tplData, t); err != nil {
				return nil, fmt.Errorf("unmarshal layout config failed, err: %v", err.Error())
			}
			ts = append(ts, t)
		}
	}

	return ts, nil
}
