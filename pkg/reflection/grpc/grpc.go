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
	"github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1"
	reflection_v1_service "github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1/serverreflection"
	"github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1alpha"
	reflection_v1alpha_service "github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1alpha/serverreflection"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"google.golang.org/protobuf/reflect/protoregistry"
)

var (
	NewV1ServiceInfo      = reflection_v1_service.NewServiceInfo
	NewV1AlphaServiceInfo = reflection_v1alpha_service.NewServiceInfo
)

func NewV1Handler(svcs map[string]*serviceinfo.ServiceInfo) v1.ServerReflection {
	return handler.NewServerReflection(svcs, protoregistry.GlobalFiles, protoregistry.GlobalTypes)
}

func NewV1alphaHandler(svcs map[string]*serviceinfo.ServiceInfo) v1alpha.ServerReflection {
	return asV1Alpha(NewV1Handler(svcs))
}
