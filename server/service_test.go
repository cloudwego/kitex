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

package server

import (
	"context"
	"strconv"
	"testing"

	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var (
	mockGenericService = &serviceinfo.ServiceInfo{
		ServiceName:  mocks.MockServiceName,
		Methods:      make(map[string]serviceinfo.MethodInfo),
		PayloadCodec: serviceinfo.Thrift,
		Extra:        make(map[string]interface{}),
		GenericMethod: func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
			return mocks.ServiceInfo().MethodInfo(context.Background(), methodName)
		},
	}
	mockGeneric3Service = &serviceinfo.ServiceInfo{
		ServiceName:  mocks.MockService3Name,
		Methods:      make(map[string]serviceinfo.MethodInfo),
		PayloadCodec: serviceinfo.Thrift,
		Extra:        make(map[string]interface{}),
		GenericMethod: func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
			return mocks.Service3Info().MethodInfo(context.Background(), methodName)
		},
	}
	mockCombineService = &serviceinfo.ServiceInfo{
		ServiceName: serviceinfo.CombineServiceName,
		Methods: map[string]serviceinfo.MethodInfo{
			mocks.MockMethod: mocks.ServiceInfo().MethodInfo(context.Background(), mocks.MockMethod),
		},
		PayloadCodec: serviceinfo.Thrift,
		Extra: map[string]interface{}{
			serviceinfo.CombineServiceKey: true,
		},
	}
)

func TestSearchService(t *testing.T) {
	type svc struct {
		svcInfo           *serviceinfo.ServiceInfo
		isFallbackService bool
		isUnknownService  bool
	}
	testcases := []struct {
		svcs                            []svc
		refuseTrafficWithoutServiceName bool

		serviceName, methodName string
		strict                  bool
		expectSvcInfo           *serviceinfo.ServiceInfo
		expectUnknownHandler    bool
	}{
		{
			svcs: []svc{
				{
					svcInfo: mocks.ServiceInfo(),
				},
				{
					svcInfo:           mocks.Service3Info(),
					isFallbackService: true,
				},
			},
			serviceName:   mocks.MockServiceName,
			methodName:    mocks.MockMethod,
			expectSvcInfo: mocks.ServiceInfo(),
		},
		{
			svcs: []svc{
				{
					svcInfo: mocks.ServiceInfo(),
				},
				{
					svcInfo:           mocks.Service3Info(),
					isFallbackService: true,
				},
			},
			serviceName:   "",
			methodName:    mocks.MockMethod,
			expectSvcInfo: mocks.Service3Info(),
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: true,
				},
				{
					svcInfo: mocks.Service3Info(),
				},
			},
			serviceName:   mocks.MockService3Name,
			methodName:    mocks.MockMethod,
			expectSvcInfo: mocks.Service3Info(),
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: true,
				},
				{
					svcInfo: mocks.Service3Info(),
				},
			},
			serviceName:   "",
			methodName:    mocks.MockMethod,
			expectSvcInfo: mocks.ServiceInfo(),
		},
		{
			svcs: []svc{
				{
					svcInfo: mocks.ServiceInfo(),
				},
				{
					svcInfo: mocks.Service3Info(),
				},
			},
			refuseTrafficWithoutServiceName: true,
			serviceName:                     "",
			methodName:                      mocks.MockMethod,
			expectSvcInfo:                   nil,
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: true,
				},
				{
					svcInfo: mocks.Service3Info(),
				},
			},
			serviceName:   serviceinfo.GenericService,
			methodName:    mocks.MockMethod,
			expectSvcInfo: nil,
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: true,
				},
				{
					svcInfo: mocks.Service3Info(),
				},
			},
			serviceName:   "xxxxxx",
			methodName:    mocks.MockExceptionMethod,
			expectSvcInfo: mocks.ServiceInfo(),
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: true,
				},
				{
					svcInfo: mocks.Service3Info(),
				},
			},
			serviceName:   serviceinfo.GenericService,
			methodName:    mocks.MockExceptionMethod,
			expectSvcInfo: mocks.ServiceInfo(),
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: true,
				},
				{
					svcInfo: mocks.Service3Info(),
				},
			},
			serviceName:   serviceinfo.CombineServiceName,
			methodName:    mocks.MockExceptionMethod,
			expectSvcInfo: mocks.ServiceInfo(),
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: true,
				},
				{
					svcInfo: mocks.Service3Info(),
				},
			},
			serviceName:   "xxxxxx",
			methodName:    mocks.MockMethod,
			expectSvcInfo: nil,
		},
		{
			svcs: []svc{
				{
					svcInfo: generic.ServiceInfoWithGeneric(generic.BinaryThriftGeneric()),
				},
			},
			serviceName:   "xxxxxx",
			methodName:    mocks.MockMethod,
			expectSvcInfo: generic.ServiceInfoWithGeneric(generic.BinaryThriftGeneric()),
		},
		{
			svcs: []svc{
				{
					svcInfo: mocks.ServiceInfo(),
				},
				{
					svcInfo: mocks.Service2Info(),
				},
			},
			strict:        true,
			serviceName:   mocks.MockServiceName,
			methodName:    mocks.MockMethod,
			expectSvcInfo: mocks.ServiceInfo(),
		},
		{
			svcs: []svc{
				{
					svcInfo:           mockGenericService,
					isFallbackService: true,
				},
				{
					svcInfo: mocks.Service3Info(),
				},
			},
			methodName:    mocks.MockMethod,
			expectSvcInfo: mockGenericService,
		},
		{
			svcs: []svc{
				{
					svcInfo: mockGenericService,
				},
				{
					svcInfo:           mocks.Service3Info(),
					isFallbackService: true,
				},
			},
			methodName:    mocks.MockMethod,
			expectSvcInfo: mocks.Service3Info(),
		},
		{
			svcs: []svc{
				{
					svcInfo:           mockGenericService,
					isFallbackService: true,
				},
				{
					svcInfo: mockGeneric3Service,
				},
			},
			methodName:    mocks.MockMethod,
			expectSvcInfo: mockGenericService,
		},
		{
			svcs: []svc{
				{
					svcInfo: mockGenericService,
				},
				{
					svcInfo:           mockGeneric3Service,
					isFallbackService: true,
				},
			},
			methodName:    mocks.MockMethod,
			expectSvcInfo: mockGeneric3Service,
		},
		{
			svcs: []svc{
				{
					svcInfo: mockGenericService,
				},
				{
					svcInfo:           mockGeneric3Service,
					isFallbackService: true,
				},
			},
			methodName:    "xxxxxxx",
			expectSvcInfo: nil,
		},
		{
			svcs: []svc{
				{
					svcInfo: mocks.ServiceInfo(),
				},
				{
					isUnknownService: true,
				},
			},
			serviceName:   mocks.MockServiceName,
			methodName:    mocks.MockMethod,
			expectSvcInfo: mocks.ServiceInfo(),
		},
		{
			svcs: []svc{
				{
					svcInfo: mocks.ServiceInfo(),
				},
				{
					isUnknownService: true,
				},
			},
			serviceName:          mocks.MockServiceName,
			methodName:           "xxxxxxx",
			expectSvcInfo:        mocks.ServiceInfo(),
			expectUnknownHandler: true,
		},
		{
			svcs: []svc{
				{
					svcInfo: mocks.ServiceInfo(),
				},
				{
					isUnknownService: true,
				},
			},
			strict:        true,
			serviceName:   "xxxxxx",
			methodName:    mocks.MockMethod,
			expectSvcInfo: &serviceinfo.ServiceInfo{ServiceName: "xxxxxx"},
		},
		{
			svcs: []svc{
				{
					svcInfo: mockGenericService,
				},
				{
					isUnknownService: true,
				},
			},
			methodName:    mocks.MockMethod,
			expectSvcInfo: mockGenericService,
		},
		{
			svcs: []svc{
				{
					svcInfo: mockGenericService,
				},
				{
					isUnknownService: true,
				},
			},
			methodName:    "xxxxxx",
			expectSvcInfo: &serviceinfo.ServiceInfo{},
		},
		{
			svcs: []svc{
				{
					svcInfo: mockGenericService,
				},
				{
					isUnknownService: true,
				},
			},
			serviceName:          mocks.MockServiceName,
			methodName:           "xxxxxxx",
			expectSvcInfo:        mockGenericService,
			expectUnknownHandler: true,
		},
		{
			svcs: []svc{
				{
					svcInfo: mockCombineService,
				},
			},
			serviceName:   "xxxxxx",
			methodName:    "xxxxxx",
			expectSvcInfo: mockCombineService,
		},
		{
			svcs: []svc{
				{
					svcInfo: mockCombineService,
				},
			},
			strict:        true,
			serviceName:   "xxxxxx",
			methodName:    "xxxxxx",
			expectSvcInfo: nil,
		},
		{
			svcs: []svc{
				{
					svcInfo: generic.ServiceInfoWithGeneric(generic.BinaryPbGeneric(mocks.MockServiceName, "")),
				},
				{
					isUnknownService: true,
				},
			},
			serviceName:          mocks.MockServiceName,
			methodName:           mocks.MockMethod,
			expectSvcInfo:        generic.ServiceInfoWithGeneric(generic.BinaryPbGeneric(mocks.MockServiceName, "")),
			expectUnknownHandler: false,
		},
		{
			svcs: []svc{
				{
					svcInfo: generic.ServiceInfoWithGeneric(generic.BinaryPbGeneric(mocks.MockServiceName, "")),
				},
				{
					isUnknownService: true,
				},
			},
			serviceName:          mocks.MockServiceName,
			methodName:           mocks.MockMethod,
			expectSvcInfo:        generic.ServiceInfoWithGeneric(generic.BinaryPbGeneric(mocks.MockServiceName, "")),
			expectUnknownHandler: false,
		},
	}
	for i, tcase := range testcases {
		svcs := newServices()
		for _, svc := range tcase.svcs {
			test.Assert(t, svcs.addService(svc.svcInfo, mocks.MyServiceHandler(), &RegisterOptions{IsFallbackService: svc.isFallbackService, IsUnknownService: svc.isUnknownService}) == nil)
		}
		test.Assert(t, svcs.check(tcase.refuseTrafficWithoutServiceName) == nil)
		svcInfo := svcs.SearchService(tcase.serviceName, tcase.methodName, tcase.strict, serviceinfo.Thrift)
		if tcase.expectSvcInfo == nil {
			test.Assert(t, svcInfo == nil, i)
		} else {
			test.Assert(t, svcInfo.ServiceName == tcase.expectSvcInfo.ServiceName, i)
			svc := svcs.getService(svcInfo.ServiceName)
			test.Assert(t, (svc.getHandler(tcase.methodName) == svc.unknownMethodHandler) == tcase.expectUnknownHandler, i)
		}
	}
}

func TestAddService(t *testing.T) {
	type svc struct {
		svcInfo           *serviceinfo.ServiceInfo
		isFallbackService bool
		isUnknownService  bool
	}
	testcases := []struct {
		svcs            []svc
		expectAddSvcErr bool
	}{
		{
			svcs: []svc{
				{
					isUnknownService: true,
				},
				{
					isUnknownService: true,
				},
			},
			expectAddSvcErr: true,
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: true,
				},
				{
					svcInfo:           mocks.Service3Info(),
					isFallbackService: true,
				},
			},
			expectAddSvcErr: true,
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: true,
				},
				{
					svcInfo: mocks.ServiceInfo(),
				},
			},
			expectAddSvcErr: true,
		},
	}
	for i, tcase := range testcases {
		svcs := newServices()
		var hasAddErr bool
		for _, svc := range tcase.svcs {
			err := svcs.addService(svc.svcInfo, mocks.MyServiceHandler(), &RegisterOptions{IsFallbackService: svc.isFallbackService, IsUnknownService: svc.isUnknownService})
			if err != nil {
				hasAddErr = true
			}
		}
		test.Assert(t, tcase.expectAddSvcErr == hasAddErr, i)
	}
}

func TestCheckService(t *testing.T) {
	type svc struct {
		svcInfo           *serviceinfo.ServiceInfo
		isFallbackService bool
		isUnknownService  bool
	}
	testcases := []struct {
		svcs                            []svc
		refuseTrafficWithoutServiceName bool
		expectCheckErr                  bool
	}{
		{
			svcs:           nil,
			expectCheckErr: true,
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: false,
				},
				{
					svcInfo:           mocks.Service2Info(),
					isFallbackService: true,
				},
				{
					svcInfo:           mocks.Service3Info(),
					isFallbackService: false,
				},
			},
			expectCheckErr: true,
		},
		{
			svcs: []svc{
				{
					svcInfo:           mocks.ServiceInfo(),
					isFallbackService: false,
				},
				{
					svcInfo:           mocks.Service3Info(),
					isFallbackService: false,
				},
			},
			expectCheckErr: true,
		},
		{
			svcs: []svc{
				{
					svcInfo: generic.ServiceInfoWithGeneric(generic.BinaryThriftGeneric()),
				},
				{
					svcInfo: mocks.ServiceInfo(),
				},
			},
			expectCheckErr: true,
		},
		{
			svcs: []svc{
				{
					svcInfo: generic.ServiceInfoWithGeneric(generic.BinaryThriftGeneric()),
				},
				{
					isUnknownService: true,
				},
			},
			expectCheckErr: true,
		},
		{
			svcs: []svc{
				{
					svcInfo: generic.ServiceInfoWithGeneric(generic.BinaryThriftGeneric()),
				},
			},
			expectCheckErr: false,
		},
		{
			svcs: []svc{
				{
					svcInfo: mockCombineService,
				},
				{
					svcInfo: mocks.ServiceInfo(),
				},
			},
			expectCheckErr: true,
		},
		{
			svcs: []svc{
				{
					svcInfo: mockCombineService,
				},
				{
					isUnknownService: true,
				},
			},
			expectCheckErr: true,
		},
		{
			svcs: []svc{
				{
					svcInfo: mockCombineService,
				},
			},
			expectCheckErr: false,
		},
	}
	for i, tcase := range testcases {
		svcs := newServices()
		for _, svc := range tcase.svcs {
			test.Assert(t, svcs.addService(svc.svcInfo, mocks.MyServiceHandler(), &RegisterOptions{IsFallbackService: svc.isFallbackService, IsUnknownService: svc.isUnknownService}) == nil)
		}
		test.Assert(t, svcs.check(tcase.refuseTrafficWithoutServiceName) != nil == tcase.expectCheckErr, i)
	}
}

func TestRegisterBinaryGenericMethodFunc(t *testing.T) {
	// binary thrift generic
	svcInfo := generic.ServiceInfoWithGeneric(generic.BinaryThriftGenericV2("xxx"))
	test.Assert(t, registerBinaryGenericMethodFunc(svcInfo) == nil)

	// not supported payload codec
	svcInfo = &serviceinfo.ServiceInfo{PayloadCodec: serviceinfo.Hessian2}
	test.Assert(t, registerBinaryGenericMethodFunc(svcInfo) == nil)

	// protobuf
	svcInfo = &serviceinfo.ServiceInfo{PayloadCodec: serviceinfo.Protobuf}
	test.Assert(t, registerBinaryGenericMethodFunc(svcInfo) != nil)

	// register generic method to service info with generated code
	svcInfo = mocks.ServiceInfo()
	nSvcInfo := registerBinaryGenericMethodFunc(svcInfo)
	test.Assert(t, nSvcInfo.GenericMethod != nil)
	mi := nSvcInfo.MethodInfo(context.Background(), mocks.MockStreamingMethod)
	test.Assert(t, mi.StreamingMode() == serviceinfo.StreamingBidirectional)
	mi = nSvcInfo.MethodInfo(context.Background(), "xxxxxx")
	test.Assert(t, mi.StreamingMode() == serviceinfo.StreamingNone)
	mi = nSvcInfo.MethodInfo(igeneric.WithGenericStreamingMode(context.Background(), serviceinfo.StreamingBidirectional), "xxxxxx")
	test.Assert(t, mi.StreamingMode() == serviceinfo.StreamingBidirectional)

	// register generic method to generic service info
	svcInfo = mockGenericService
	mi = svcInfo.MethodInfo(igeneric.WithGenericStreamingMode(context.Background(), serviceinfo.StreamingBidirectional), "xxxxxx")
	test.Assert(t, mi == nil)
	nSvcInfo = registerBinaryGenericMethodFunc(svcInfo)
	test.Assert(t, nSvcInfo.GenericMethod != nil)
	mi = nSvcInfo.MethodInfo(context.Background(), mocks.MockMethod)
	test.Assert(t, mi.StreamingMode() == serviceinfo.StreamingNone)
	mi = nSvcInfo.MethodInfo(context.Background(), mocks.MockStreamingMethod)
	test.Assert(t, mi.StreamingMode() == serviceinfo.StreamingBidirectional)
	mi = nSvcInfo.MethodInfo(igeneric.WithGenericStreamingMode(context.Background(), serviceinfo.StreamingBidirectional), "xxxxxx")
	test.Assert(t, mi.StreamingMode() == serviceinfo.StreamingBidirectional)
}

func BenchmarkUnknownService(b *testing.B) {
	us := &unknownService{svcs: make(map[string]*service)}
	svc := us.getSvc("xxxxxx")
	test.Assert(b, svc == nil)

	b.RunParallel(func(pb *testing.PB) {
		var i int
		for pb.Next() {
			i++
			codec := serviceinfo.Thrift
			if i%2 == 0 {
				codec = serviceinfo.Protobuf
			}
			svc := us.getOrStoreSvc(strconv.Itoa(i), codec)
			test.Assert(b, svc != nil)
			test.Assert(b, svc.svcInfo.ServiceName == strconv.Itoa(i))
			test.Assert(b, svc.svcInfo.PayloadCodec == codec)
		}
	})
	svc = us.getSvc("1")
	test.Assert(b, svc != nil)
	test.Assert(b, svc.svcInfo.ServiceName == "1")
	test.Assert(b, svc.svcInfo.PayloadCodec == serviceinfo.Thrift)
}
