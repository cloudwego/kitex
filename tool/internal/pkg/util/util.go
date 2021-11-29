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

package util

import (
	"fmt"
	"go/build"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/cloudwego/kitex/tool/internal/pkg/log"
)

// StringSlice implements the flag.Value interface on string slices
// to allow a flag to be set multiple times.
type StringSlice []string

func (ss *StringSlice) String() string {
	return fmt.Sprintf("%v", *ss)
}

// Set implements the flag.Value interface.
func (ss *StringSlice) Set(value string) error {
	*ss = append(*ss, value)
	return nil
}

// FormatCode formats go source codes.
func FormatCode(code []byte) ([]byte, error) {
	formatCode, err := format.Source(code)
	if err != nil {
		return code, fmt.Errorf("format code error: %s", err)
	}
	return formatCode, nil
}

// WriteToFile writes data into target file.
func WriteToFile(filename string, data []byte) {
	var err error

	if strings.HasSuffix(filename, ".go") {
		data, err = FormatCode(data)
		if err != nil {
			log.Warnf("format %v code error: %v\n", filename, err)
		}
		err = os.WriteFile(filename, data, 0o644)
	} else if strings.HasSuffix(filename, ".sh") {
		err = os.WriteFile(filename, data, 0o755)
	} else {
		err = os.WriteFile(filename, data, 0o644)
	}
	if err != nil {
		log.Warn("write to file err:", err.Error())
	}
}

// GetGOPATH retrieves the GOPATH from environment variables or the `go env` command.
func GetGOPATH() string {
	goPath := os.Getenv("GOPATH")
	// If there are many path in GOPATH, pick up the first one.
	if GoPaths := strings.Split(goPath, ":"); len(GoPaths) > 1 {
		return GoPaths[0]
	}
	// GOPATH not set through environment variables, try to get one by executing "go env GOPATH"
	output, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		log.Warn(err)
		os.Exit(1)
	}

	goPath = strings.TrimSpace(string(output))
	if len(goPath) == 0 {
		buildContext := build.Default
		goPath = buildContext.GOPATH
	}

	if len(goPath) == 0 {
		panic("GOPATH not found")
	}
	return goPath
}

// Exists reports whether a file exists.
func Exists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return os.IsExist(err)
	}
	return !fi.IsDir()
}

// LowerFirst converts the first letter to upper case for the given string.
func LowerFirst(s string) string {
	rs := []rune(s)
	rs[0] = unicode.ToLower(rs[0])
	return string(rs)
}

// UpperFirst converts the first letter to upper case for the given string.
func UpperFirst(s string) string {
	rs := []rune(s)
	rs[0] = unicode.ToUpper(rs[0])
	return string(rs)
}

// NotPtr converts an pointer type into non-pointer type.
func NotPtr(s string) string {
	return strings.Replace(s, "*", "", -1)
}

// SearchGoMod searches go.mod from the given directory (which must be an absolute path) to
// the root directory. When the go.mod is found, its module name and path will be returned.
func SearchGoMod(cwd string) (moduleName, path string, found bool) {
	for {
		path = filepath.Join(cwd, "go.mod")
		data, err := os.ReadFile(path)
		if err == nil {
			re := regexp.MustCompile(`^\s*module\s+(\S+)\s*`)
			for _, line := range strings.Split(string(data), "\n") {
				m := re.FindStringSubmatch(line)
				if m != nil {
					return m[1], cwd, true
				}
			}
			return fmt.Sprintf("<module name not found in '%s'>", path), path, true
		}

		if !os.IsNotExist(err) {
			return
		}
		if cwd == "/" {
			break
		}
		cwd = filepath.Dir(cwd)
	}
	return
}
