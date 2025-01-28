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

package grpc

import (
	"github.com/cloudwego/kitex/pkg/reflection/grpc/handler"
	v1 "github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1"
	"github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1alpha"
)

// asV1Alpha returns an implementation of the v1alpha version of the reflection
// interface that delegates all calls to the given v1 version.
func asV1Alpha(svr v1.ServerReflection) v1alpha.ServerReflection {
	return v1AlphaServerImpl{svr: svr}
}

type v1AlphaServerImpl struct {
	svr v1.ServerReflection
}

func (s v1AlphaServerImpl) ServerReflectionInfo(stream v1alpha.ServerReflection_ServerReflectionInfoServer) error {
	return s.svr.ServerReflectionInfo(v1AlphaServerStreamAdapter{stream})
}

type v1AlphaServerStreamAdapter struct {
	v1alpha.ServerReflection_ServerReflectionInfoServer
}

func (s v1AlphaServerStreamAdapter) Send(response *v1.ServerReflectionResponse) error {
	return s.ServerReflection_ServerReflectionInfoServer.Send(handler.V1ToV1AlphaResponse(response))
}

func (s v1AlphaServerStreamAdapter) Recv() (*v1.ServerReflectionRequest, error) {
	resp, err := s.ServerReflection_ServerReflectionInfoServer.Recv()
	if err != nil {
		return nil, err
	}
	return handler.V1AlphaToV1Request(resp), nil
}
