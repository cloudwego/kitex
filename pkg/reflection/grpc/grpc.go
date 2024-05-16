package grpc

import (
	"github.com/cloudwego/kitex/pkg/reflection/grpc/handler"
	"github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1"
	reflection_v1_service "github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1/serverreflection"
	reflection_v1alpha_service "github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1alpha/serverreflection"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"google.golang.org/protobuf/reflect/protoregistry"
)

var (
	NewV1ServiceInfo      = reflection_v1_service.NewServiceInfo
	NewV1AlphaServiceInfo = reflection_v1alpha_service.NewServiceInfo
)

func NewV1Handler(svcs map[string]*serviceinfo.ServiceInfo) v1.ServerReflection {
	return &handler.ServerReflectionServer{
		Svcs:         svcs,
		DescResolver: protoregistry.GlobalFiles,
		ExtResolver:  protoregistry.GlobalTypes,
	}
}
