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

package xds

import (
	"github.com/cloudwego/kitex/pkg/xds/internal/manager"
	"github.com/cloudwego/kitex/pkg/xds/xdssuite"
)

func newManager() (xdssuite.XDSResourceManager, error) {
	return manager.NewXDSResourceManager(nil)
}

// Init initializes the xds resource manager.
func Init() error {
	return xdssuite.BuildXDSResourceManager(newManager)
}
