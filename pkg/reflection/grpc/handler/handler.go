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

package handler

import (
	reflection_v1 "github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1"
	reflection_v1alpha "github.com/cloudwego/kitex/pkg/reflection/grpc/reflection/v1alpha"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"io"
	"sort"
)

//// ServiceInfoProvider is an interface used to retrieve metadata about the
//// services to expose.
//type ServiceInfoProvider interface {
//	GetServiceInfo() map[string]*serviceinfo.ServiceInfo
//}

// ExtensionResolver is the interface used to query details about extensions.
// This interface is satisfied by protoregistry.GlobalTypes.
type ExtensionResolver interface {
	protoregistry.ExtensionTypeResolver
	RangeExtensionsByMessage(message protoreflect.FullName, f func(protoreflect.ExtensionType) bool)
}

// ServerReflectionServer is the server API for ServerReflection service.
type ServerReflectionServer struct {
	Svcs         map[string]*serviceinfo.ServiceInfo
	DescResolver protodesc.Resolver
	ExtResolver  ExtensionResolver
}

// FileDescWithDependencies returns a slice of serialized fileDescriptors in
// wire format ([]byte). The fileDescriptors will include fd and all the
// transitive dependencies of fd with names not in sentFileDescriptors.
func (s *ServerReflectionServer) FileDescWithDependencies(fd protoreflect.FileDescriptor, sentFileDescriptors map[string]bool) ([][]byte, error) {
	if fd.IsPlaceholder() {
		// If the given root file is a placeholder, treat it
		// as missing instead of serializing it.
		return nil, protoregistry.NotFound
	}
	var r [][]byte
	queue := []protoreflect.FileDescriptor{fd}
	for len(queue) > 0 {
		currentfd := queue[0]
		queue = queue[1:]
		if currentfd.IsPlaceholder() {
			// Skip any missing files in the dependency graph.
			continue
		}
		if sent := sentFileDescriptors[currentfd.Path()]; len(r) == 0 || !sent {
			sentFileDescriptors[currentfd.Path()] = true
			fdProto := protodesc.ToFileDescriptorProto(currentfd)
			currentfdEncoded, err := proto.Marshal(fdProto)
			if err != nil {
				return nil, err
			}
			r = append(r, currentfdEncoded)
		}
		for i := 0; i < currentfd.Imports().Len(); i++ {
			queue = append(queue, currentfd.Imports().Get(i))
		}
	}
	return r, nil
}

// FileDescEncodingContainingSymbol finds the file descriptor containing the
// given symbol, finds all of its previously unsent transitive dependencies,
// does marshalling on them, and returns the marshalled result. The given symbol
// can be a type, a service or a method.
func (s *ServerReflectionServer) FileDescEncodingContainingSymbol(name string, sentFileDescriptors map[string]bool) ([][]byte, error) {
	d, err := s.DescResolver.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		return nil, err
	}
	return s.FileDescWithDependencies(d.ParentFile(), sentFileDescriptors)
}

// FileDescEncodingContainingExtension finds the file descriptor containing
// given extension, finds all of its previously unsent transitive dependencies,
// does marshalling on them, and returns the marshalled result.
func (s *ServerReflectionServer) FileDescEncodingContainingExtension(typeName string, extNum int32, sentFileDescriptors map[string]bool) ([][]byte, error) {
	xt, err := s.ExtResolver.FindExtensionByNumber(protoreflect.FullName(typeName), protoreflect.FieldNumber(extNum))
	if err != nil {
		return nil, err
	}
	return s.FileDescWithDependencies(xt.TypeDescriptor().ParentFile(), sentFileDescriptors)
}

// AllExtensionNumbersForTypeName returns all extension numbers for the given type.
func (s *ServerReflectionServer) AllExtensionNumbersForTypeName(name string) ([]int32, error) {
	var numbers []int32
	s.ExtResolver.RangeExtensionsByMessage(protoreflect.FullName(name), func(xt protoreflect.ExtensionType) bool {
		numbers = append(numbers, int32(xt.TypeDescriptor().Number()))
		return true
	})
	sort.Slice(numbers, func(i, j int) bool {
		return numbers[i] < numbers[j]
	})
	if len(numbers) == 0 {
		// maybe return an error if given type name is not known
		if _, err := s.DescResolver.FindDescriptorByName(protoreflect.FullName(name)); err != nil {
			return nil, err
		}
	}
	return numbers, nil
}

// ListServices returns the names of services this server exposes.
func (s *ServerReflectionServer) ListServices() []*reflection_v1.ServiceResponse {
	resp := make([]*reflection_v1.ServiceResponse, 0, len(s.Svcs))
	for svc := range s.Svcs {
		resp = append(resp, &reflection_v1.ServiceResponse{Name: svc})
	}
	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Name < resp[j].Name
	})
	return resp
}

// ServerReflectionInfo is the reflection service handler.
func (s *ServerReflectionServer) ServerReflectionInfo(stream reflection_v1.ServerReflection_ServerReflectionInfoServer) error {
	sentFileDescriptors := make(map[string]bool)
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		out := &reflection_v1.ServerReflectionResponse{
			ValidHost:       in.Host,
			OriginalRequest: in,
		}
		switch req := in.MessageRequest.(type) {
		case *reflection_v1.ServerReflectionRequest_FileByFilename:
			var b [][]byte
			fd, err := s.DescResolver.FindFileByPath(req.FileByFilename)
			if err == nil {
				b, err = s.FileDescWithDependencies(fd, sentFileDescriptors)
			}
			if err != nil {
				out.MessageResponse = &reflection_v1.ServerReflectionResponse_ErrorResponse{
					ErrorResponse: &reflection_v1.ErrorResponse{
						ErrorCode:    int32(codes.NotFound),
						ErrorMessage: err.Error(),
					},
				}
			} else {
				out.MessageResponse = &reflection_v1.ServerReflectionResponse_FileDescriptorResponse{
					FileDescriptorResponse: &reflection_v1.FileDescriptorResponse{FileDescriptorProto: b},
				}
			}
		case *reflection_v1.ServerReflectionRequest_FileContainingSymbol:
			b, err := s.FileDescEncodingContainingSymbol(req.FileContainingSymbol, sentFileDescriptors)
			if err != nil {
				out.MessageResponse = &reflection_v1.ServerReflectionResponse_ErrorResponse{
					ErrorResponse: &reflection_v1.ErrorResponse{
						ErrorCode:    int32(codes.NotFound),
						ErrorMessage: err.Error(),
					},
				}
			} else {
				out.MessageResponse = &reflection_v1.ServerReflectionResponse_FileDescriptorResponse{
					FileDescriptorResponse: &reflection_v1.FileDescriptorResponse{FileDescriptorProto: b},
				}
			}
		case *reflection_v1.ServerReflectionRequest_FileContainingExtension:
			typeName := req.FileContainingExtension.ContainingType
			extNum := req.FileContainingExtension.ExtensionNumber
			b, err := s.FileDescEncodingContainingExtension(typeName, extNum, sentFileDescriptors)
			if err != nil {
				out.MessageResponse = &reflection_v1.ServerReflectionResponse_ErrorResponse{
					ErrorResponse: &reflection_v1.ErrorResponse{
						ErrorCode:    int32(codes.NotFound),
						ErrorMessage: err.Error(),
					},
				}
			} else {
				out.MessageResponse = &reflection_v1.ServerReflectionResponse_FileDescriptorResponse{
					FileDescriptorResponse: &reflection_v1.FileDescriptorResponse{FileDescriptorProto: b},
				}
			}
		case *reflection_v1.ServerReflectionRequest_AllExtensionNumbersOfType:
			extNums, err := s.AllExtensionNumbersForTypeName(req.AllExtensionNumbersOfType)
			if err != nil {
				out.MessageResponse = &reflection_v1.ServerReflectionResponse_ErrorResponse{
					ErrorResponse: &reflection_v1.ErrorResponse{
						ErrorCode:    int32(codes.NotFound),
						ErrorMessage: err.Error(),
					},
				}
			} else {
				out.MessageResponse = &reflection_v1.ServerReflectionResponse_AllExtensionNumbersResponse{
					AllExtensionNumbersResponse: &reflection_v1.ExtensionNumberResponse{
						BaseTypeName:    req.AllExtensionNumbersOfType,
						ExtensionNumber: extNums,
					},
				}
			}
		case *reflection_v1.ServerReflectionRequest_ListServices:
			out.MessageResponse = &reflection_v1.ServerReflectionResponse_ListServicesResponse{
				ListServicesResponse: &reflection_v1.ListServiceResponse{
					Service: s.ListServices(),
				},
			}
		default:
			return status.Errorf(codes.InvalidArgument, "invalid MessageRequest: %v", in.MessageRequest)
		}

		if err := stream.Send(out); err != nil {
			return err
		}
	}
}

// V1ToV1AlphaResponse converts a v1 ServerReflectionResponse to a v1alpha.
func V1ToV1AlphaResponse(v1 *reflection_v1.ServerReflectionResponse) *reflection_v1alpha.ServerReflectionResponse {
	var v1alpha reflection_v1alpha.ServerReflectionResponse
	v1alpha.ValidHost = v1.ValidHost
	if v1.OriginalRequest != nil {
		v1alpha.OriginalRequest = V1ToV1AlphaRequest(v1.OriginalRequest)
	}
	switch mr := v1.MessageResponse.(type) {
	case *reflection_v1.ServerReflectionResponse_FileDescriptorResponse:
		if mr != nil {
			v1alpha.MessageResponse = &reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse{
				FileDescriptorResponse: &reflection_v1alpha.FileDescriptorResponse{
					FileDescriptorProto: mr.FileDescriptorResponse.GetFileDescriptorProto(),
				},
			}
		}
	case *reflection_v1.ServerReflectionResponse_AllExtensionNumbersResponse:
		if mr != nil {
			v1alpha.MessageResponse = &reflection_v1alpha.ServerReflectionResponse_AllExtensionNumbersResponse{
				AllExtensionNumbersResponse: &reflection_v1alpha.ExtensionNumberResponse{
					BaseTypeName:    mr.AllExtensionNumbersResponse.GetBaseTypeName(),
					ExtensionNumber: mr.AllExtensionNumbersResponse.GetExtensionNumber(),
				},
			}
		}
	case *reflection_v1.ServerReflectionResponse_ListServicesResponse:
		if mr != nil {
			svcs := make([]*reflection_v1alpha.ServiceResponse, len(mr.ListServicesResponse.GetService()))
			for i, svc := range mr.ListServicesResponse.GetService() {
				svcs[i] = &reflection_v1alpha.ServiceResponse{
					Name: svc.GetName(),
				}
			}
			v1alpha.MessageResponse = &reflection_v1alpha.ServerReflectionResponse_ListServicesResponse{
				ListServicesResponse: &reflection_v1alpha.ListServiceResponse{
					Service: svcs,
				},
			}
		}
	case *reflection_v1.ServerReflectionResponse_ErrorResponse:
		if mr != nil {
			v1alpha.MessageResponse = &reflection_v1alpha.ServerReflectionResponse_ErrorResponse{
				ErrorResponse: &reflection_v1alpha.ErrorResponse{
					ErrorCode:    mr.ErrorResponse.GetErrorCode(),
					ErrorMessage: mr.ErrorResponse.GetErrorMessage(),
				},
			}
		}
	default:
		// no value set
	}
	return &v1alpha
}

// V1AlphaToV1Request converts a v1alpha ServerReflectionRequest to a v1.
func V1AlphaToV1Request(v1alpha *reflection_v1alpha.ServerReflectionRequest) *reflection_v1.ServerReflectionRequest {
	var v1 reflection_v1.ServerReflectionRequest
	v1.Host = v1alpha.Host
	switch mr := v1alpha.MessageRequest.(type) {
	case *reflection_v1alpha.ServerReflectionRequest_FileByFilename:
		v1.MessageRequest = &reflection_v1.ServerReflectionRequest_FileByFilename{
			FileByFilename: mr.FileByFilename,
		}
	case *reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol:
		v1.MessageRequest = &reflection_v1.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: mr.FileContainingSymbol,
		}
	case *reflection_v1alpha.ServerReflectionRequest_FileContainingExtension:
		if mr.FileContainingExtension != nil {
			v1.MessageRequest = &reflection_v1.ServerReflectionRequest_FileContainingExtension{
				FileContainingExtension: &reflection_v1.ExtensionRequest{
					ContainingType:  mr.FileContainingExtension.GetContainingType(),
					ExtensionNumber: mr.FileContainingExtension.GetExtensionNumber(),
				},
			}
		}
	case *reflection_v1alpha.ServerReflectionRequest_AllExtensionNumbersOfType:
		v1.MessageRequest = &reflection_v1.ServerReflectionRequest_AllExtensionNumbersOfType{
			AllExtensionNumbersOfType: mr.AllExtensionNumbersOfType,
		}
	case *reflection_v1alpha.ServerReflectionRequest_ListServices:
		v1.MessageRequest = &reflection_v1.ServerReflectionRequest_ListServices{
			ListServices: mr.ListServices,
		}
	default:
		// no value set
	}
	return &v1
}

// V1ToV1AlphaRequest converts a v1 ServerReflectionRequest to a v1alpha.
func V1ToV1AlphaRequest(v1 *reflection_v1.ServerReflectionRequest) *reflection_v1alpha.ServerReflectionRequest {
	var v1alpha reflection_v1alpha.ServerReflectionRequest
	v1alpha.Host = v1.Host
	switch mr := v1.MessageRequest.(type) {
	case *reflection_v1.ServerReflectionRequest_FileByFilename:
		if mr != nil {
			v1alpha.MessageRequest = &reflection_v1alpha.ServerReflectionRequest_FileByFilename{
				FileByFilename: mr.FileByFilename,
			}
		}
	case *reflection_v1.ServerReflectionRequest_FileContainingSymbol:
		if mr != nil {
			v1alpha.MessageRequest = &reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: mr.FileContainingSymbol,
			}
		}
	case *reflection_v1.ServerReflectionRequest_FileContainingExtension:
		if mr != nil {
			v1alpha.MessageRequest = &reflection_v1alpha.ServerReflectionRequest_FileContainingExtension{
				FileContainingExtension: &reflection_v1alpha.ExtensionRequest{
					ContainingType:  mr.FileContainingExtension.GetContainingType(),
					ExtensionNumber: mr.FileContainingExtension.GetExtensionNumber(),
				},
			}
		}
	case *reflection_v1.ServerReflectionRequest_AllExtensionNumbersOfType:
		if mr != nil {
			v1alpha.MessageRequest = &reflection_v1alpha.ServerReflectionRequest_AllExtensionNumbersOfType{
				AllExtensionNumbersOfType: mr.AllExtensionNumbersOfType,
			}
		}
	case *reflection_v1.ServerReflectionRequest_ListServices:
		if mr != nil {
			v1alpha.MessageRequest = &reflection_v1alpha.ServerReflectionRequest_ListServices{
				ListServices: mr.ListServices,
			}
		}
	default:
		// no value set
	}
	return &v1alpha
}

// V1AlphaToV1Response converts a v1alpha ServerReflectionResponse to a v1.
func V1AlphaToV1Response(v1alpha *reflection_v1alpha.ServerReflectionResponse) *reflection_v1.ServerReflectionResponse {
	var v1 reflection_v1.ServerReflectionResponse
	v1.ValidHost = v1alpha.ValidHost
	if v1alpha.OriginalRequest != nil {
		v1.OriginalRequest = V1AlphaToV1Request(v1alpha.OriginalRequest)
	}
	switch mr := v1alpha.MessageResponse.(type) {
	case *reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse:
		if mr != nil {
			v1.MessageResponse = &reflection_v1.ServerReflectionResponse_FileDescriptorResponse{
				FileDescriptorResponse: &reflection_v1.FileDescriptorResponse{
					FileDescriptorProto: mr.FileDescriptorResponse.GetFileDescriptorProto(),
				},
			}
		}
	case *reflection_v1alpha.ServerReflectionResponse_AllExtensionNumbersResponse:
		if mr != nil {
			v1.MessageResponse = &reflection_v1.ServerReflectionResponse_AllExtensionNumbersResponse{
				AllExtensionNumbersResponse: &reflection_v1.ExtensionNumberResponse{
					BaseTypeName:    mr.AllExtensionNumbersResponse.GetBaseTypeName(),
					ExtensionNumber: mr.AllExtensionNumbersResponse.GetExtensionNumber(),
				},
			}
		}
	case *reflection_v1alpha.ServerReflectionResponse_ListServicesResponse:
		if mr != nil {
			svcs := make([]*reflection_v1.ServiceResponse, len(mr.ListServicesResponse.GetService()))
			for i, svc := range mr.ListServicesResponse.GetService() {
				svcs[i] = &reflection_v1.ServiceResponse{
					Name: svc.GetName(),
				}
			}
			v1.MessageResponse = &reflection_v1.ServerReflectionResponse_ListServicesResponse{
				ListServicesResponse: &reflection_v1.ListServiceResponse{
					Service: svcs,
				},
			}
		}
	case *reflection_v1alpha.ServerReflectionResponse_ErrorResponse:
		if mr != nil {
			v1.MessageResponse = &reflection_v1.ServerReflectionResponse_ErrorResponse{
				ErrorResponse: &reflection_v1.ErrorResponse{
					ErrorCode:    mr.ErrorResponse.GetErrorCode(),
					ErrorMessage: mr.ErrorResponse.GetErrorMessage(),
				},
			}
		}
	default:
		// no value set
	}
	return &v1
}
