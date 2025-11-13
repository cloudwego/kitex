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

package shmipc

import (
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans"
)

type cliTransHandlerFactory struct{}

// NewCliTransHandlerFactory to build ClientTransHandlerFactory in shmipc mode
func NewCliTransHandlerFactory() remote.ClientTransHandlerFactory {
	return &cliTransHandlerFactory{}
}

func (f *cliTransHandlerFactory) NewTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	return newCliTransHandler(opt)
}

func newCliTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	return trans.NewDefaultCliTransHandler(opt, newShmIPCExtension())
}
