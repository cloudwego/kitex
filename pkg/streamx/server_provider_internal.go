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

package streamx

import (
	"context"
	"net"
)

func NewServerProvider(ss ServerProvider) ServerProvider {
	if _, ok := ss.(*internalServerProvider); ok {
		return ss
	}
	return internalServerProvider{ServerProvider: ss}
}

type internalServerProvider struct {
	ServerProvider
}

func (p internalServerProvider) OnStream(ctx context.Context, conn net.Conn) (context.Context, ServerStream, error) {
	ctx, ss, err := p.ServerProvider.OnStream(ctx, conn)
	if err != nil {
		return nil, nil, err
	}
	return ctx, ss, nil
}
