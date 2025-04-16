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

	root, ok, err = searchGitRoot(filepath.Join(subGitRepoPath, "efg.thrift"))
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
