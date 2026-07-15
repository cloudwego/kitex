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

package grpc

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	mocksremote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestGRPCCodecDecodeMaxReceiveMessageSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	codec := NewGRPCCodec(WithMaxReceiveMessageSize(1))
	mockIn := mocksremote.NewMockByteBuffer(ctrl)
	mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 2}, nil).Times(1)

	cfg := rpcinfo.NewRPCConfig()
	rpcinfo.AsMutableRPCConfig(cfg).SetPayloadCodec(serviceinfo.PayloadCodec(-1))
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.EmptyEndpointInfo(),
		remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "grpcService"}, "method").ImmutableView(),
		rpcinfo.NewInvocation("grpcService", "method"),
		cfg,
		rpcinfo.NewRPCStats(),
	)
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	msg := remote.NewMessage(nil, nil, ri, remote.Stream, remote.Client)

	err := codec.Decode(ctx, msg, mockIn)
	if pErr, ok := err.(perrors.ProtocolError); !ok || pErr.TypeId() != perrors.SizeLimit {
		t.Fatalf("unexpected error: %T %v", err, err)
	}
}
