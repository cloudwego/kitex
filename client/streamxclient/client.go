/*
 * Copyright 2024 CloudWeGo Authors
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

package streamxclient

import (
	"github.com/cloudwego/kitex/client"
	iclient "github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

type Client = client.StreamX

func NewClient(svcInfo *serviceinfo.ServiceInfo, opts ...Option) (Client, error) {
	iopts := make([]client.Option, 0, len(opts)+1)
	for _, opt := range opts {
		iopts = append(iopts, ConvertStreamXClientOption(opt))
	}
	nopts := iclient.NewOptions(iopts)
	c, err := client.NewClientWithOptions(svcInfo, nopts)
	if err != nil {
		return nil, err
	}
	return c.(client.StreamX), nil
}