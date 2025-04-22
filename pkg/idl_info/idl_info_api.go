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

// GetIDLInfoList get all IDLInfoMeta， deep copy safe
func GetIDLInfoList() []IDLInfoMeta {
	originalMetas := getAllIDLInfoMetas()
	// 创建一个新的切片用于存储深拷贝的数据
	copiedMetas := make([]IDLInfoMeta, len(originalMetas))
	// 遍历原始切片，逐个拷贝元素
	for i, meta := range originalMetas {
		copiedMetas[i] = *meta
	}
	return copiedMetas
}

// GetIDLInfoDetails get IDLInfo by IDLServiceName
func GetIDLInfoDetails(IDLServiceName string) (*IDLInfo, bool) {
	info, ok := getInfoFromManager(IDLServiceName)
	if !ok {
		return nil, false
	}
	// deep copy and return
	return &IDLInfo{
		IDLInfoMeta:       info.IDLInfoMeta,
		IDLFileContentMap: copyStringMap(info.IDLFileContentMap),
		IDLFileVersionMap: copyStringMap(info.IDLFileVersionMap),
	}, true
}

func copyStringMap(m map[string]string) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
