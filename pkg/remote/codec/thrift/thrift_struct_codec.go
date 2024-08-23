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

package thrift

import (
	"context"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

// Serialize implements the remote.StructCodec interface.
// It's only for encoding thrift struct to bytes
func (c thriftCodec) Serialize(ctx context.Context, data interface{}) ([]byte, error) {
	return c.marshalThriftData(ctx, data)
}

// Deserialize implements the remote.StructCodec interface.
// It's only for decoding bytes to thrift struct (`method`, `msgType` and `seqID` are not required here)
func (c thriftCodec) Deserialize(ctx context.Context, data interface{}, payload []byte) (err error) {
	buf := remote.NewReaderBuffer(payload)
	defer buf.Release(err)
	if err = c.unmarshalThriftData(buf, data, len(payload)); err != nil {
		err = perrors.NewProtocolError(err)
		return
	}

	return nil
}
