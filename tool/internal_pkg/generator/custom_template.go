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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

var DefaultDelimiters = [2]string{"{{", "}}"}

type Templates struct {
	Layouts []Template `yaml:"layouts"`
}

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

	t, err := readTemplates(g.CustomTemplate)
	if err != nil {
		return nil, err
	}

	for _, tpl := range t.Layouts {
		update := util.Exists(tpl.Path)
		if update && tpl.UpdateIsSkip {
			log.Infof("skip generate file %s", tpl.Path)
			continue
		}

		// just create dir
		if tpl.Path[len(tpl.Path)-1] == '/' && tpl.LoopField == "" {
			os.MkdirAll(tpl.Path, 0o755)
			continue
		}

		// special handling Methods field
		if tpl.LoopField == "Methods" {
			tmp := pkg.Methods
			m := pkg.AllMethods()
			for _, method := range m {
				realPath := convert(tpl.Path, method)
				filePath := filepath.Join(g.OutputPath, realPath)
				if util.Exists(filePath) && tpl.UpdateIsSkip {
					continue
				}
				pkg.Methods = []*MethodInfo{method}
				task := &Task{
					Name: path.Base(realPath),
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
		task := &Task{
			Name: path.Base(tpl.Path),
			Path: filepath.Join(g.OutputPath, tpl.Path),
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

func readTemplates(path string) (*Templates, error) {
	cdata, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read layout config from  %s failed, err: %v", path, err.Error())
	}
	t := &Templates{}
	if err = yaml.Unmarshal(cdata, t); err != nil {
		return nil, fmt.Errorf("unmarshal layout config failed, err: %v", err.Error())
	}
	if reflect.DeepEqual(*t, Templates{}) {
		return nil, errors.New("empty config")
	}
	return t, nil
}

func convert(path string, method *MethodInfo) string {
	reg := regexp.MustCompile(`{{.*}}`)
	s := reg.FindAllString(path, -1)
	// not found `{{}}`, no need to replace
	if len(s) == 0 {
		return path
	}
	// Replace `{{}}` with specific field in MethodInfo
	// If the field is not found, it will be replaced with
	// "<invalid Value>".
	field := s[0][2 : len(s[0])-2]
	v := reflect.ValueOf(method)
	f := reflect.Indirect(v).FieldByName(field).String()
	return reg.ReplaceAllString(path, f)
}
