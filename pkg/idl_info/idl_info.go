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

package idl_info

import (
	"fmt"
	"os"

	"github.com/cloudwego/kitex/pkg/klog"
)

// IDLInfo is a whole group of IDLs of a service
type IDLInfo struct {
	IDLInfoMeta
	IDLFileContentMap map[string]string `json:"idl_file_content_map"`
	IDLFileVersionMap map[string]string `json:"idl_file_version_map"`
}

// IDLInfoMeta is the meta info of IDLInfo
type IDLInfoMeta struct {
	IDLServiceName  string `json:"idl_service_name"`
	IDLType         string `json:"idl_type"`
	GoModule        string `json:"go_module"`
	MainIDLPath     string `json:"mainIDLPath"`
	MainIDLCommitID string `json:"main_idl_commit_id"`
}

const IgnoreCheckEnvironment = "KITEX_IDL_IGNORE_REGISTER_CHECK"

func IgnoreRegisterCheck() bool {
	return os.Getenv(IgnoreCheckEnvironment) == "1"
}

// RegisterThriftIDL should only execute when init, so it's not concurrency safe
func RegisterThriftIDL(IDLServiceName, goModule, IDLRepoCommitID string, isMainIDL bool, idlContent, idlPath string) (err error) {
	defer func() {
		if err != nil && IgnoreRegisterCheck() {
			klog.Warnf(err.Error())
			err = nil
		}
	}()

	if IDLServiceName == "" {
		return fmt.Errorf("[kitex IDL] IDL service name is nil")
	}
	info, ok := getInfoFromManager(IDLServiceName)
	if !ok {
		info = &IDLInfo{
			IDLInfoMeta: IDLInfoMeta{
				IDLServiceName: IDLServiceName,
				IDLType:        "thrift",
				GoModule:       goModule,
			},
			IDLFileContentMap: make(map[string]string),
			IDLFileVersionMap: make(map[string]string),
		}
		setInfoToManager(IDLServiceName, info)
	}

	if info.GoModule != goModule {
		// if an IDLServiceName is registered by multiple go mods, panic and stop
		// if environment 'KITEX_IDL_INFO_DISABLE_CONFLICT_CHECK' is enabled, then skip panic.
		return fmt.Errorf("[kitex IDL] '%s' go mod conflict: '%s' -> '%s'", IDLServiceName, info.GoModule, goModule)
	}
	info.IDLFileContentMap[idlPath] = idlContent
	info.IDLFileVersionMap[idlPath] = IDLRepoCommitID
	if isMainIDL {
		if info.MainIDLPath != "" && info.MainIDLPath != idlPath {
			// todo 如果用户只更新了部分 IDL，调整了新的主 IDL，就有可能出现这样的情况，只做提示
			klog.Warnf("[kitex IDL] '%s' main idl path is override: '%s' -> '%s'", IDLServiceName, info.MainIDLPath, idlPath)
		}
		info.MainIDLPath = idlPath
		info.MainIDLCommitID = IDLRepoCommitID
	}
	return nil
}
