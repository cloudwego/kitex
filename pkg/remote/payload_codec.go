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

package remote

import (
	"context"
	"fmt"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var payloadCodecs = make(map[serviceinfo.PayloadCodec]PayloadCodec)

// PayloadCodec is used to marshal and unmarshal payload.
type PayloadCodec interface {
	Marshal(ctx context.Context, message Message, out ByteBuffer) error
	Unmarshal(ctx context.Context, message Message, in ByteBuffer) error
	Name() string
}

// StructCodec is used to serialize and deserialize a given struct.
type StructCodec interface {
	Serialize(ctx context.Context, data interface{}) ([]byte, error)
	Deserialize(ctx context.Context, data interface{}, payload []byte) error
}

// GetPayloadCodec gets desired payload codec from message.
func GetPayloadCodec(message Message) (PayloadCodec, error) {
	if message.PayloadCodec() != nil {
		return message.PayloadCodec(), nil
	}
	ct := message.ProtocolInfo().CodecType
	return GetPayloadCodecByCodecType(ct)
}

// GetPayloadCodecByCodecType gets payload codec by codecType.
func GetPayloadCodecByCodecType(ct serviceinfo.PayloadCodec) (PayloadCodec, error) {
	pc := payloadCodecs[ct]
	if pc == nil {
		return nil, fmt.Errorf("payload codec not found with codecType=%v", ct)
	}
	return pc, nil
}

// PutPayloadCode puts the desired payload codec to message.
func PutPayloadCode(name serviceinfo.PayloadCodec, v PayloadCodec) {
	payloadCodecs[name] = v
}

// GetStructCodec gets desired payload struct codec from message.
func GetStructCodec(message Message) (StructCodec, error) {
	if codec, err := GetPayloadCodec(message); err == nil {
		if structCodec := codec.(StructCodec); structCodec != nil {
			return structCodec, nil
		}
	}
	return nil, fmt.Errorf("payload struct codec not found with codecType=%v", message.ProtocolInfo().CodecType)
}

// GetStructCodecByCodecType gets the struct codec by codecType.
func GetStructCodecByCodecType(ct serviceinfo.PayloadCodec) (StructCodec, error) {
	if codec, err := GetPayloadCodecByCodecType(ct); err == nil {
		if structCodec := codec.(StructCodec); structCodec != nil {
			return structCodec, nil
		}
	}
	return nil, fmt.Errorf("payload struct codec not found with codecType=%v", ct)
}
