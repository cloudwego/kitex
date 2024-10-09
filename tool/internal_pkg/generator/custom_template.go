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
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

var DefaultDelimiters = [2]string{"{{", "}}"}

type updateType string

const (
	skip              updateType = "skip"
	cover             updateType = "cover"
	incrementalUpdate updateType = "append"
)

type Update struct {
	// update type: skip / cover / append. Default is skip.
	// If `LoopMethod` is true, only Type field is effect and no append behavior.
	Type string `yaml:"type,omitempty"`
	// Match key in append type. If the rendered key exists in the file, the method will be skipped.
	Key string `yaml:"key,omitempty"`
	// Append template. Use it to render append content.
	AppendTpl string `yaml:"append_tpl,omitempty"`
	// Append import template. Use it to render import content to append.
	ImportTpl []string `yaml:"import_tpl,omitempty"`
}

type Template struct {
	// The generated path and its filename. For example: biz/test.go
	// will generate test.go in biz directory.
	Path string `yaml:"path,omitempty"`
	// Render template content, currently only supports go template syntax
	Body string `yaml:"body,omitempty"`
	// define update behavior
	UpdateBehavior *Update `yaml:"update_behavior,omitempty"`
	// If set this field, kitex will generate file by cycle. For example:
	// test_a/test_b/{{ .Name}}_test.go
	LoopMethod bool `yaml:"loop_method,omitempty"`
	// If both set this field and combine-service, kitex will generate service by cycle.
	LoopService bool `yaml:"loop_service,omitempty"`
}

type customGenerator struct {
	fs       []*File
	pkg      *PackageInfo
	basePath string
}

func NewCustomGenerator(pkg *PackageInfo, basePath string) *customGenerator {
	return &customGenerator{
		pkg:      pkg,
		basePath: basePath,
	}
}

func (c *customGenerator) loopGenerate(tpl *Template) error {
	tmp := c.pkg.Methods
	defer func() {
		c.pkg.Methods = tmp
	}()
	m := c.pkg.AllMethods()
	for _, method := range m {
		c.pkg.Methods = []*MethodInfo{method}
		pathTask := &Task{
			Text: tpl.Path,
		}
		renderPath, err := pathTask.RenderString(c.pkg)
		if err != nil {
			return err
		}
		filePath := filepath.Join(c.basePath, renderPath)
		// update
		if util.Exists(filePath) && updateType(tpl.UpdateBehavior.Type) == skip {
			continue
		}
		task := &Task{
			Name: path.Base(renderPath),
			Path: filePath,
			Text: tpl.Body,
		}
		f, err := task.Render(c.pkg)
		if err != nil {
			return err
		}
		c.fs = append(c.fs, f)
	}
	return nil
}

func (c *customGenerator) commonGenerate(tpl *Template) error {
	// Use all services including base service.
	tmp := c.pkg.Methods
	defer func() {
		c.pkg.Methods = tmp
	}()
	c.pkg.Methods = c.pkg.AllMethods()

	pathTask := &Task{
		Text: tpl.Path,
	}
	renderPath, err := pathTask.RenderString(c.pkg)
	if err != nil {
		return err
	}
	filePath := filepath.Join(c.basePath, renderPath)
	update := util.Exists(filePath)
	if update && updateType(tpl.UpdateBehavior.Type) == skip {
		log.Infof("skip generate file %s", tpl.Path)
		return nil
	}
	var f *File
	if update && updateType(tpl.UpdateBehavior.Type) == incrementalUpdate {
		cc := &commonCompleter{
			path:   filePath,
			pkg:    c.pkg,
			update: tpl.UpdateBehavior,
		}
		f, err = cc.Complete()
		if err != nil {
			return err
		}
	} else {
		// just create dir
		if tpl.Path[len(tpl.Path)-1] == '/' {
			os.MkdirAll(filePath, 0o755)
			return nil
		}

		task := &Task{
			Name: path.Base(tpl.Path),
			Path: filePath,
			Text: tpl.Body,
		}

		f, err = task.Render(c.pkg)
		if err != nil {
			return err
		}
	}

	c.fs = append(c.fs, f)
	return nil
}

func (g *generator) GenerateCustomPackage(pkg *PackageInfo) (fs []*File, err error) {
	g.updatePackageInfo(pkg)

	g.setImports(HandlerFileName, pkg)
	t, err := readTemplates(g.TemplateDir)
	if err != nil {
		return nil, err
	}
	for _, tpl := range t {
		if tpl.LoopService && g.CombineService {
			svrInfo, cs := pkg.ServiceInfo, pkg.CombineServices

			for i := range cs {
				pkg.ServiceInfo = cs[i]
				f, err := renderFile(pkg, g.OutputPath, tpl)
				if err != nil {
					return nil, err
				}
				fs = append(fs, f...)
			}
			pkg.ServiceInfo, pkg.CombineServices = svrInfo, cs
		} else {
			f, err := renderFile(pkg, g.OutputPath, tpl)
			if err != nil {
				return nil, err
			}
			fs = append(fs, f...)
		}
	}
	return fs, nil
}

func renderFile(pkg *PackageInfo, outputPath string, tpl *Template) (fs []*File, err error) {
	cg := NewCustomGenerator(pkg, outputPath)
	// special handling Methods field
	if tpl.LoopMethod {
		err = cg.loopGenerate(tpl)
	} else {
		err = cg.commonGenerate(tpl)
	}
	if errors.Is(err, errNoNewMethod) {
		err = nil
	}
	return cg.fs, err
}

func readTemplates(dir string) ([]*Template, error) {
	files, _ := os.ReadDir(dir)
	var ts []*Template
	for _, f := range files {
		// filter dir and non-yaml files
		if f.Name() != ExtensionFilename && !f.IsDir() && (strings.HasSuffix(f.Name(), "yaml") || strings.HasSuffix(f.Name(), "yml")) {
			p := filepath.Join(dir, f.Name())
			tplData, err := os.ReadFile(p)
			if err != nil {
				return nil, fmt.Errorf("read layout config from  %s failed, err: %v", p, err.Error())
			}
			t := &Template{
				UpdateBehavior: &Update{Type: string(skip)},
			}
			if err = yaml.Unmarshal(tplData, t); err != nil {
				return nil, fmt.Errorf("%s: unmarshal layout config failed, err: %s", f.Name(), err.Error())
			}
			ts = append(ts, t)
		}
	}

	return ts, nil
}

// parseMeta parses the meta flag and returns a map where the value is a slice of strings
func parseMeta(metaFlags string) (map[string][]string, error) {
	meta := make(map[string][]string)
	if metaFlags == "" {
		return meta, nil
	}

	// split for each key=value pairs
	pairs := strings.Split(metaFlags, ";")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key := kv[0]
			values := strings.Split(kv[1], ",")
			meta[key] = values
		} else {
			return nil, fmt.Errorf("invalid meta format: %s", pair)
		}
	}
	return meta, nil
}

func parseMiddlewares(middlewares []MiddlewareForResolve) ([]UserDefinedMiddleware, error) {
	var mwList []UserDefinedMiddleware

	for _, mw := range middlewares {
		content, err := os.ReadFile(mw.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read middleware file %s: %v", mw.Path, err)
		}
		mwList = append(mwList, UserDefinedMiddleware{
			Name:    mw.Name,
			Content: string(content),
		})
	}
	return mwList, nil
}

func (g *generator) GenerateCustomPackageWithTpl(pkg *PackageInfo) (fs []*File, err error) {
	g.updatePackageInfo(pkg)

	g.setImports(HandlerFileName, pkg)
	pkg.ExtendMeta, err = parseMeta(g.MetaFlags)
	if err != nil {
		return nil, err
	}
	if g.Config.IncludesTpl != "" {
		inc := g.Config.IncludesTpl
		if strings.HasPrefix(inc, "git@") || strings.HasPrefix(inc, "http://") || strings.HasPrefix(inc, "https://") {
			localGitPath, errMsg, gitErr := util.RunGitCommand(inc)
			if gitErr != nil {
				if errMsg == "" {
					errMsg = gitErr.Error()
				}
				return nil, fmt.Errorf("failed to pull IDL from git:%s\nYou can execute 'rm -rf ~/.kitex' to clean the git cache and try again", errMsg)
			}
			if g.RenderTplDir != "" {
				g.RenderTplDir = filepath.Join(localGitPath, g.RenderTplDir)
			} else {
				g.RenderTplDir = localGitPath
			}
			if util.Exists(g.RenderTplDir) {
				return nil, fmt.Errorf("the render template directory path you specified does not exists int the git path")
			}
		}
	}
	var meta *Meta
	metaPath := filepath.Join(g.RenderTplDir, KitexRenderMetaFile)
	if util.Exists(metaPath) {
		meta, err = readMetaFile(metaPath)
		if err != nil {
			return nil, err
		}
		middlewares, err := parseMiddlewares(meta.MWs)
		if err != nil {
			return nil, err
		}
		pkg.MWs = middlewares
	}
	tpls, err := readTpls(g.RenderTplDir, g.RenderTplDir, meta)
	if err != nil {
		return nil, err
	}
	for _, tpl := range tpls {
		newPath := filepath.Join(g.OutputPath, tpl.Path)
		dir := filepath.Dir(newPath)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
		if tpl.LoopService && g.CombineService {
			svrInfo, cs := pkg.ServiceInfo, pkg.CombineServices

			for i := range cs {
				pkg.ServiceInfo = cs[i]
				f, err := renderFile(pkg, g.OutputPath, tpl)
				if err != nil {
					return nil, err
				}
				fs = append(fs, f...)
			}
			pkg.ServiceInfo, pkg.CombineServices = svrInfo, cs
		} else {
			f, err := renderFile(pkg, g.OutputPath, tpl)
			if err != nil {
				return nil, err
			}
			fs = append(fs, f...)
		}
	}
	return fs, nil
}

const KitexRenderMetaFile = "kitex_render_meta.yaml"

// Meta represents the structure of the kitex_render_meta.yaml file.
type Meta struct {
	Templates []Template             `yaml:"templates"`
	MWs       []MiddlewareForResolve `yaml:"middlewares"`
}

type MiddlewareForResolve struct {
	// name of the middleware
	Name string `yaml:"name"`
	// path of the middleware
	Path string `yaml:"path"`
}

func readMetaFile(metaPath string) (*Meta, error) {
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read meta file from %s: %v", metaPath, err)
	}

	var meta Meta
	err = yaml.Unmarshal(metaData, &meta)
	if err != nil {
		return nil, fmt.Errorf("failed to parse yaml file %s: %v", metaPath, err)
	}

	return &meta, nil
}

func getMetadata(meta *Meta, relativePath string) *Template {
	for i := range meta.Templates {
		if meta.Templates[i].Path == relativePath {
			return &meta.Templates[i]
		}
	}
	return &Template{
		UpdateBehavior: &Update{Type: string(skip)},
	}
}

func readTpls(rootDir, currentDir string, meta *Meta) (ts []*Template, error error) {
	defaultMetadata := &Template{
		UpdateBehavior: &Update{Type: string(skip)},
	}

	files, _ := os.ReadDir(currentDir)
	for _, f := range files {
		// filter dir and non-tpl files
		if f.IsDir() {
			subDir := filepath.Join(currentDir, f.Name())
			subTemplates, err := readTpls(rootDir, subDir, meta)
			if err != nil {
				return nil, err
			}
			ts = append(ts, subTemplates...)
		} else if strings.HasSuffix(f.Name(), ".tpl") {
			p := filepath.Join(currentDir, f.Name())
			tplData, err := os.ReadFile(p)
			if err != nil {
				return nil, fmt.Errorf("read file from  %s failed, err: %v", p, err.Error())
			}
			// Remove the .tpl suffix from the Path and compute relative path
			relativePath, err := filepath.Rel(rootDir, p)
			if err != nil {
				return nil, fmt.Errorf("failed to compute relative path for %s: %v", p, err)
			}
			trimmedPath := strings.TrimSuffix(relativePath, ".tpl")
			// If kitex_render_meta.yaml exists, get the corresponding metadata; otherwise, use the default metadata
			var metadata *Template
			if meta != nil {
				metadata = getMetadata(meta, relativePath)
			} else {
				metadata = defaultMetadata
			}
			t := &Template{
				Path:           trimmedPath,
				Body:           string(tplData),
				UpdateBehavior: metadata.UpdateBehavior,
				LoopMethod:     metadata.LoopMethod,
				LoopService:    metadata.LoopService,
			}
			ts = append(ts, t)
		}
	}

	return ts, nil
}

func (g *generator) RenderWithMultipleFiles(pkg *PackageInfo) (fs []*File, err error) {
	for _, file := range g.Config.TemplateFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read file from  %s failed, err: %v", file, err.Error())
		}
		var updatedContent string
		if g.Config.DebugTpl {
			// when --debug is enabled, add a magic string at the top of the template content for distinction.
			updatedContent = "// Kitex template debug file. use template clean to delete it.\n\n" + string(content)
		} else {
			updatedContent = string(content)
		}
		filename := filepath.Base(strings.TrimSuffix(file, ".tpl"))
		tpl := &Template{
			Path:           filename,
			Body:           updatedContent,
			UpdateBehavior: &Update{Type: string(skip)},
		}
		f, err := renderFile(pkg, g.OutputPath, tpl)
		if err != nil {
			return nil, err
		}
		fs = append(fs, f...)
	}
	return
}
