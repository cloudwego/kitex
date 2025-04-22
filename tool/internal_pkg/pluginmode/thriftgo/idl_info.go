/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package thriftgo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/plugin"
	"github.com/cloudwego/thriftgo/tool/trimmer/dump"
	"gopkg.in/yaml.v3"

	"github.com/cloudwego/kitex"
	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

const IdlInfoMetaFile = "idl_info_meta.yaml"

type idlMeta struct {
	IDLServiceName  string `yaml:"idl_service_name"`
	GoModule        string `yaml:"go_module"`
	MainIDLCommitID string `yaml:"main_idl_commit_id"`
	MainIDLFile     string `yaml:"main_idl_file"`
}

// decideIDLRegistration determines whether the IDL needs to be registered. If so, it returns the meta for registration.
func (p *patcher) decideIDLRegistration(outputPath, mainIDLFile string) (meta *idlMeta, doIDLRegistration bool, err error) {
	metaFilePath := filepath.Join(outputPath, IdlInfoMetaFile)
	IDLServiceName := p.registerIDL
	// when registerIDL is empty, check if there's meta file under kitex_gen.
	// If it exists, automatically parse the IDLServiceName and register it.
	if IDLServiceName == "" {
		mdata, err := os.ReadFile(metaFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		meta = &idlMeta{}
		if err = yaml.Unmarshal(mdata, &meta); err != nil {
			return nil, false, err
		}
		if meta.IDLServiceName == "" {
			return nil, false, fmt.Errorf("idl service name is empty in '%s', you can delete it or complete it", metaFilePath)
		}
		IDLServiceName = meta.IDLServiceName
		fmt.Printf("[INFO] [kitex IDL] auto register [%s]\n", IDLServiceName)
	} else {
		// todo: fixme later with log.Infof when kitex tool refactor pr is merged.
		fmt.Printf("[INFO] [kitex IDL] register [%s]\n", IDLServiceName)
	}
	// get commitID of IDL git repo, so users can distinguish the versions of the IDL.
	// if IDL repo is not git repo or fail to get commit, just warning and continue, because commitID is not important, just help user to know the IDL version.
	mainIDLCommitID, err := getGitCommitHashWithCache(mainIDLFile)
	if err != nil {
		fmt.Printf("[WARN] [kitex idl info] get commitID of IDL repo failed: %v\n", err)
		mainIDLCommitID = ""
	}
	return &idlMeta{
		IDLServiceName:  IDLServiceName,
		GoModule:        p.module,
		MainIDLCommitID: mainIDLCommitID,
		MainIDLFile:     mainIDLFile,
	}, true, nil
}

func genIDLInfo(suffix, outputPath string, ast *parser.Thrift, meta *idlMeta, pkgName string, isMain, useTrimmer bool) (patches []*plugin.Generated, err error) {
	patches = make([]*plugin.Generated, 0, 2)
	var content string
	// if ast is trimmed, dump it to string.
	// otherwise, read the original file and dump it to string, which has the same format as the original file.
	if useTrimmer {
		content, err = dump.DumpIDL(ast)
		if err != nil {
			return nil, fmt.Errorf("dump trimmed idl %q: %w", ast.Filename, err)
		}
	} else {
		bs, err := os.ReadFile(ast.Filename)
		if err != nil {
			return nil, fmt.Errorf("copy idl content %q: %w", ast.Filename, err)
		}
		content = string(bs)
	}

	baseName := filepath.Base(ast.Filename)
	ext := filepath.Ext(baseName)
	fileNameWithoutExt := strings.TrimSuffix(baseName, ext) + suffix
	copiedFilename := util.JoinPath(outputPath, fileNameWithoutExt+ext)

	patches = append(patches, &plugin.Generated{
		Content: content,
		Name:    &copiedFilename,
	})

	// get commitID for current file.
	// It might be different from main idl file commit id when -i (include) is set so some files are from another git repo.
	commitID, err := getGitCommitHashWithCache(ast.Filename)
	if err != nil {
		fmt.Printf("[WARN] [kitex idl info] get commitID failed for %s: %v\n", ast.Filename, err)
		commitID = ""
	}

	idlInfo := renderIDLInfo(pkgName, isMain, meta.IDLServiceName, meta.GoModule, commitID, ast.Filename, copiedFilename)
	idlInfoPath := util.JoinPath(outputPath, "k-idl-"+fileNameWithoutExt+".go")

	patches = append(patches, &plugin.Generated{
		Content: idlInfo,
		Name:    &idlInfoPath,
	})

	return patches, nil
}

func renderIDLInfo(pkg string, isMain bool, IDLServiceName, goModule, commitID, originIDLFilename, copiedIDLFilename string) string {
	w := &bytes.Buffer{}
	fm := func(format string, aa ...any) {
		fmt.Fprintf(w, format, aa...)
		fmt.Fprintln(w)
	}

	fileShortname := filepath.Base(copiedIDLFilename)
	fileAlias := strings.ReplaceAll(strings.TrimSuffix(fileShortname, filepath.Ext(fileShortname)), ".", "_")
	contentVarName := fmt.Sprintf("content_%s", fileAlias)

	fm("// Code generated by Kitex %s. DO NOT EDIT.", kitex.Version)
	fm("package %s\n", pkg)
	fm("import (")
	fm("  _ \"embed\"\n")
	fm("  \"github.com/cloudwego/kitex/pkg/idl_info\"")
	fm(")\n")

	fm("//go:embed %s", fileShortname)
	fm("var %s string\n", contentVarName)

	fm("func init() {")
	fm("\terr := idl_info.RegisterThriftIDL(\"%s\",\"%s\",\"%s\" ,%t, %s, \"%s\")", IDLServiceName, goModule, commitID, isMain, contentVarName, originIDLFilename)
	fm("\tif err!=nil{")
	fm("\t\tpanic(err)")
	fm("\t}")
	fm("}")

	return w.String()
}

var gitCommitHashCache = make(map[string]string)

func getGitCommitHashWithCache(path string) (string, error) {
	gitRoot, hasGit, err := searchGitRoot(path)
	if err != nil {
		return "", err
	}
	if !hasGit {
		return "", nil
	}

	if hash, exists := gitCommitHashCache[gitRoot]; exists {
		return hash, nil
	}

	// git rev-parse --short=8 HEAD
	cmd := exec.Command("git", "rev-parse", "--short=8", "HEAD")
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git error %s: %s", err.Error(), string(output))
	}
	hash := strings.TrimSpace(string(output))

	gitCommitHashCache[gitRoot] = hash

	return hash, nil
}

func searchGitRoot(path string) (gitRoot string, hasGit bool, err error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", false, err
	}
	if !fileInfo.IsDir() {
		path = filepath.Dir(path)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false, err
	}

	for {
		gitDir := filepath.Join(absPath, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return absPath, true, nil
		}
		parent := filepath.Dir(absPath)
		if parent == absPath {
			break
		}
		absPath = parent
	}
	return "", false, nil
}
