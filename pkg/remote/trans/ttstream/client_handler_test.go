/*
 * Copyright 2026 CloudWeGo Authors
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

package ttstream

import (
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func Test_getProtocolId(t *testing.T) {
	tests := []struct {
		desc        string
		payloadType serviceinfo.PayloadCodec
		expectID    ttheader.ProtocolID
		expectErr   bool
	}{
		{
			desc:        "thrift payload",
			payloadType: serviceinfo.Thrift,
			expectID:    ttheader.ProtocolIDThriftStruct,
		},
		{
			desc:        "protobuf payload",
			payloadType: serviceinfo.Protobuf,
			expectID:    ttheader.ProtocolIDProtobufStruct,
		},
		{
			desc:        "unsupported payload type",
			payloadType: serviceinfo.PayloadCodec(999),
			expectID:    0,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create mock rpcinfo with specific payload codec
			cfg := rpcinfo.NewRPCConfig()
			rpcinfo.AsMutableRPCConfig(cfg).SetPayloadCodec(tt.payloadType)
			ri := rpcinfo.NewRPCInfo(nil, nil, nil, cfg, nil)

			gotID, err := getProtocolId(ri)
			if tt.expectErr {
				test.Assert(t, err != nil)
				test.Assert(t, strings.Contains(err.Error(), "not supported payload type:"), err)
			} else {
				test.Assert(t, err == nil, err)
				test.Assert(t, gotID == tt.expectID, gotID)
			}
		})
	}
}
