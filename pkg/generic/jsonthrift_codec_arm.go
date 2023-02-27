//go:build !amd64 || !go1.16
// +build !amd64 !go1.16

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

package generic

import (
	"context"

	"github.com/cloudwego/kitex/pkg/remote"
)

type jsonThriftCodec struct {
	svcDsc           atomic.Value // *idl
	provider         DescriptorProvider
	codec            remote.PayloadCodec
	binaryWithBase64 bool
}

func newJsonThriftCodec(p DescriptorProvider, codec remote.PayloadCodec) (*jsonThriftCodec, error) {
	svc := <-p.Provide()
	c := &jsonThriftCodec{codec: codec, provider: p, binaryWithBase64: true}
	c.svcDsc.Store(svc)
	go c.update()
	return c, nil
}

func (c *jsonThriftCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	return c.originalMarshal(ctx, msg, in)
}

func (c *jsonThriftCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	return c.originalUnmarshal(ctx, msg, in)
}

func (c *jsonThriftCodec) Name() string {
	return "JSONThrift"
}
