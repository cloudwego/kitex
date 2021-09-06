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

	"github.com/jackedelic/kitex/internal/pkg/remote"
)

// JSONRequest alias of string
type JSONRequest = string

type jsonThriftCodec struct {
}

func (c *jsonThriftCodec) UpdateIDL(main string, includes map[string]string) error {
	return nil
}

func (c *jsonThriftCodec) Marshal(ctx context.Context, message remote.Message, out remote.ByteBuffer) error {
	// ...
	return nil
}

func (c *jsonThriftCodec) Unmarshal(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	// ...
	return nil
}

func (c *jsonThriftCodec) Name() string {
	return "JSONThrift"
}
