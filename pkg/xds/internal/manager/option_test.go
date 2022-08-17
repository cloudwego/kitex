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

package manager

import (
	"github.com/cloudwego/kitex/internal/test"
	"testing"
)

func Test_xdsResourceManager_Option(t *testing.T) {
	addr := "localhost:8080"
	opts := []Option{
		{
			F: func(o *Options) {
				o.XDSSvrConfig.SvrAddr = addr
			},
		},
	}
	o := NewOptions(opts)
	test.Assert(t, o.XDSSvrConfig.SvrAddr == addr)
}
