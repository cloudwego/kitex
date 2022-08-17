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
)

// WithXDSServerAddress specifies the address of the xDS server the manager will connect to.
func WithXDSServerAddress(address string) manager.Option {
	return manager.Option{
		F: func(o *manager.Options) {
			o.XDSSvrConfig.SvrAddr = address
		},
	}
}
