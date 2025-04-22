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
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestSearchGitRoot(t *testing.T) {
	temp := t.TempDir()

	// temp/
	// ├── idl_repo
	// │   ├── .git
	// │   ├── sub_git_repo
	// │   │   ├── .git
	// │   │   └── d.thrift ( git root: temp/idl_repo/sub_git_repo)
	// │   ├── sub_dir
	// │   │   └── c.thrift ( git root: temp/idl_repo)
	// │   └── b.thrift ( git root: temp/idl_repo)
	// └── a.thrift ( not in git repo )

	idlRepoPath := filepath.Join(temp, "idl_repo")
	createDir(idlRepoPath, t)
	createGitDir(idlRepoPath, t)

	subGitRepoPath := filepath.Join(idlRepoPath, "sub_git_repo")
	createDir(subGitRepoPath, t)
	createGitDir(subGitRepoPath, t)
	dThriftPath := filepath.Join(subGitRepoPath, "d.thrift")
	createFile(dThriftPath, t)

	subDirPath := filepath.Join(idlRepoPath, "sub_dir")
	createDir(subDirPath, t)
	cThriftPath := filepath.Join(subDirPath, "c.thrift")
	createFile(cThriftPath, t)

	bThriftPath := filepath.Join(idlRepoPath, "b.thrift")
	createFile(bThriftPath, t)

	aThriftPath := filepath.Join(temp, "a.thrift")
	createFile(aThriftPath, t)

	root, ok, err := searchGitRoot(aThriftPath)
	test.Assert(t, err == nil)
	test.Assert(t, !ok)
	test.Assert(t, root == "")

	root, ok, err = searchGitRoot(bThriftPath)
	test.Assert(t, err == nil)
	test.Assert(t, ok)
	test.Assert(t, root == idlRepoPath)

	root, ok, err = searchGitRoot(cThriftPath)
	test.Assert(t, err == nil)
	test.Assert(t, ok)
	test.Assert(t, root == idlRepoPath)

	root, ok, err = searchGitRoot(dThriftPath)
	test.Assert(t, err == nil)
	test.Assert(t, ok)
	test.Assert(t, root == subGitRepoPath)

	_, ok, err = searchGitRoot(filepath.Join(subGitRepoPath, "efg.thrift"))
	test.Assert(t, err != nil)
	test.Assert(t, !ok)
}

func createDir(path string, t *testing.T) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", path, err)
	}
}

func createGitDir(parentPath string, t *testing.T) {
	gitDir := filepath.Join(parentPath, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git directory in %s: %v", parentPath, err)
	}
}

func createFile(filePath string, t *testing.T) {
	if _, err := os.Create(filePath); err != nil {
		t.Fatalf("Failed to create file %s: %v", filePath, err)
	}
}
