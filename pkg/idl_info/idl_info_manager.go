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
	"sync"
)

var (
	globalManager = make(map[string]*IDLInfo)
	managerMutex  sync.RWMutex
)

// getInfoFromManager
func getInfoFromManager(psm string) (*IDLInfo, bool) {
	managerMutex.RLock()
	defer managerMutex.RUnlock()
	info, ok := globalManager[psm]
	return info, ok
}

// setInfoToManager
func setInfoToManager(psm string, info *IDLInfo) {
	managerMutex.Lock()
	defer managerMutex.Unlock()
	globalManager[psm] = info
}

// getAllIDLInfo
func getAllIDLInfoMetas() []*IDLInfoMeta {
	managerMutex.RLock()
	defer managerMutex.RUnlock()
	metas := make([]*IDLInfoMeta, 0, len(globalManager))
	for _, info := range globalManager {
		metas = append(metas, &info.IDLInfoMeta)
	}
	return metas
}

// resetGlobalManager
func resetGlobalManager() {
	managerMutex.Lock()
	defer managerMutex.Unlock()
	globalManager = make(map[string]*IDLInfo)
}
