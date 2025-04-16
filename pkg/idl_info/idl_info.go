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
}

// IDLInfoMeta is the meta info of IDLInfo
type IDLInfoMeta struct {
	IDLServiceName string `json:"idl_service_name"`
	IDLType        string `json:"idl_type"`
	MainIDLPath    string `json:"mainIDLPath"`
	GoModule       string `json:"go_module"`
	IDLCommitID    string `json:"idl_commit_id"`
}

// RegisterThriftIDL should only execute when init, so it's not concurrency safe
func RegisterThriftIDL(IDLServiceName, goModule, IDLRepoCommitID string, isMainIDL bool, idlContent, idlPath string) error {
	info, ok := getInfoFromManager(IDLServiceName)
	if !ok {
		info = &IDLInfo{
			IDLInfoMeta: IDLInfoMeta{
				IDLServiceName: IDLServiceName,
				IDLType:        "thrift",
				IDLCommitID:    IDLRepoCommitID,
				GoModule:       goModule,
			},
			IDLFileContentMap: make(map[string]string),
		}
		setInfoToManager(IDLServiceName, info)
	}

	if info.GoModule != goModule {
		// if an IDLServiceName is registered by multiple go mods, panic and stop
		// if environment 'KITEX_IDL_INFO_DISABLE_CONFLICT_CHECK' is enabled, then skip panic.
		err := fmt.Errorf("[kitex IDL] '%s' go mod conflict: '%s' -> '%s'", IDLServiceName, info.GoModule, goModule)
		if os.Getenv("KITEX_IDL_INFO_DISABLE_CONFLICT_CHECK") == "1" {
			klog.Warnf(err.Error())
			// skip this IDL's registration
			return nil
		} else {
			return err
		}
	}
	info.IDLFileContentMap[idlPath] = idlContent
	if isMainIDL {
		if info.MainIDLPath != "" && info.MainIDLPath != idlPath {
			// todo 如果用户只更新了部分 IDL，调整了新的主 IDL，就有可能出现这样的情况，只做提示
			klog.Warnf("[kitex IDL] '%s' main idl path is override: '%s' -> '%s'", IDLServiceName, info.MainIDLPath, idlPath)
		}
		info.MainIDLPath = idlPath
	}
	return nil
}
