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

package grpc

import (
	"context"
	"io"
	"testing"

	"github.com/golang/mock/gomock"

	mocksremote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func Test_grpcCodec_Decode(t *testing.T) {
	codec := NewGRPCCodec()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testcases := []struct {
		desc              string
		role              remote.RPCRole
		mode              serviceinfo.StreamingMode
		getByteBufferFunc func() remote.ByteBuffer
		expectErr         error
	}{
		{
			desc: "client-side ClientStreaming decodes first grpc frame failed",
			role: remote.Client,
			mode: serviceinfo.StreamingClient,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return(nil, status.Err(codes.Internal, "test")).Times(1)
				return mockIn
			},
			expectErr: status.Err(codes.Internal, "test"),
		},
		{
			desc: "client-side ClientStreaming decodes second grpc frame successfully => wrong gRPC protocol implementation on the server side",
			role: remote.Client,
			mode: serviceinfo.StreamingClient,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				return mockIn
			},
			expectErr: errWrongGRPCImplementation,
		},
		{
			desc: "client-side ClientStreaming decodes second grpc frame getting io.EOF => normal exit on the server side",
			role: remote.Client,
			mode: serviceinfo.StreamingClient,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				mockIn.EXPECT().Next(5).Return(nil, io.EOF).Times(1)
				return mockIn
			},
			expectErr: ErrInvalidPayload,
		},
		{
			desc: "client-side ClientStreaming decodes second grpc frame getting gRPC errors",
			role: remote.Client,
			mode: serviceinfo.StreamingClient,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				mockIn.EXPECT().Next(5).Return(nil, status.Err(codes.Internal, "gRPC errors")).Times(1)
				return mockIn
			},
			expectErr: status.Err(codes.Internal, "gRPC errors"),
		},
		{
			desc: "client-side ClientStreaming decodes second grpc frame getting biz error",
			role: remote.Client,
			mode: serviceinfo.StreamingClient,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				mockIn.EXPECT().Next(5).Return(nil, kerrors.NewGRPCBizStatusError(10000, "test")).Times(1)
				return mockIn
			},
			expectErr: kerrors.NewGRPCBizStatusError(10000, "test"),
		},
		{
			desc: "client-side Unary decodes second grpc frame getting io.EOF => normal exit on the server side",
			role: remote.Client,
			mode: serviceinfo.StreamingUnary,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				mockIn.EXPECT().Next(5).Return(nil, io.EOF).Times(1)
				return mockIn
			},
			expectErr: ErrInvalidPayload,
		},
		{
			desc: "client-side Unary decodes second grpc frame getting biz error",
			role: remote.Client,
			mode: serviceinfo.StreamingUnary,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				mockIn.EXPECT().Next(5).Return(nil, kerrors.NewGRPCBizStatusError(10000, "test")).Times(1)
				return mockIn
			},
			expectErr: kerrors.NewGRPCBizStatusError(10000, "test"),
		},
		{
			desc: "client-side None decodes second grpc frame getting io.EOF => normal exit on the server side",
			role: remote.Client,
			mode: serviceinfo.StreamingUnary,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				mockIn.EXPECT().Next(5).Return(nil, io.EOF).Times(1)
				return mockIn
			},
			expectErr: ErrInvalidPayload,
		},
		{
			desc: "client-side None decodes second grpc frame getting biz error",
			role: remote.Client,
			mode: serviceinfo.StreamingNone,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				mockIn.EXPECT().Next(5).Return(nil, kerrors.NewGRPCBizStatusError(10000, "test")).Times(1)
				return mockIn
			},
			expectErr: kerrors.NewGRPCBizStatusError(10000, "test"),
		},
		{
			desc: "client-side ServerStreaming decodes",
			role: remote.Client,
			mode: serviceinfo.StreamingServer,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				return mockIn
			},
			expectErr: ErrInvalidPayload,
		},
		{
			desc: "client-side BidiStreaming decodes",
			role: remote.Client,
			mode: serviceinfo.StreamingBidirectional,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				return mockIn
			},
			expectErr: ErrInvalidPayload,
		},
		{
			desc: "server-side decodes",
			role: remote.Server,
			getByteBufferFunc: func() remote.ByteBuffer {
				mockIn := mocksremote.NewMockByteBuffer(ctrl)
				mockIn.EXPECT().Next(5).Return([]byte{0, 0, 0, 0, 1}, nil).Times(1)
				mockIn.EXPECT().Next(1).Return([]byte{1}, nil).Times(1)
				return mockIn
			},
			expectErr: ErrInvalidPayload,
		},
	}
	mockServiceName := "grpcService"
	mockMethod := "InvokeClientStreaming"
	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			inv := rpcinfo.NewInvocation(mockServiceName, mockMethod)
			inv.SetStreamingMode(tc.mode)
			cfg := rpcinfo.NewRPCConfig()
			// avoid unmarshal
			rpcinfo.AsMutableRPCConfig(cfg).SetPayloadCodec(serviceinfo.PayloadCodec(-1))
			ri := rpcinfo.NewRPCInfo(
				rpcinfo.EmptyEndpointInfo(),
				remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{ServiceName: mockServiceName}, mockMethod).ImmutableView(),
				inv, cfg, rpcinfo.NewRPCStats())
			ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
			mockMsg := remote.NewMessage(nil, ri, remote.Stream, tc.role)
			mockIn := tc.getByteBufferFunc()
			err := codec.Decode(ctx, mockMsg, mockIn)
			test.DeepEqual(t, err, tc.expectErr)
		})
	}
}

func Test_isNonServerStreaming(t *testing.T) {
	testcases := []struct {
		mode      serviceinfo.StreamingMode
		expectRes bool
	}{
		{
			mode:      serviceinfo.StreamingNone,
			expectRes: true,
		},
		{
			mode:      serviceinfo.StreamingUnary,
			expectRes: true,
		},
		{
			mode:      serviceinfo.StreamingClient,
			expectRes: true,
		},
		{
			mode:      serviceinfo.StreamingServer,
			expectRes: false,
		},
		{
			mode:      serviceinfo.StreamingBidirectional,
			expectRes: false,
		},
	}

	for _, tc := range testcases {
		test.Assert(t, isNonServerStreaming(tc.mode) == tc.expectRes)
	}
}
