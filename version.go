/*
 * Copyright 2021 CloudWeGo Authors
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

package kitex

import (
	"regexp"
	"runtime/debug"
)

const (
	// Name info of this framework, used for statistics and debug
	Name = "KiteX"

	packageName    = "github.com/cloudwego/kitex"
	versionUnknown = "unknown"
)

var (
	// Version info of this framework, used for statistics and debug
	Version string

	versionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)*$`)
)

func init() {
	Version = getVersion(packageName)
}

func getVersion(path string) (version string) {
	defer func() {
		if !versionPattern.MatchString(version) {
			version = versionUnknown
		}
	}()
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if info.Main.Path == path {
		version = info.Main.Version
		return
	}
	for _, dep := range info.Deps {
		if dep.Path == path {
			if dep.Replace != nil {
				version = dep.Replace.Version
			} else {
				version = dep.Version
			}
			return
		}
	}
	return
}
